package conf

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xtls/xray-core/app/dispatcher"
	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/app/stats"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/serial"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/transport/internet"
)

var (
	inboundConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"tunnel":        func() interface{} { return new(DokodemoConfig) },
		"dokodemo-door": func() interface{} { return new(DokodemoConfig) },
		"http":          func() interface{} { return new(HTTPServerConfig) },
		"shadowsocks":   func() interface{} { return new(ShadowsocksServerConfig) },
		"mixed":         func() interface{} { return new(SocksServerConfig) },
		"socks":         func() interface{} { return new(SocksServerConfig) },
		"vless":         func() interface{} { return new(VLessInboundConfig) },
		"vmess":         func() interface{} { return new(VMessInboundConfig) },
		"trojan":        func() interface{} { return new(TrojanServerConfig) },
		"wireguard":     func() interface{} { return &WireGuardConfig{IsClient: false} },
		"hysteria":      func() interface{} { return new(HysteriaServerConfig) },
		"tun":           func() interface{} { return new(TunConfig) },
	}, "protocol", "settings")

	outboundConfigLoader = NewJSONConfigLoader(ConfigCreatorCache{
		"block":       func() interface{} { return new(BlackholeConfig) },
		"blackhole":   func() interface{} { return new(BlackholeConfig) },
		"loopback":    func() interface{} { return new(LoopbackConfig) },
		"direct":      func() interface{} { return new(FreedomConfig) },
		"freedom":     func() interface{} { return new(FreedomConfig) },
		"http":        func() interface{} { return new(HTTPClientConfig) },
		"shadowsocks": func() interface{} { return new(ShadowsocksClientConfig) },
		"socks":       func() interface{} { return new(SocksClientConfig) },
		"vless":       func() interface{} { return new(VLessOutboundConfig) },
		"vmess":       func() interface{} { return new(VMessOutboundConfig) },
		"trojan":      func() interface{} { return new(TrojanClientConfig) },
		"hysteria":    func() interface{} { return new(HysteriaClientConfig) },
		"dns":         func() interface{} { return new(DNSOutboundConfig) },
		"wireguard":   func() interface{} { return &WireGuardConfig{IsClient: true} },
	}, "protocol", "settings")
)

type SniffingConfig struct {
	Enabled         bool        `json:"enabled"`
	DestOverride    *StringList `json:"destOverride"`
	DomainsExcluded *StringList `json:"domainsExcluded"`
	MetadataOnly    bool        `json:"metadataOnly"`
	RouteOnly       bool        `json:"routeOnly"`
}

// Build implements Buildable.
func (c *SniffingConfig) Build() (*proxyman.SniffingConfig, error) {
	var p []string
	if c.DestOverride != nil {
		for _, protocol := range *c.DestOverride {
			switch strings.ToLower(protocol) {
			case "http":
				p = append(p, "http")
			case "tls", "https", "ssl":
				p = append(p, "tls")
			case "quic":
				p = append(p, "quic")
			case "fakedns", "fakedns+others":
				p = append(p, "fakedns")
			default:
				return nil, errors.New("unknown protocol: ", protocol)
			}
		}
	}

	var d []string
	if c.DomainsExcluded != nil {
		for _, domain := range *c.DomainsExcluded {
			d = append(d, strings.ToLower(domain))
		}
	}

	return &proxyman.SniffingConfig{
		Enabled:             c.Enabled,
		DestinationOverride: p,
		DomainsExcluded:     d,
		MetadataOnly:        c.MetadataOnly,
		RouteOnly:           c.RouteOnly,
	}, nil
}

type MuxConfig struct {
	Enabled         bool   `json:"enabled"`
	Concurrency     int16  `json:"concurrency"`
	XudpConcurrency int16  `json:"xudpConcurrency"`
	XudpProxyUDP443 string `json:"xudpProxyUDP443"`
}

// Build creates MultiplexingConfig, Concurrency < 0 completely disables mux.
func (m *MuxConfig) Build() (*proxyman.MultiplexingConfig, error) {
	switch m.XudpProxyUDP443 {
	case "":
		m.XudpProxyUDP443 = "reject"
	case "reject", "allow", "skip":
	default:
		return nil, errors.New(`unknown "xudpProxyUDP443": `, m.XudpProxyUDP443)
	}
	return &proxyman.MultiplexingConfig{
		Enabled:         m.Enabled,
		Concurrency:     int32(m.Concurrency),
		XudpConcurrency: int32(m.XudpConcurrency),
		XudpProxyUDP443: m.XudpProxyUDP443,
	}, nil
}

type InboundDetourConfig struct {
	Protocol       string           `json:"protocol"`
	PortList       *PortList        `json:"port"`
	ListenOn       *Address         `json:"listen"`
	Settings       *json.RawMessage `json:"settings"`
	Tag            string           `json:"tag"`
	StreamSetting  *StreamConfig    `json:"streamSettings"`
	SniffingConfig *SniffingConfig  `json:"sniffing"`
}

// Build implements Buildable.
func (c *InboundDetourConfig) Build() (*core.InboundHandlerConfig, error) {
	receiverSettings := &proxyman.ReceiverConfig{}

	// TUN inbound doesn't need port configuration as it uses network interface instead
	if strings.ToLower(c.Protocol) == "tun" {
		// Skip port validation for TUN
	} else if c.ListenOn == nil {
		// Listen on anyip, must set PortList
		if c.PortList == nil {
			return nil, errors.New("Listen on AnyIP but no Port(s) set in InboundDetour.")
		}
		receiverSettings.PortList = c.PortList.Build()
	} else {
		// Listen on specific IP or Unix Domain Socket
		receiverSettings.Listen = c.ListenOn.Build()
		listenDS := c.ListenOn.Family().IsDomain() && (filepath.IsAbs(c.ListenOn.Domain()) || c.ListenOn.Domain()[0] == '@')
		listenIP := c.ListenOn.Family().IsIP() || (c.ListenOn.Family().IsDomain() && c.ListenOn.Domain() == "localhost")
		if listenIP {
			// Listen on specific IP, must set PortList
			if c.PortList == nil {
				return nil, errors.New("Listen on specific ip without port in InboundDetour.")
			}
			// Listen on IP:Port
			receiverSettings.PortList = c.PortList.Build()
		} else if listenDS {
			if c.PortList != nil {
				// Listen on Unix Domain Socket, PortList should be nil
				receiverSettings.PortList = nil
			}
		} else {
			return nil, errors.New("unable to listen on domain address: ", c.ListenOn.Domain())
		}
	}

	if c.StreamSetting != nil {
		ss, err := c.StreamSetting.Build()
		if err != nil {
			return nil, err
		}
		receiverSettings.StreamSettings = ss
		if strings.Contains(ss.SecurityType, "reality") && (receiverSettings.PortList == nil ||
			len(receiverSettings.PortList.Ports()) != 1 || receiverSettings.PortList.Ports()[0] != 443) {
			errors.LogWarning(context.Background(), `REALITY: Listening on non-443 ports may get your IP blocked by the GFW`)
		}
	}
	if c.SniffingConfig != nil {
		s, err := c.SniffingConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build sniffing config").Base(err)
		}
		receiverSettings.SniffingSettings = s
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}
	rawConfig, err := inboundConfigLoader.LoadWithID(settings, c.Protocol)
	if err != nil {
		return nil, errors.New("failed to load inbound detour config for protocol ", c.Protocol).Base(err)
	}
	if dokodemoConfig, ok := rawConfig.(*DokodemoConfig); ok {
		receiverSettings.ReceiveOriginalDestination = dokodemoConfig.FollowRedirect
	}
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, errors.New("failed to build inbound handler for protocol ", c.Protocol).Base(err)
	}

	return &core.InboundHandlerConfig{
		Tag:              c.Tag,
		ReceiverSettings: serial.ToTypedMessage(receiverSettings),
		ProxySettings:    serial.ToTypedMessage(ts),
	}, nil
}

type OutboundDetourConfig struct {
	Protocol       string           `json:"protocol"`
	SendThrough    *string          `json:"sendThrough"`
	Tag            string           `json:"tag"`
	Settings       *json.RawMessage `json:"settings"`
	StreamSetting  *StreamConfig    `json:"streamSettings"`
	ProxySettings  *ProxyConfig     `json:"proxySettings"`
	MuxSettings    *MuxConfig       `json:"mux"`
	TargetStrategy string           `json:"targetStrategy"`
}

func (c *OutboundDetourConfig) checkChainProxyConfig() error {
	if c.StreamSetting == nil || c.ProxySettings == nil || c.StreamSetting.SocketSettings == nil {
		return nil
	}
	if len(c.ProxySettings.Tag) > 0 && len(c.StreamSetting.SocketSettings.DialerProxy) > 0 {
		return errors.New("proxySettings.tag is conflicted with sockopt.dialerProxy").AtWarning()
	}
	return nil
}

// Build implements Buildable.
func (c *OutboundDetourConfig) Build() (*core.OutboundHandlerConfig, error) {
	senderSettings := &proxyman.SenderConfig{}
	switch strings.ToLower(c.TargetStrategy) {
	case "asis", "":
		senderSettings.TargetStrategy = internet.DomainStrategy_AS_IS
	case "useip":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP
	case "useipv4":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP4
	case "useipv6":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP6
	case "useipv4v6":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP46
	case "useipv6v4":
		senderSettings.TargetStrategy = internet.DomainStrategy_USE_IP64
	case "forceip":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP
	case "forceipv4":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP4
	case "forceipv6":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP6
	case "forceipv4v6":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP46
	case "forceipv6v4":
		senderSettings.TargetStrategy = internet.DomainStrategy_FORCE_IP64
	default:
		return nil, errors.New("unsupported target domain strategy: ", c.TargetStrategy)
	}
	if err := c.checkChainProxyConfig(); err != nil {
		return nil, err
	}

	if c.SendThrough != nil {
		address := ParseSendThough(c.SendThrough)
		//Check if CIDR exists
		if strings.Contains(*c.SendThrough, "/") {
			senderSettings.ViaCidr = strings.Split(*c.SendThrough, "/")[1]
		} else {
			if address.Family().IsDomain() {
				domain := address.Address.Domain()
				if domain != "origin" && domain != "srcip" {
					return nil, errors.New("unable to send through: " + address.String())
				}
			}
		}
		senderSettings.Via = address.Build()
	}

	if c.StreamSetting != nil {
		ss, err := c.StreamSetting.Build()
		if err != nil {
			return nil, errors.New("failed to build stream settings for outbound detour").Base(err)
		}
		senderSettings.StreamSettings = ss
	}

	if c.ProxySettings != nil {
		ps, err := c.ProxySettings.Build()
		if err != nil {
			return nil, errors.New("invalid outbound detour proxy settings").Base(err)
		}
		if ps.TransportLayerProxy {
			if senderSettings.StreamSettings != nil {
				if senderSettings.StreamSettings.SocketSettings != nil {
					senderSettings.StreamSettings.SocketSettings.DialerProxy = ps.Tag
				} else {
					senderSettings.StreamSettings.SocketSettings = &internet.SocketConfig{DialerProxy: ps.Tag}
				}
			} else {
				senderSettings.StreamSettings = &internet.StreamConfig{SocketSettings: &internet.SocketConfig{DialerProxy: ps.Tag}}
			}
			ps = nil
		}
		senderSettings.ProxySettings = ps
	}

	if c.MuxSettings != nil {
		ms, err := c.MuxSettings.Build()
		if err != nil {
			return nil, errors.New("failed to build Mux config").Base(err)
		}
		senderSettings.MultiplexSettings = ms
	}

	settings := []byte("{}")
	if c.Settings != nil {
		settings = ([]byte)(*c.Settings)
	}
	rawConfig, err := outboundConfigLoader.LoadWithID(settings, c.Protocol)
	if err != nil {
		return nil, errors.New("failed to load outbound detour config for protocol ", c.Protocol).Base(err)
	}
	ts, err := rawConfig.(Buildable).Build()
	if err != nil {
		return nil, errors.New("failed to build outbound handler for protocol ", c.Protocol).Base(err)
	}

	return &core.OutboundHandlerConfig{
		SenderSettings: serial.ToTypedMessage(senderSettings),
		Tag:            c.Tag,
		ProxySettings:  serial.ToTypedMessage(ts),
	}, nil
}

type StatsConfig struct{}

// Build implements Buildable.
func (c *StatsConfig) Build() (*stats.Config, error) {
	return &stats.Config{}, nil
}

type Config struct {
	// Deprecated: Global transport config is no longer used
	// left for returning error
	Transport map[string]json.RawMessage `json:"transport"`

	LogConfig        *LogConfig              `json:"log"`
	RouterConfig     *RouterConfig           `json:"routing"`
	DNSConfig        *DNSConfig              `json:"dns"`
	InboundConfigs   []InboundDetourConfig   `json:"inbounds"`
	OutboundConfigs  []OutboundDetourConfig  `json:"outbounds"`
	Policy           *PolicyConfig           `json:"policy"`
	API              *APIConfig              `json:"api"`
	Metrics          *MetricsConfig          `json:"metrics"`
	Stats            *StatsConfig            `json:"stats"`
	Reverse          *ReverseConfig          `json:"reverse"`
	FakeDNS          *FakeDNSConfig          `json:"fakeDns"`
	Observatory      *ObservatoryConfig      `json:"observatory"`
	BurstObservatory *BurstObservatoryConfig `json:"burstObservatory"`
	Version          *VersionConfig          `json:"version"`
}

func (c *Config) findInboundTag(tag string) int {
	found := -1
	for idx, ib := range c.InboundConfigs {
		if ib.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

func (c *Config) findOutboundTag(tag string) int {
	found := -1
	for idx, ob := range c.OutboundConfigs {
		if ob.Tag == tag {
			found = idx
			break
		}
	}
	return found
}

// Override method accepts another Config overrides the current attribute
func (c *Config) Override(o *Config, fn string) {
	// only process the non-deprecated members

	if o.LogConfig != nil {
		c.LogConfig = o.LogConfig
	}
	if o.RouterConfig != nil {
		c.RouterConfig = o.RouterConfig
	}
	if o.DNSConfig != nil {
		c.DNSConfig = o.DNSConfig
	}
	if o.Transport != nil {
		c.Transport = o.Transport
	}
	if o.Policy != nil {
		c.Policy = o.Policy
	}
	if o.API != nil {
		c.API = o.API
	}
	if o.Metrics != nil {
		c.Metrics = o.Metrics
	}
	if o.Stats != nil {
		c.Stats = o.Stats
	}
	if o.Reverse != nil {
		c.Reverse = o.Reverse
	}

	if o.FakeDNS != nil {
		c.FakeDNS = o.FakeDNS
	}

	if o.Observatory != nil {
		c.Observatory = o.Observatory
	}

	if o.BurstObservatory != nil {
		c.BurstObservatory = o.BurstObservatory
	}

	if o.Version != nil {
		c.Version = o.Version
	}

	// update the Inbound in slice if the only one in override config has same tag
	if len(o.InboundConfigs) > 0 {
		for i := range o.InboundConfigs {
			if idx := c.findInboundTag(o.InboundConfigs[i].Tag); idx > -1 {
				c.InboundConfigs[idx] = o.InboundConfigs[i]
				errors.LogInfo(context.Background(), "[", fn, "] updated inbound with tag: ", o.InboundConfigs[i].Tag)

			} else {
				c.InboundConfigs = append(c.InboundConfigs, o.InboundConfigs[i])
				errors.LogInfo(context.Background(), "[", fn, "] appended inbound with tag: ", o.InboundConfigs[i].Tag)
			}

		}
	}

	// update the Outbound in slice if the only one in override config has same tag
	if len(o.OutboundConfigs) > 0 {
		outboundPrepends := []OutboundDetourConfig{}
		for i := range o.OutboundConfigs {
			if idx := c.findOutboundTag(o.OutboundConfigs[i].Tag); idx > -1 {
				c.OutboundConfigs[idx] = o.OutboundConfigs[i]
				errors.LogInfo(context.Background(), "[", fn, "] updated outbound with tag: ", o.OutboundConfigs[i].Tag)
			} else {
				if strings.Contains(strings.ToLower(fn), "tail") {
					c.OutboundConfigs = append(c.OutboundConfigs, o.OutboundConfigs[i])
					errors.LogInfo(context.Background(), "[", fn, "] appended outbound with tag: ", o.OutboundConfigs[i].Tag)
				} else {
					outboundPrepends = append(outboundPrepends, o.OutboundConfigs[i])
					errors.LogInfo(context.Background(), "[", fn, "] prepend outbound with tag: ", o.OutboundConfigs[i].Tag)
				}
			}
		}
		if !strings.Contains(strings.ToLower(fn), "tail") && len(outboundPrepends) > 0 {
			c.OutboundConfigs = append(outboundPrepends, c.OutboundConfigs...)
		}
	}
}

// Build implements Buildable.
func (c *Config) Build() (*core.Config, error) {
	if err := PostProcessConfigureFile(c); err != nil {
		return nil, errors.New("failed to post-process configuration file").Base(err)
	}

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.InboundConfig{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
		},
	}

	if c.API != nil {
		apiConf, err := c.API.Build()
		if err != nil {
			return nil, errors.New("failed to build API configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(apiConf))
	}
	if c.Metrics != nil {
		metricsConf, err := c.Metrics.Build()
		if err != nil {
			return nil, errors.New("failed to build metrics configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(metricsConf))
	}
	if c.Stats != nil {
		statsConf, err := c.Stats.Build()
		if err != nil {
			return nil, errors.New("failed to build stats configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(statsConf))
	}

	var logConfMsg *serial.TypedMessage
	if c.LogConfig != nil {
		logConfMsg = serial.ToTypedMessage(c.LogConfig.Build())
	} else {
		logConfMsg = serial.ToTypedMessage(DefaultLogConfig())
	}
	// let logger module be the first App to start,
	// so that other modules could print log during initiating
	config.App = append([]*serial.TypedMessage{logConfMsg}, config.App...)

	if c.RouterConfig != nil {
		routerConfig, err := c.RouterConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build routing configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(routerConfig))
	}

	if c.DNSConfig != nil {
		dnsApp, err := c.DNSConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build DNS configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(dnsApp))
	}

	if c.Policy != nil {
		pc, err := c.Policy.Build()
		if err != nil {
			return nil, errors.New("failed to build policy configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(pc))
	}

	if c.Reverse != nil {
		r, err := c.Reverse.Build()
		if err != nil {
			return nil, errors.New("failed to build reverse configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(r))
	}

	if c.FakeDNS != nil {
		r, err := c.FakeDNS.Build()
		if err != nil {
			return nil, errors.New("failed to build fake DNS configuration").Base(err)
		}
		config.App = append([]*serial.TypedMessage{serial.ToTypedMessage(r)}, config.App...)
	}

	if c.Observatory != nil {
		r, err := c.Observatory.Build()
		if err != nil {
			return nil, errors.New("failed to build observatory configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(r))
	}

	if c.BurstObservatory != nil {
		r, err := c.BurstObservatory.Build()
		if err != nil {
			return nil, errors.New("failed to build burst observatory configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(r))
	}

	if c.Version != nil {
		r, err := c.Version.Build()
		if err != nil {
			return nil, errors.New("failed to build version configuration").Base(err)
		}
		config.App = append(config.App, serial.ToTypedMessage(r))
	}

	var inbounds []InboundDetourConfig

	if len(c.InboundConfigs) > 0 {
		inbounds = append(inbounds, c.InboundConfigs...)
	}

	if len(c.Transport) > 0 {
		return nil, errors.PrintRemovedFeatureError("Global transport config", "streamSettings in inbounds and outbounds")
	}

	for _, rawInboundConfig := range inbounds {
		ic, err := rawInboundConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build inbound config with tag ", rawInboundConfig.Tag).Base(err)
		}
		config.Inbound = append(config.Inbound, ic)
	}

	var outbounds []OutboundDetourConfig

	if len(c.OutboundConfigs) > 0 {
		outbounds = append(outbounds, c.OutboundConfigs...)
	}

	for _, rawOutboundConfig := range outbounds {
		oc, err := rawOutboundConfig.Build()
		if err != nil {
			return nil, errors.New("failed to build outbound config with tag ", rawOutboundConfig.Tag).Base(err)
		}
		config.Outbound = append(config.Outbound, oc)
	}

	return config, nil
}

func (c *Config) BuildMPHCache(customMatcherFilePath *string) error {
	var geosite []*router.GeoSite
	deps := make(map[string][]string)
	uniqueGeosites := make(map[string]bool)
	uniqueTags := make(map[string]bool)
	matcherFilePath := platform.GetAssetLocation("matcher.cache")

	if customMatcherFilePath != nil {
		matcherFilePath = *customMatcherFilePath
	}

	processGeosite := func(dStr string) bool {
		prefix := ""
		if strings.HasPrefix(dStr, "geosite:") {
			prefix = "geosite:"
		} else if strings.HasPrefix(dStr, "ext-domain:") {
			prefix = "ext-domain:"
		}
		if prefix == "" {
			return false
		}
		key := strings.ToLower(dStr)
		country := strings.ToUpper(dStr[len(prefix):])
		if !uniqueGeosites[country] {
			ds, err := loadGeositeWithAttr("geosite.dat", country)
			if err == nil {
				uniqueGeosites[country] = true
				geosite = append(geosite, &router.GeoSite{CountryCode: key, Domain: ds})
			}
		}
		return true
	}

	processDomains := func(tag string, rawDomains []string) {
		var manualDomains []*router.Domain
		var dDeps []string
		for _, dStr := range rawDomains {
			if processGeosite(dStr) {
				dDeps = append(dDeps, strings.ToLower(dStr))
			} else {
				ds, err := parseDomainRule(dStr)
				if err == nil {
					manualDomains = append(manualDomains, ds...)
				}
			}
		}
		if len(manualDomains) > 0 {
			if !uniqueTags[tag] {
				uniqueTags[tag] = true
				geosite = append(geosite, &router.GeoSite{CountryCode: tag, Domain: manualDomains})
			}
		}
		if len(dDeps) > 0 {
			deps[tag] = append(deps[tag], dDeps...)
		}
	}

	// proccess rules
	if c.RouterConfig != nil {
		for _, rawRule := range c.RouterConfig.RuleList {
			type SimpleRule struct {
				RuleTag string      `json:"ruleTag"`
				Domain  *StringList `json:"domain"`
				Domains *StringList `json:"domains"`
			}
			var sr SimpleRule
			json.Unmarshal(rawRule, &sr)
			if sr.RuleTag == "" {
				continue
			}
			var allDomains []string
			if sr.Domain != nil {
				allDomains = append(allDomains, *sr.Domain...)
			}
			if sr.Domains != nil {
				allDomains = append(allDomains, *sr.Domains...)
			}
			processDomains(sr.RuleTag, allDomains)
		}
	}

	// proccess dns servers
	if c.DNSConfig != nil {
		for _, ns := range c.DNSConfig.Servers {
			if ns.Tag == "" {
				continue
			}
			processDomains(ns.Tag, ns.Domains)
		}
	}

	var hostIPs map[string][]string
	if c.DNSConfig != nil && c.DNSConfig.Hosts != nil {
		hostIPs = make(map[string][]string)
		var hostDeps []string
		var hostPatterns []string

		// use raw map to avoid expanding geosites
		var domains []string
		for domain := range c.DNSConfig.Hosts.Hosts {
			domains = append(domains, domain)
		}
		sort.Strings(domains)

		manualHostGroups := make(map[string][]*router.Domain)
		manualHostIPs := make(map[string][]string)
		manualHostNames := make(map[string]string)

		for _, domain := range domains {
			ha := c.DNSConfig.Hosts.Hosts[domain]
			m := getHostMapping(ha)

			var ips []string
			if m.ProxiedDomain != "" {
				ips = append(ips, m.ProxiedDomain)
			} else {
				for _, ip := range m.Ip {
					ips = append(ips, net.IPAddress(ip).String())
				}
			}

			if processGeosite(domain) {
				tag := strings.ToLower(domain)
				hostDeps = append(hostDeps, tag)
				hostIPs[tag] = ips
				hostPatterns = append(hostPatterns, domain)
			} else {
				// build manual domains by their destination IPs
				sort.Strings(ips)
				ipKey := strings.Join(ips, ",")
				ds, err := parseDomainRule(domain)
				if err == nil {
					manualHostGroups[ipKey] = append(manualHostGroups[ipKey], ds...)
					manualHostIPs[ipKey] = ips
					if _, ok := manualHostNames[ipKey]; !ok {
						manualHostNames[ipKey] = domain
					}
				}
			}
		}

		// create manual host groups
		var ipKeys []string
		for k := range manualHostGroups {
			ipKeys = append(ipKeys, k)
		}
		sort.Strings(ipKeys)

		for _, k := range ipKeys {
			tag := manualHostNames[k]
			geosite = append(geosite, &router.GeoSite{CountryCode: tag, Domain: manualHostGroups[k]})
			hostDeps = append(hostDeps, tag)
			hostIPs[tag] = manualHostIPs[k]

			// record tag _ORDER links the matcher to IP addresses
			hostPatterns = append(hostPatterns, tag)
		}

		deps["HOSTS"] = hostDeps
		hostIPs["_ORDER"] = hostPatterns
	}

	f, err := os.Create(matcherFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer

	if err := router.SerializeGeoSiteList(geosite, deps, hostIPs, &buf); err != nil {
		return err
	}

	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// Convert string to Address.
func ParseSendThough(Addr *string) *Address {
	var addr Address
	addr.Address = net.ParseAddress(strings.Split(*Addr, "/")[0])
	return &addr
}
