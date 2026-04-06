package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/features"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/dns/localdns"
	"github.com/xtls/xray-core/features/inbound"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/transport/internet"
)

// Server is an instance of Xray. At any time, there must be at most one Server instance running.
type Server interface {
	common.Runnable
}

// ServerType returns the type of the server.
func ServerType() interface{} {
	return (*Instance)(nil)
}

type resolution struct {
	deps     []reflect.Type
	callback interface{}
}

func getFeature(allFeatures []features.Feature, t reflect.Type) features.Feature {
	for _, f := range allFeatures {
		if reflect.TypeOf(f.Type()) == t {
			return f
		}
	}
	return nil
}

func (r *resolution) callbackResolution(allFeatures []features.Feature) error {
	callback := reflect.ValueOf(r.callback)
	var input []reflect.Value
	callbackType := callback.Type()
	for i := 0; i < callbackType.NumIn(); i++ {
		pt := callbackType.In(i)
		for _, f := range allFeatures {
			if reflect.TypeOf(f).AssignableTo(pt) {
				input = append(input, reflect.ValueOf(f))
				break
			}
		}
	}

	if len(input) != callbackType.NumIn() {
		panic("Can't get all input parameters")
	}

	var err error
	ret := callback.Call(input)
	errInterface := reflect.TypeOf((*error)(nil)).Elem()
	for i := len(ret) - 1; i >= 0; i-- {
		if ret[i].Type() == errInterface {
			v := ret[i].Interface()
			if v != nil {
				err = v.(error)
			}
			break
		}
	}

	return err
}

// Instance combines all Xray features.
type Instance struct {
	statusLock                 sync.Mutex
	features                   []features.Feature
	pendingResolutions         []resolution
	pendingOptionalResolutions []resolution
	running                    bool
	resolveLock                sync.Mutex

	ctx context.Context
}

// Instance state
func (server *Instance) IsRunning() bool {
	return server.running
}

func AddInboundHandler(server *Instance, config *InboundHandlerConfig) error {
	inboundManager := server.GetFeature(inbound.ManagerType()).(inbound.Manager)
	rawHandler, err := CreateObject(server, config)
	if err != nil {
		return err
	}
	handler, ok := rawHandler.(inbound.Handler)
	if !ok {
		return errors.New("not an InboundHandler")
	}
	if err := inboundManager.AddHandler(server.ctx, handler); err != nil {
		return err
	}
	return nil
}

func addInboundHandlers(server *Instance, configs []*InboundHandlerConfig) error {
	for _, inboundConfig := range configs {
		if err := AddInboundHandler(server, inboundConfig); err != nil {
			return err
		}
	}

	return nil
}

func AddOutboundHandler(server *Instance, config *OutboundHandlerConfig) error {
	outboundManager := server.GetFeature(outbound.ManagerType()).(outbound.Manager)
	rawHandler, err := CreateObject(server, config)
	if err != nil {
		return err
	}
	handler, ok := rawHandler.(outbound.Handler)
	if !ok {
		return errors.New("not an OutboundHandler")
	}
	if err := outboundManager.AddHandler(server.ctx, handler); err != nil {
		return err
	}
	return nil
}

func addOutboundHandlers(server *Instance, configs []*OutboundHandlerConfig) error {
	for _, outboundConfig := range configs {
		if err := AddOutboundHandler(server, outboundConfig); err != nil {
			return err
		}
	}

	return nil
}

// RequireFeatures is a helper function to require features from Instance in context.
// See Instance.RequireFeatures for more information.
func RequireFeatures(ctx context.Context, callback interface{}) error {
	v := MustFromContext(ctx)
	return v.RequireFeatures(callback, false)
}

// OptionalFeatures is a helper function to aquire features from Instance in context.
// See Instance.RequireFeatures for more information.
func OptionalFeatures(ctx context.Context, callback interface{}) error {
	v := MustFromContext(ctx)
	return v.RequireFeatures(callback, true)
}

// New returns a new Xray instance based on given configuration.
// The instance is not started at this point.
// To ensure Xray instance works properly, the config must contain one Dispatcher, one InboundHandlerManager and one OutboundHandlerManager. Other features are optional.
func New(config *Config) (*Instance, error) {
	server := &Instance{ctx: context.Background()}

	done, err := initInstanceWithConfig(config, server)
	if done {
		return nil, err
	}

	return server, nil
}

func NewWithContext(ctx context.Context, config *Config) (*Instance, error) {
	server := &Instance{ctx: ctx}

	done, err := initInstanceWithConfig(config, server)
	if done {
		return nil, err
	}

	return server, nil
}

func initInstanceWithConfig(config *Config, server *Instance) (bool, error) {
	server.ctx = context.WithValue(server.ctx, "cone",
		platform.NewEnvFlag(platform.UseCone).GetValue(func() string { return "" }) != "true")

	for _, appSettings := range config.App {
		settings, err := appSettings.GetInstance()
		if err != nil {
			return true, err
		}
		obj, err := CreateObject(server, settings)
		if err != nil {
			return true, err
		}
		if feature, ok := obj.(features.Feature); ok {
			if err := server.AddFeature(feature); err != nil {
				return true, err
			}
		}
	}

	essentialFeatures := []struct {
		Type     interface{}
		Instance features.Feature
	}{
		{dns.ClientType(), localdns.New()},
		{policy.ManagerType(), policy.DefaultManager{}},
		{routing.RouterType(), routing.DefaultRouter{}},
		{stats.ManagerType(), stats.NoopManager{}},
	}

	for _, f := range essentialFeatures {
		if server.GetFeature(f.Type) == nil {
			if err := server.AddFeature(f.Instance); err != nil {
				return true, err
			}
		}
	}

	internet.InitSystemDialer(
		server.GetFeature(dns.ClientType()).(dns.Client),
		func() outbound.Manager {
			obm, _ := server.GetFeature(outbound.ManagerType()).(outbound.Manager)
			return obm
		}(),
	)

	server.resolveLock.Lock()
	if server.pendingResolutions != nil {
		server.resolveLock.Unlock()
		return true, errors.New("not all dependencies are resolved.")
	}
	server.resolveLock.Unlock()

	if err := addInboundHandlers(server, config.Inbound); err != nil {
		return true, err
	}

	if err := addOutboundHandlers(server, config.Outbound); err != nil {
		return true, err
	}
	return false, nil
}

// Type implements common.HasType.
func (s *Instance) Type() interface{} {
	return ServerType()
}

// Close shutdown the Xray instance.
func (s *Instance) Close() error {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()

	s.running = false

	var errs []interface{}
	for _, f := range s.features {
		if err := f.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.New("failed to close all features").Base(errors.New(serial.Concat(errs...)))
	}

	return nil
}

// RequireFeatures registers a callback, which will be called when all dependent features are registered.
// The callback must be a func(). All its parameters must be features.Feature.
func (s *Instance) RequireFeatures(callback interface{}, optional bool) error {
	callbackType := reflect.TypeOf(callback)
	if callbackType.Kind() != reflect.Func {
		panic("not a function")
	}

	var featureTypes []reflect.Type
	for i := 0; i < callbackType.NumIn(); i++ {
		featureTypes = append(featureTypes, reflect.PtrTo(callbackType.In(i)))
	}

	r := resolution{
		deps:     featureTypes,
		callback: callback,
	}

	s.resolveLock.Lock()
	foundAll := true
	for _, d := range r.deps {
		f := getFeature(s.features, d)
		if f == nil {
			foundAll = false
			break
		}
	}
	if foundAll {
		s.resolveLock.Unlock()
		return r.callbackResolution(s.features)
	} else {
		if optional {
			s.pendingOptionalResolutions = append(s.pendingOptionalResolutions, r)
		} else {
			s.pendingResolutions = append(s.pendingResolutions, r)
		}
		s.resolveLock.Unlock()
		return nil
	}
}

// AddFeature registers a feature into current Instance.
func (s *Instance) AddFeature(feature features.Feature) error {
	if s.running {
		if err := feature.Start(); err != nil {
			errors.LogInfoInner(s.ctx, err, "failed to start feature")
		}
		return nil
	}

	s.resolveLock.Lock()
	s.features = append(s.features, feature)

	var availableResolution []resolution
	var pending []resolution
	for _, r := range s.pendingResolutions {
		foundAll := true
		for _, d := range r.deps {
			f := getFeature(s.features, d)
			if f == nil {
				foundAll = false
				break
			}
		}
		if foundAll {
			availableResolution = append(availableResolution, r)
		} else {
			pending = append(pending, r)
		}
	}
	s.pendingResolutions = pending

	var pendingOptional []resolution
	for _, r := range s.pendingOptionalResolutions {
		foundAll := true
		for _, d := range r.deps {
			f := getFeature(s.features, d)
			if f == nil {
				foundAll = false
				break
			}
		}
		if foundAll {
			availableResolution = append(availableResolution, r)
		} else {
			pendingOptional = append(pendingOptional, r)
		}
	}
	s.pendingOptionalResolutions = pendingOptional
	s.resolveLock.Unlock()

	var err error
	for _, r := range availableResolution {
		err = r.callbackResolution(s.features) // only return the last error for now
	}
	return err
}

// GetFeature returns a feature of the given type, or nil if such feature is not registered.
func (s *Instance) GetFeature(featureType interface{}) features.Feature {
	return getFeature(s.features, reflect.TypeOf(featureType))
}

// Start starts the Xray instance, including all registered features. When Start returns error, the state of the instance is unknown.
// A Xray instance can be started only once. Upon closing, the instance is not guaranteed to start again.
//
// xray:api:stable
func (s *Instance) Start() error {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()

	s.running = true
	for _, f := range s.features {
		if err := f.Start(); err != nil {
			return err
		}
	}

	errors.LogWarning(s.ctx, "Xray ", Version(), " started")

	return nil
}
