package service

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/goccy/go-json"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/infra/conf"

	tcplimit "github.com/mhsanaei/3x-ui/v3/internal/tcp-limit"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var tcpConnectionLimiter interface {
	Apply(int, []tcplimit.Target) error
	Disable() error
	Stage(int, []tcplimit.Target) (func() error, error)
} = tcplimit.NewManager()

// WithTCPConnectionLimit covers ordinary gRPC inbound mutations, which do not
// necessarily pass through RestartXray. The DB already holds the desired ports.
func WithTCPConnectionLimit(apply func() error) error {
	lock.Lock()
	defer lock.Unlock()
	s := &XrayService{}
	limit, err := s.settingService.GetTCPPerIPConnectionLimit()
	if err != nil {
		return err
	}
	if limit == 0 {
		return apply()
	}
	cfg, err := s.getXrayConfig()
	if err != nil {
		return err
	}
	rollback, err := stageTCPConnectionLimitToConfig(limit, cfg)
	if err != nil {
		return err
	}
	if err := apply(); err != nil {
		if rollbackErr := rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("restore TCP connection limit: %w", rollbackErr))
		}
		return err
	}
	return nil
}

// ApplyTCPConnectionLimit applies a proposed setting before it is saved. The
// caller holds the service lock through the settings transaction and rollback.
func ApplyTCPConnectionLimit(limit int) error {
	if limit == 0 {
		oldLimit, err := (&SettingService{}).GetTCPPerIPConnectionLimit()
		if err != nil {
			return err
		}
		if oldLimit > 0 {
			return tcpConnectionLimiter.Disable()
		}
		return tcpConnectionLimiter.Apply(0, nil)
	}
	cfg, err := (&XrayService{}).getXrayConfig()
	if err != nil {
		return err
	}
	ports, err := tcpConnectionLimitTargets(cfg)
	if err != nil {
		return err
	}
	return tcpConnectionLimiter.Apply(limit, ports)
}

func stageTCPConnectionLimitToConfig(limit int, cfg *xray.Config) (func() error, error) {
	if limit == 0 {
		return tcpConnectionLimiter.Stage(0, nil)
	}
	ports, err := tcpConnectionLimitTargets(cfg)
	if err != nil {
		return nil, err
	}
	return tcpConnectionLimiter.Stage(limit, ports)
}

// tcpConnectionLimitTargets excludes local relay/API listeners and UDP transports.
// Remote-node inbounds are already absent from the local generated config.
func tcpConnectionLimitTargets(cfg *xray.Config) ([]tcplimit.Target, error) {
	var targets []tcplimit.Target
	for _, inbound := range cfg.InboundConfigs {
		switch inbound.Protocol {
		case "vless", "vmess", "trojan", "shadowsocks", "http", "socks", "mixed":
		default:
			continue
		}
		if inbound.Port < 1 || inbound.Port > 65535 {
			continue
		}
		if inbound.Protocol == "shadowsocks" {
			var settings struct {
				Network *conf.NetworkList `json:"network"`
			}
			if err := json.Unmarshal(inbound.Settings, &settings); err != nil {
				return nil, fmt.Errorf("TCP connection limit: inbound %s settings: %w", inbound.Tag, err)
			}
			if !xnet.HasNetwork(settings.Network.Build(), xnet.Network_TCP) {
				continue
			}
		}
		var listen string
		if len(inbound.Listen) > 0 {
			if err := json.Unmarshal(inbound.Listen, &listen); err != nil {
				return nil, fmt.Errorf("TCP connection limit: inbound %s listen: %w", inbound.Tag, err)
			}
		}
		var listenAddress netip.Addr
		if listen != "" {
			addr, err := netip.ParseAddr(listen)
			if err != nil || addr.Unmap().IsLoopback() {
				continue
			}
			if !addr.IsUnspecified() {
				listenAddress = addr.Unmap()
			}
		}
		var stream struct {
			Network string `json:"network"`
			Sockopt struct {
				V6Only bool `json:"v6only"`
			} `json:"sockopt"`
		}
		if len(inbound.StreamSettings) > 0 {
			if err := json.Unmarshal(inbound.StreamSettings, &stream); err != nil {
				return nil, fmt.Errorf("TCP connection limit: inbound %s transport: %w", inbound.Tag, err)
			}
		}
		if stream.Sockopt.V6Only && !listenAddress.IsValid() {
			listenAddress = netip.IPv6Unspecified()
		}
		switch stream.Network {
		case "", "tcp", "raw", "ws", "websocket", "http", "h2", "grpc", "httpupgrade", "splithttp", "xhttp":
			targets = append(targets, tcplimit.Target{Address: listenAddress, Port: uint16(inbound.Port)})
		}
	}
	return targets, nil
}
