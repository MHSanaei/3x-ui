package outbound

import (
	"context"
	"crypto/rand"
	goerrors "errors"
	"io"
	"math/big"
	"os"

	"github.com/xtls/xray-core/common/dice"

	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/mux"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/proxy"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tls"
	"github.com/xtls/xray-core/transport/pipe"
	"google.golang.org/protobuf/proto"
)

func getStatCounter(v *core.Instance, tag string) (stats.Counter, stats.Counter) {
	var uplinkCounter stats.Counter
	var downlinkCounter stats.Counter

	policy := v.GetFeature(policy.ManagerType()).(policy.Manager)
	if len(tag) > 0 && policy.ForSystem().Stats.OutboundUplink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "outbound>>>" + tag + ">>>traffic>>>uplink"
		c, _ := stats.GetOrRegisterCounter(statsManager, name)
		if c != nil {
			uplinkCounter = c
		}
	}
	if len(tag) > 0 && policy.ForSystem().Stats.OutboundDownlink {
		statsManager := v.GetFeature(stats.ManagerType()).(stats.Manager)
		name := "outbound>>>" + tag + ">>>traffic>>>downlink"
		c, _ := stats.GetOrRegisterCounter(statsManager, name)
		if c != nil {
			downlinkCounter = c
		}
	}

	return uplinkCounter, downlinkCounter
}

// Handler implements outbound.Handler.
type Handler struct {
	tag             string
	senderSettings  *proxyman.SenderConfig
	streamSettings  *internet.MemoryStreamConfig
	proxyConfig     proto.Message
	proxy           proxy.Outbound
	outboundManager outbound.Manager
	mux             *mux.ClientManager
	xudp            *mux.ClientManager
	udp443          string
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter
}

// NewHandler creates a new Handler based on the given configuration.
func NewHandler(ctx context.Context, config *core.OutboundHandlerConfig) (outbound.Handler, error) {
	v := core.MustFromContext(ctx)
	uplinkCounter, downlinkCounter := getStatCounter(v, config.Tag)
	h := &Handler{
		tag:             config.Tag,
		outboundManager: v.GetFeature(outbound.ManagerType()).(outbound.Manager),
		uplinkCounter:   uplinkCounter,
		downlinkCounter: downlinkCounter,
	}

	if config.SenderSettings != nil {
		senderSettings, err := config.SenderSettings.GetInstance()
		if err != nil {
			return nil, err
		}
		switch s := senderSettings.(type) {
		case *proxyman.SenderConfig:
			h.senderSettings = s
			mss, err := internet.ToMemoryStreamConfig(s.StreamSettings)
			if err != nil {
				return nil, errors.New("failed to parse stream settings").Base(err).AtWarning()
			}
			h.streamSettings = mss
		default:
			return nil, errors.New("settings is not SenderConfig")
		}
	}

	proxyConfig, err := config.ProxySettings.GetInstance()
	if err != nil {
		return nil, err
	}
	h.proxyConfig = proxyConfig

	ctx = session.ContextWithFullHandler(ctx, h)

	rawProxyHandler, err := common.CreateObject(ctx, proxyConfig)
	if err != nil {
		return nil, err
	}

	proxyHandler, ok := rawProxyHandler.(proxy.Outbound)
	if !ok {
		return nil, errors.New("not an outbound handler")
	}

	if h.senderSettings != nil && h.senderSettings.MultiplexSettings != nil {
		if config := h.senderSettings.MultiplexSettings; config.Enabled {
			if config.Concurrency < 0 {
				h.mux = &mux.ClientManager{Enabled: false}
			}
			if config.Concurrency == 0 {
				config.Concurrency = 8 // same as before
			}
			if config.Concurrency > 0 {
				h.mux = &mux.ClientManager{
					Enabled: true,
					Picker: &mux.IncrementalWorkerPicker{
						Factory: &mux.DialingWorkerFactory{
							Proxy:  proxyHandler,
							Dialer: h,
							Strategy: mux.ClientStrategy{
								MaxConcurrency: uint32(config.Concurrency),
								MaxConnection:  128,
							},
						},
					},
				}
			}
			if config.XudpConcurrency < 0 {
				h.xudp = &mux.ClientManager{Enabled: false}
			}
			if config.XudpConcurrency == 0 {
				h.xudp = nil // same as before
			}
			if config.XudpConcurrency > 0 {
				h.xudp = &mux.ClientManager{
					Enabled: true,
					Picker: &mux.IncrementalWorkerPicker{
						Factory: &mux.DialingWorkerFactory{
							Proxy:  proxyHandler,
							Dialer: h,
							Strategy: mux.ClientStrategy{
								MaxConcurrency: uint32(config.XudpConcurrency),
								MaxConnection:  128,
							},
						},
					},
				}
			}
			h.udp443 = config.XudpProxyUDP443
		}
	}

	h.proxy = proxyHandler
	return h, nil
}

// Tag implements outbound.Handler.
func (h *Handler) Tag() string {
	return h.tag
}

// Dispatch implements proxy.Outbound.Dispatch.
func (h *Handler) Dispatch(ctx context.Context, link *transport.Link) {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	content := session.ContentFromContext(ctx)
	if h.senderSettings != nil && h.senderSettings.TargetStrategy.HasStrategy() && ob.Target.Address.Family().IsDomain() && (content == nil || !content.SkipDNSResolve) {
		strategy := h.senderSettings.TargetStrategy
		if ob.Target.Network == net.Network_UDP && ob.OriginalTarget.Address != nil {
			strategy = strategy.GetDynamicStrategy(ob.OriginalTarget.Address.Family())
		}
		ips, err := internet.LookupForIP(ob.Target.Address.Domain(), strategy, nil)
		if err != nil {
			errors.LogInfoInner(ctx, err, "failed to resolve ip for target ", ob.Target.Address.Domain())
			if h.senderSettings.TargetStrategy.ForceIP() {
				err := errors.New("failed to resolve ip for target ", ob.Target.Address.Domain()).Base(err)
				session.SubmitOutboundErrorToOriginator(ctx, err)
				common.Interrupt(link.Writer)
				common.Interrupt(link.Reader)
				return
			}

		} else {
			unchangedDomain := ob.Target.Address.Domain()
			ob.Target.Address = net.IPAddress(ips[dice.Roll(len(ips))])
			errors.LogInfo(ctx, "target: ", unchangedDomain, " resolved to: ", ob.Target.Address.String())
		}
	}
	if ob.Target.Network == net.Network_UDP && ob.OriginalTarget.Address != nil && ob.OriginalTarget.Address != ob.Target.Address {
		link.Reader = &buf.EndpointOverrideReader{Reader: link.Reader, Dest: ob.Target.Address, OriginalDest: ob.OriginalTarget.Address}
		link.Writer = &buf.EndpointOverrideWriter{Writer: link.Writer, Dest: ob.Target.Address, OriginalDest: ob.OriginalTarget.Address}
	}
	if h.mux != nil {
		test := func(err error) {
			if err != nil {
				err := errors.New("failed to process mux outbound traffic").Base(err)
				session.SubmitOutboundErrorToOriginator(ctx, err)
				errors.LogInfo(ctx, err.Error())
				common.Interrupt(link.Writer)
				common.Interrupt(link.Reader)
			}
		}
		if ob.Target.Network == net.Network_UDP && ob.Target.Port == 443 {
			switch h.udp443 {
			case "reject":
				test(errors.New("XUDP rejected UDP/443 traffic").AtInfo())
				return
			case "skip":
				goto out
			}
		}
		if h.xudp != nil && ob.Target.Network == net.Network_UDP {
			if !h.xudp.Enabled {
				goto out
			}
			test(h.xudp.Dispatch(ctx, link))
			return
		}
		if h.mux.Enabled {
			test(h.mux.Dispatch(ctx, link))
			return
		}
	}
out:
	err := h.proxy.Process(ctx, link, h)
	var errC error
	if err != nil {
		errC = errors.Cause(err)
		if goerrors.Is(errC, io.EOF) || goerrors.Is(errC, io.ErrClosedPipe) || goerrors.Is(errC, context.Canceled) {
			err = nil
		}
	}
	if err != nil {
		// Ensure outbound ray is properly closed.
		err := errors.New("failed to process outbound traffic").Base(err)
		session.SubmitOutboundErrorToOriginator(ctx, err)
		errors.LogInfo(ctx, err.Error())
		common.Interrupt(link.Writer)
	} else {
		if errC != nil && goerrors.Is(errC, io.ErrClosedPipe) {
			common.Interrupt(link.Writer)
		} else {
			common.Close(link.Writer)
		}
	}
	common.Interrupt(link.Reader)
}

func (h *Handler) DestIpAddress() net.IP {
	return internet.DestIpAddress()
}

// Dial implements internet.Dialer.
func (h *Handler) Dial(ctx context.Context, dest net.Destination) (stat.Connection, error) {
	if h.senderSettings != nil {

		if h.senderSettings.ProxySettings.HasTag() {

			tag := h.senderSettings.ProxySettings.Tag
			handler := h.outboundManager.GetHandler(tag)
			if handler != nil {
				errors.LogDebug(ctx, "proxying to ", tag, " for dest ", dest)
				outbounds := session.OutboundsFromContext(ctx)
				ctx = session.ContextWithOutbounds(ctx, append(outbounds, &session.Outbound{
					Target: dest,
					Tag:    tag,
				})) // add another outbound in session ctx
				opts := pipe.OptionsFromContext(ctx)
				uplinkReader, uplinkWriter := pipe.New(opts...)
				downlinkReader, downlinkWriter := pipe.New(opts...)

				go handler.Dispatch(ctx, &transport.Link{Reader: uplinkReader, Writer: downlinkWriter})
				conn := cnc.NewConnection(cnc.ConnectionInputMulti(uplinkWriter), cnc.ConnectionOutputMulti(downlinkReader))

				if config := tls.ConfigFromStreamSettings(h.streamSettings); config != nil {
					tlsConfig := config.GetTLSConfig(tls.WithDestination(dest))
					conn = tls.Client(conn, tlsConfig)
				}

				return h.getStatCouterConnection(conn), nil
			}

			errors.LogError(ctx, "failed to get outbound handler with tag: ", tag)
			return nil, errors.New("failed to get outbound handler with tag: " + tag)
		}

		if h.senderSettings.Via != nil {
			outbounds := session.OutboundsFromContext(ctx)
			ob := outbounds[len(outbounds)-1]
			h.SetOutboundGateway(ctx, ob)
		}

	}

	if conn, err := h.getUoTConnection(ctx, dest); err != os.ErrInvalid {
		return conn, err
	}

	conn, err := internet.Dial(ctx, dest, h.streamSettings)
	conn = h.getStatCouterConnection(conn)
	outbounds := session.OutboundsFromContext(ctx)
	if outbounds != nil {
		ob := outbounds[len(outbounds)-1]
		ob.Conn = conn
	} else {
		// for Vision's pre-connect
	}
	return conn, err
}

func (h *Handler) SetOutboundGateway(ctx context.Context, ob *session.Outbound) {
	if ob.Gateway == nil && h.senderSettings != nil && h.senderSettings.Via != nil && !h.senderSettings.ProxySettings.HasTag() && (h.streamSettings.SocketSettings == nil || len(h.streamSettings.SocketSettings.DialerProxy) == 0) {
		var domain string
		addr := h.senderSettings.Via.AsAddress()
		domain = h.senderSettings.Via.GetDomain()
		switch {
		case h.senderSettings.ViaCidr != "":
			ob.Gateway = ParseRandomIP(addr, h.senderSettings.ViaCidr)

		case domain == "origin":
			if inbound := session.InboundFromContext(ctx); inbound != nil {
				if inbound.Local.IsValid() && inbound.Local.Address.Family().IsIP() {
					ob.Gateway = inbound.Local.Address
					errors.LogDebug(ctx, "use inbound local ip as sendthrough: ", inbound.Local.Address.String())
				}
			}
		case domain == "srcip":
			if inbound := session.InboundFromContext(ctx); inbound != nil {
				if inbound.Source.IsValid() && inbound.Source.Address.Family().IsIP() {
					ob.Gateway = inbound.Source.Address
					errors.LogDebug(ctx, "use inbound source ip as sendthrough: ", inbound.Source.Address.String())
				}
			}
		//case addr.Family().IsDomain():
		default:
			ob.Gateway = addr

		}

	}
}

func (h *Handler) getStatCouterConnection(conn stat.Connection) stat.Connection {
	if h.uplinkCounter != nil || h.downlinkCounter != nil {
		return &stat.CounterConnection{
			Connection:   conn,
			ReadCounter:  h.downlinkCounter,
			WriteCounter: h.uplinkCounter,
		}
	}
	return conn
}

// GetOutbound implements proxy.GetOutbound.
func (h *Handler) GetOutbound() proxy.Outbound {
	return h.proxy
}

// Start implements common.Runnable.
func (h *Handler) Start() error {
	return nil
}

// Close implements common.Closable.
func (h *Handler) Close() error {
	common.Close(h.mux)
	common.Close(h.proxy)
	return nil
}

// SenderSettings implements outbound.Handler.
func (h *Handler) SenderSettings() *serial.TypedMessage {
	return serial.ToTypedMessage(h.senderSettings)
}

// ProxySettings implements outbound.Handler.
func (h *Handler) ProxySettings() *serial.TypedMessage {
	return serial.ToTypedMessage(h.proxyConfig)
}

func ParseRandomIP(addr net.Address, prefix string) net.Address {

	_, ipnet, _ := net.ParseCIDR(addr.IP().String() + "/" + prefix)

	ones, bits := ipnet.Mask.Size()
	subnetSize := new(big.Int).Lsh(big.NewInt(1), uint(bits-ones))

	rnd, _ := rand.Int(rand.Reader, subnetSize)

	startInt := new(big.Int).SetBytes(ipnet.IP)
	rndInt := new(big.Int).Add(startInt, rnd)

	rndBytes := rndInt.Bytes()
	padded := make([]byte, len(ipnet.IP))
	copy(padded[len(padded)-len(rndBytes):], rndBytes)

	return net.ParseAddress(net.IP(padded).String())
}
