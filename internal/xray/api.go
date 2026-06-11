// Package xray provides integration with the Xray proxy core.
// It includes API client functionality, configuration management, traffic monitoring,
// and process control for Xray instances.
package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/xtls/xray-core/app/proxyman/command"
	routerService "github.com/xtls/xray-core/app/router/command"
	statsService "github.com/xtls/xray-core/app/stats/command"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/infra/conf"
	hysteriaAccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// XrayAPI is a gRPC client for managing Xray core configuration, inbounds, outbounds, and statistics.
type XrayAPI struct {
	HandlerServiceClient *command.HandlerServiceClient
	StatsServiceClient   *statsService.StatsServiceClient
	RoutingServiceClient *routerService.RoutingServiceClient
	grpcClient           *grpc.ClientConn
	isConnected          bool
	StatsLastValues      map[string]int64
}

func getRequiredUserString(user map[string]any, key string) (string, error) {
	value, ok := user[key]
	if !ok || value == nil {
		return "", fmt.Errorf("missing required user field %q", key)
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for user field %q: %T", key, value)
	}

	return strValue, nil
}

func getOptionalUserString(user map[string]any, key string) (string, error) {
	value, ok := user[key]
	if !ok || value == nil {
		return "", nil
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for user field %q: %T", key, value)
	}

	return strValue, nil
}

// Init connects to the Xray API server and initializes handler and stats service clients.
func (x *XrayAPI) Init(apiPort int) error {
	if apiPort <= 0 || apiPort > math.MaxUint16 {
		return fmt.Errorf("invalid Xray API port: %d", apiPort)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", apiPort)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to Xray API: %w", err)
	}

	x.grpcClient = conn
	x.isConnected = true
	if x.StatsLastValues == nil {
		x.StatsLastValues = make(map[string]int64)
	}

	hsClient := command.NewHandlerServiceClient(conn)
	ssClient := statsService.NewStatsServiceClient(conn)
	rsClient := routerService.NewRoutingServiceClient(conn)

	x.HandlerServiceClient = &hsClient
	x.StatsServiceClient = &ssClient
	x.RoutingServiceClient = &rsClient

	return nil
}

// Close closes the gRPC connection and resets the XrayAPI client state.
func (x *XrayAPI) Close() {
	if x.grpcClient != nil {
		x.grpcClient.Close()
	}
	x.HandlerServiceClient = nil
	x.StatsServiceClient = nil
	x.RoutingServiceClient = nil
	x.isConnected = false
}

// AddInbound adds a new inbound configuration to the Xray core via gRPC.
func (x *XrayAPI) AddInbound(inbound []byte) error {
	client := *x.HandlerServiceClient

	conf := new(conf.InboundDetourConfig)
	err := json.Unmarshal(inbound, conf)
	if err != nil {
		logger.Debug("Failed to unmarshal inbound:", err)
		return err
	}
	config, err := conf.Build()
	if err != nil {
		logger.Debug("Failed to build inbound Detur:", err)
		return err
	}
	inboundConfig := command.AddInboundRequest{Inbound: config}

	_, err = client.AddInbound(context.Background(), &inboundConfig)

	return err
}

// DelInbound removes an inbound configuration from the Xray core by tag.
func (x *XrayAPI) DelInbound(tag string) error {
	client := *x.HandlerServiceClient
	_, err := client.RemoveInbound(context.Background(), &command.RemoveInboundRequest{
		Tag: tag,
	})
	return err
}

// AddOutbound adds a new outbound configuration to the Xray core via gRPC.
func (x *XrayAPI) AddOutbound(outbound []byte) error {
	if x.HandlerServiceClient == nil {
		return common.NewError("xray HandlerServiceClient is not initialized")
	}
	client := *x.HandlerServiceClient

	conf := new(conf.OutboundDetourConfig)
	if err := json.Unmarshal(outbound, conf); err != nil {
		logger.Debug("Failed to unmarshal outbound:", err)
		return err
	}
	config, err := conf.Build()
	if err != nil {
		logger.Debug("Failed to build outbound detour:", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.AddOutbound(ctx, &command.AddOutboundRequest{Outbound: config})
	return err
}

// DelOutbound removes an outbound configuration from the Xray core by tag.
func (x *XrayAPI) DelOutbound(tag string) error {
	if x.HandlerServiceClient == nil {
		return common.NewError("xray HandlerServiceClient is not initialized")
	}
	client := *x.HandlerServiceClient

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.RemoveOutbound(ctx, &command.RemoveOutboundRequest{Tag: tag})
	return err
}

// ApplyRoutingConfig replaces the routing rules and balancers of the running
// Xray core with the given routing section (the JSON value of the top-level
// "routing" key) via the RoutingService gRPC API. Note that this cannot change
// routing.domainStrategy/domainMatcher — those are fixed at process start.
func (x *XrayAPI) ApplyRoutingConfig(routing []byte) error {
	if x.RoutingServiceClient == nil {
		return common.NewError("xray RoutingServiceClient is not initialized")
	}

	// Rules referencing geoip:/geosite: need the dat files; point xray-core's
	// in-process loader at the panel's bin folder where they live.
	ensureXrayAssetLocation()

	routerConf := new(conf.RouterConfig)
	if err := json.Unmarshal(routing, routerConf); err != nil {
		logger.Debug("Failed to unmarshal routing config:", err)
		return err
	}
	config, err := routerConf.Build()
	if err != nil {
		logger.Debug("Failed to build routing config:", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = (*x.RoutingServiceClient).AddRule(ctx, &routerService.AddRuleRequest{
		ShouldAppend: false,
		Config:       serial.ToTypedMessage(config),
	})
	return err
}

// BalancerInfo is the live state of one balancer inside the running core.
type BalancerInfo struct {
	Tag string `json:"tag"`
	// Override is the outbound tag an admin forced via the API; empty when
	// the strategy is in control.
	Override string `json:"override"`
	// Selected are the outbound tags the strategy currently prefers, best
	// first (xray's "principle target" list).
	Selected []string `json:"selected"`
}

// GetBalancerInfo queries the running core for a balancer's current override
// and the targets its strategy would pick right now.
func (x *XrayAPI) GetBalancerInfo(tag string) (*BalancerInfo, error) {
	if x.RoutingServiceClient == nil {
		return nil, common.NewError("xray RoutingServiceClient is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := (*x.RoutingServiceClient).GetBalancerInfo(ctx, &routerService.GetBalancerInfoRequest{Tag: tag})
	if err != nil {
		return nil, err
	}

	info := &BalancerInfo{Tag: tag}
	if balancer := resp.GetBalancer(); balancer != nil {
		if balancer.Override != nil {
			info.Override = balancer.Override.Target
		}
		if balancer.PrincipleTarget != nil {
			info.Selected = balancer.PrincipleTarget.Tag
		}
	}
	return info, nil
}

// SetBalancerTarget forces a balancer to always pick the given outbound tag.
// An empty target clears the override and hands control back to the strategy.
func (x *XrayAPI) SetBalancerTarget(tag, target string) error {
	if x.RoutingServiceClient == nil {
		return common.NewError("xray RoutingServiceClient is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := (*x.RoutingServiceClient).OverrideBalancerTarget(ctx, &routerService.OverrideBalancerTargetRequest{
		BalancerTag: tag,
		Target:      target,
	})
	return err
}

// RouteTestRequest describes a synthetic connection to ask the running core
// which outbound its router would pick for it.
type RouteTestRequest struct {
	InboundTag string // optional: simulate arrival on this inbound
	Domain     string // target domain (sniffed/SOCKS-style destination)
	IP         string // target IP, used when Domain is empty or alongside it
	Port       int
	Network    string // "tcp" (default) or "udp"
	Protocol   string // optional sniffed protocol: http, tls, bittorrent, ...
	Email      string // optional user attribution for user-based rules
}

// RouteTestResult is the routing decision the core reported.
type RouteTestResult struct {
	// Matched is false when no routing rule matched — traffic would use the
	// default (first) outbound and OutboundTag is empty.
	Matched     bool     `json:"matched"`
	OutboundTag string   `json:"outboundTag"`
	// GroupTags lists the balancer chain the decision went through, when any.
	GroupTags []string `json:"groupTags,omitempty"`
}

// TestRoute asks the running core's router which outbound it would pick for
// the described connection, without sending any traffic.
func (x *XrayAPI) TestRoute(req RouteTestRequest) (*RouteTestResult, error) {
	if x.RoutingServiceClient == nil {
		return nil, common.NewError("xray RoutingServiceClient is not initialized")
	}

	network := xnet.Network_TCP
	if strings.EqualFold(req.Network, "udp") {
		network = xnet.Network_UDP
	}
	rc := &routerService.RoutingContext{
		InboundTag:   req.InboundTag,
		Network:      network,
		TargetDomain: req.Domain,
		TargetPort:   uint32(req.Port),
		Protocol:     req.Protocol,
		User:         req.Email,
	}
	if req.IP != "" {
		parsed := net.ParseIP(req.IP)
		if parsed == nil {
			return nil, common.NewErrorf("invalid IP address: %s", req.IP)
		}
		if v4 := parsed.To4(); v4 != nil {
			rc.TargetIPs = [][]byte{v4}
		} else {
			rc.TargetIPs = [][]byte{parsed.To16()}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := (*x.RoutingServiceClient).TestRoute(ctx, &routerService.TestRouteRequest{
		RoutingContext: rc,
		PublishResult:  false,
	})
	if err != nil {
		// The router reports "no rule matched" as an error; for the caller
		// that simply means the default outbound takes the traffic.
		if strings.Contains(strings.ToLower(err.Error()), "not enough information") {
			return &RouteTestResult{Matched: false}, nil
		}
		return nil, err
	}

	return &RouteTestResult{
		Matched:     true,
		OutboundTag: resp.GetOutboundTag(),
		GroupTags:   resp.GetOutboundGroupTags(),
	}, nil
}

// IsMissingHandlerErr reports whether err is xray's response to removing a
// handler (inbound/outbound) that does not exist — e.g. it was already
// removed through the runtime API while the panel's config snapshot was
// stale. Safe to treat as success for removal operations.
func IsMissingHandlerErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "not enough information")
}

// IsExistingTagErr reports whether err is xray's response to adding a handler
// whose tag is already taken by a running handler.
func IsExistingTagErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "existing tag")
}

// ensureXrayAssetLocation makes geoip.dat/geosite.dat resolvable when xray-core
// config builders run inside the panel process. The xray binary resolves assets
// relative to its own executable, but the panel binary lives one level above
// the bin folder, so an explicit location is required.
func ensureXrayAssetLocation() {
	if os.Getenv("XRAY_LOCATION_ASSET") != "" || os.Getenv("xray.location.asset") != "" {
		return
	}
	if abs, err := filepath.Abs(config.GetBinFolderPath()); err == nil {
		os.Setenv("XRAY_LOCATION_ASSET", abs)
	}
}

// AddUser adds a user to an inbound in the Xray core using the specified protocol and user data.
func (x *XrayAPI) AddUser(Protocol string, inboundTag string, user map[string]any) error {
	userEmail, err := getRequiredUserString(user, "email")
	if err != nil {
		return err
	}

	var account *serial.TypedMessage
	switch Protocol {
	case "vmess":
		userID, err := getRequiredUserString(user, "id")
		if err != nil {
			return err
		}

		account = serial.ToTypedMessage(&vmess.Account{
			Id: userID,
		})
	case "vless":
		userID, err := getRequiredUserString(user, "id")
		if err != nil {
			return err
		}

		userFlow, err := getOptionalUserString(user, "flow")
		if err != nil {
			return err
		}

		vlessAccount := &vless.Account{
			Id:   userID,
			Flow: userFlow,
		}
		// Add testseed if provided
		if testseedVal, ok := user["testseed"]; ok {
			if testseedArr, ok := testseedVal.([]any); ok && len(testseedArr) >= 4 {
				testseed := make([]uint32, len(testseedArr))
				for i, v := range testseedArr {
					if num, ok := v.(float64); ok {
						testseed[i] = uint32(num)
					}
				}
				vlessAccount.Testseed = testseed
			} else if testseedArr, ok := testseedVal.([]uint32); ok && len(testseedArr) >= 4 {
				vlessAccount.Testseed = testseedArr
			}
		}
		// Add testpre if provided (for outbound, but can be in user for compatibility)
		if testpreVal, ok := user["testpre"]; ok {
			if testpre, ok := testpreVal.(float64); ok && testpre > 0 {
				vlessAccount.Testpre = uint32(testpre)
			} else if testpre, ok := testpreVal.(uint32); ok && testpre > 0 {
				vlessAccount.Testpre = testpre
			}
		}
		account = serial.ToTypedMessage(vlessAccount)
	case "trojan":
		password, err := getRequiredUserString(user, "password")
		if err != nil {
			return err
		}

		account = serial.ToTypedMessage(&trojan.Account{
			Password: password,
		})
	case "shadowsocks":
		cipher, err := getOptionalUserString(user, "cipher")
		if err != nil {
			return err
		}

		password, err := getRequiredUserString(user, "password")
		if err != nil {
			return err
		}

		var ssCipherType shadowsocks.CipherType
		switch cipher {
		case "aes-256-gcm":
			ssCipherType = shadowsocks.CipherType_AES_256_GCM
		case "chacha20-poly1305", "chacha20-ietf-poly1305":
			ssCipherType = shadowsocks.CipherType_CHACHA20_POLY1305
		case "xchacha20-poly1305", "xchacha20-ietf-poly1305":
			ssCipherType = shadowsocks.CipherType_XCHACHA20_POLY1305
		default:
			ssCipherType = shadowsocks.CipherType_NONE
		}

		if ssCipherType != shadowsocks.CipherType_NONE {
			account = serial.ToTypedMessage(&shadowsocks.Account{
				Password:   password,
				CipherType: ssCipherType,
			})
		} else {
			account = serial.ToTypedMessage(&shadowsocks_2022.ServerConfig{
				Key:   password,
				Email: userEmail,
			})
		}
	case "hysteria":
		auth, err := getRequiredUserString(user, "auth")
		if err != nil {
			return err
		}

		account = serial.ToTypedMessage(&hysteriaAccount.Account{
			Auth: auth,
		})
	default:
		return nil
	}

	client := *x.HandlerServiceClient

	_, err = client.AlterInbound(context.Background(), &command.AlterInboundRequest{
		Tag: inboundTag,
		Operation: serial.ToTypedMessage(&command.AddUserOperation{
			User: &protocol.User{
				Email:   userEmail,
				Account: account,
			},
		}),
	})
	return err
}

// RemoveUser removes a user from an inbound in the Xray core by email.
func (x *XrayAPI) RemoveUser(inboundTag, email string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	op := &command.RemoveUserOperation{Email: email}
	req := &command.AlterInboundRequest{
		Tag:       inboundTag,
		Operation: serial.ToTypedMessage(op),
	}

	_, err := (*x.HandlerServiceClient).AlterInbound(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}

	return nil
}

// GetTraffic queries traffic statistics from the Xray core, optionally resetting counters.
func (x *XrayAPI) GetTraffic() ([]*Traffic, []*ClientTraffic, error) {
	if x.grpcClient == nil {
		return nil, nil, common.NewError("xray api is not initialized")
	}

	trafficRegex := regexp.MustCompile(`(inbound|outbound)>>>([^>]+)>>>traffic>>>(downlink|uplink)`)
	clientTrafficRegex := regexp.MustCompile(`user>>>([^>]+)>>>traffic>>>(downlink|uplink)`)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if x.StatsServiceClient == nil {
		return nil, nil, common.NewError("xray StatusServiceClient is not initialized")
	}

	resp, err := (*x.StatsServiceClient).QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: false})
	if err != nil {
		logger.Debug("Failed to query Xray stats:", err)
		return nil, nil, err
	}

	tagTrafficMap := make(map[string]*Traffic)
	emailTrafficMap := make(map[string]*ClientTraffic)

	for _, stat := range resp.GetStat() {
		lastValue, ok := x.StatsLastValues[stat.Name]
		x.StatsLastValues[stat.Name] = stat.Value
		if !ok || stat.Value < lastValue {
			// skip first time of seen stat
			continue
		}
		value := stat.Value - lastValue
		if matches := trafficRegex.FindStringSubmatch(stat.Name); len(matches) == 4 {
			processTraffic(matches, value, tagTrafficMap)
		} else if matches := clientTrafficRegex.FindStringSubmatch(stat.Name); len(matches) == 3 {
			processClientTraffic(matches, value, emailTrafficMap)
		}
	}
	return mapToSlice(tagTrafficMap), mapToSlice(emailTrafficMap), nil
}

// processTraffic aggregates a traffic stat into trafficMap using regex matches and value.
func processTraffic(matches []string, value int64, trafficMap map[string]*Traffic) {
	isInbound := matches[1] == "inbound"
	tag := matches[2]
	isDown := matches[3] == "downlink"

	if tag == "api" {
		return
	}

	traffic, ok := trafficMap[tag]
	if !ok {
		traffic = &Traffic{
			IsInbound:  isInbound,
			IsOutbound: !isInbound,
			Tag:        tag,
		}
		trafficMap[tag] = traffic
	}

	if isDown {
		traffic.Down = value
	} else {
		traffic.Up = value
	}
}

// processClientTraffic updates clientTrafficMap with upload/download values for a client email.
func processClientTraffic(matches []string, value int64, clientTrafficMap map[string]*ClientTraffic) {
	email := matches[1]
	isDown := matches[2] == "downlink"

	traffic, ok := clientTrafficMap[email]
	if !ok {
		traffic = &ClientTraffic{Email: email}
		clientTrafficMap[email] = traffic
	}

	if isDown {
		traffic.Down = value
	} else {
		traffic.Up = value
	}
}

// mapToSlice converts a map of pointers to a slice of pointers.
func mapToSlice[T any](m map[string]*T) []*T {
	result := make([]*T, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}
