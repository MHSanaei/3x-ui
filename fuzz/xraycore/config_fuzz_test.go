package xraycorefuzz

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	core "github.com/xtls/xray-core/core"
	xconf "github.com/xtls/xray-core/infra/conf"
	xserial "github.com/xtls/xray-core/infra/conf/serial"

	_ "github.com/xtls/xray-core/main/distro/all"
)

const (
	maxConfigInputBytes = 64 << 10
	maxConfigIteration  = 2 * time.Second
)

func FuzzXrayCoreFullConfigBuild(f *testing.F) {
	addStringSeeds(f,
		minimalFullConfig,
		minimalVLESSInboundConfig,
		fullConfigWithAPIStatsDNSRouting,
		fullConfigWithVLESSWS,
		fullConfigWithVLESSGRPC,
		`{`,
		`null`,
		`{"inbounds":[{"protocol":"vless","port":"not-a-port","settings":{"clients":[]}}]}`,
		`{"routing":{"rules":[{"type":"field","domain":["regexp:("],"outboundTag":"direct"}]},"outbounds":[{"protocol":"freedom","tag":"direct"}]}`,
	)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxConfigInputBytes) {
			return
		}
		start := time.Now()
		cfg, err := xserial.DecodeJSONConfig(bytes.NewReader(data))
		if err != nil {
			return
		}
		buildAndInit(t, cfg)
		failIfSlow(t, start, maxConfigIteration)
	})
}

func FuzzXrayCoreInboundVLESSConfigBuild(f *testing.F) {
	addStringSeeds(f,
		minimalVLESSInboundObject,
		minimalVLESSInboundSettings,
		vlessInboundWithFallback,
		vlessInboundWithVisionFlow,
		`{"clients":[{"id":"not-a-uuid"}],"decryption":"none"}`,
		`{"clients":[{"id":"11111111-1111-1111-1111-111111111111","encryption":"none"}],"decryption":"none"}`,
		`{"protocol":"vless","port":443,"settings":{"clients":[]}}`,
	)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxConfigInputBytes/2) {
			return
		}
		start := time.Now()

		var inbound xconf.InboundDetourConfig
		if err := json.Unmarshal(data, &inbound); err == nil {
			buildAndInit(t, &xconf.Config{InboundConfigs: []xconf.InboundDetourConfig{inbound}})
		}

		if json.Valid(data) {
			wrapped := wrapVLESSInboundSettings(data)
			var vlessInbound xconf.InboundDetourConfig
			if err := json.Unmarshal(wrapped, &vlessInbound); err == nil {
				buildAndInit(t, &xconf.Config{InboundConfigs: []xconf.InboundDetourConfig{vlessInbound}})
			}
		}

		failIfSlow(t, start, maxConfigIteration)
	})
}

func FuzzXrayCoreOutboundConfigBuild(f *testing.F) {
	addStringSeeds(f,
		minimalFreedomOutboundObject,
		minimalVLESSOutboundObject,
		minimalVLESSOutboundSettings,
		`{"protocol":"vless","settings":{"vnext":[]}}`,
		`{"protocol":"vless","settings":{"vnext":[{"address":"example.com","port":443,"users":[]}]}}`,
		`{"protocol":"freedom","settings":{"domainStrategy":"forceIPv4"}}`,
	)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxConfigInputBytes/2) {
			return
		}
		start := time.Now()

		var outbound xconf.OutboundDetourConfig
		if err := json.Unmarshal(data, &outbound); err == nil {
			buildAndInit(t, &xconf.Config{OutboundConfigs: []xconf.OutboundDetourConfig{outbound}})
		}

		if json.Valid(data) {
			wrapped := wrapVLESSOutboundSettings(data)
			var vlessOutbound xconf.OutboundDetourConfig
			if err := json.Unmarshal(wrapped, &vlessOutbound); err == nil {
				buildAndInit(t, &xconf.Config{OutboundConfigs: []xconf.OutboundDetourConfig{vlessOutbound}})
			}
		}

		failIfSlow(t, start, maxConfigIteration)
	})
}

func FuzzXrayCoreStreamSettingsBuild(f *testing.F) {
	addStringSeeds(f,
		`{}`,
		`{"network":"tcp","security":"none"}`,
		`{"network":"ws","wsSettings":{"path":"/vless","headers":{"Host":"example.com"}}}`,
		`{"network":"grpc","grpcSettings":{"serviceName":"svc","multiMode":true}}`,
		`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.com","alpn":["h2","http/1.1"]}}`,
		`{"network":"tcp","security":"reality","realitySettings":{"show":false,"dest":"example.com:443","serverNames":["example.com"],"privateKey":"short","shortIds":["00"]}}`,
		`{"network":"kcp","kcpSettings":{"mtu":1350,"tti":50,"uplinkCapacity":5,"downlinkCapacity":20,"header":{"type":"wechat-video"}}}`,
		`{"network":"xhttp","xhttpSettings":{"path":"/x","mode":"auto"}}`,
	)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxConfigInputBytes/2) {
			return
		}
		start := time.Now()
		var stream xconf.StreamConfig
		if err := json.Unmarshal(data, &stream); err != nil {
			return
		}
		_, _ = stream.Build()
		failIfSlow(t, start, maxConfigIteration)
	})
}

func FuzzXrayCoreSniffingRoutingDNSConfigBuild(f *testing.F) {
	addStringSeeds(f,
		`{"enabled":true,"destOverride":["http","tls","quic","fakedns"],"metadataOnly":false,"routeOnly":false}`,
		`{"domainStrategy":"IPIfNonMatch","rules":[{"type":"field","domain":["domain:example.com"],"outboundTag":"direct"}]}`,
		`{"servers":["1.1.1.1","https://dns.google/dns-query"],"hosts":{"example.com":"127.0.0.1"},"queryStrategy":"UseIPv4"}`,
		`{"sniffing":{"enabled":true,"destOverride":["bad-proto"]},"routing":{"rules":[{"type":"field","ip":["geoip:private"],"outboundTag":"direct"}]},"dns":{"servers":["localhost"]}}`,
	)

	f.Fuzz(func(t *testing.T, data []byte) {
		if tooLarge(data, maxConfigInputBytes/2) {
			return
		}
		start := time.Now()

		var sniffing xconf.SniffingConfig
		if err := json.Unmarshal(data, &sniffing); err == nil {
			_, _ = sniffing.Build()
		}

		var routing xconf.RouterConfig
		if err := json.Unmarshal(data, &routing); err == nil {
			_, _ = routing.Build()
		}

		var dns xconf.DNSConfig
		if err := json.Unmarshal(data, &dns); err == nil {
			_, _ = dns.Build()
		}

		var cfg xconf.Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			buildAndInit(t, &cfg)
		}

		failIfSlow(t, start, maxConfigIteration)
	})
}

func buildAndInit(t *testing.T, cfg *xconf.Config) {
	t.Helper()
	if hasKnownEmptyDomainListen(cfg) {
		return
	}
	pbConfig, err := cfg.Build()
	if err != nil {
		return
	}
	if pbConfig == nil {
		t.Fatal("Build returned nil config without error")
	}
	instance, err := core.New(pbConfig)
	if err != nil {
		return
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("closing initialized Xray instance failed: %v", err)
	}
}

func TestXrayCoreKnownEmptyListenPanicReproducer(t *testing.T) {
	cfg, err := xserial.DecodeJSONConfig(bytes.NewReader([]byte(`{"inBounds":[{"listen":""}]}`)))
	if err != nil {
		t.Fatalf("failed to decode minimized empty listen reproducer: %v", err)
	}
	if !hasKnownEmptyDomainListen(cfg) {
		t.Fatal("minimized empty listen reproducer was not classified as known Xray-core panic")
	}
	panicValue := catchPanic(func() {
		_, _ = cfg.Build()
	})
	if panicValue == nil {
		t.Fatal("known Xray-core empty listen panic no longer reproduces; remove the quarantine")
	}
}

func hasKnownEmptyDomainListen(cfg *xconf.Config) bool {
	for _, inbound := range cfg.InboundConfigs {
		if inbound.ListenOn == nil || inbound.ListenOn.Address == nil {
			continue
		}
		if inbound.ListenOn.Family().IsDomain() && inbound.ListenOn.Domain() == "" {
			return true
		}
	}
	return false
}

func catchPanic(fn func()) (panicValue any) {
	defer func() {
		panicValue = recover()
	}()
	fn()
	return nil
}

func addStringSeeds(f *testing.F, seeds ...string) {
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}
}

func tooLarge(data []byte, max int) bool {
	return len(data) > max
}

func failIfSlow(t *testing.T, start time.Time, max time.Duration) {
	t.Helper()
	if elapsed := time.Since(start); elapsed > max {
		t.Fatalf("fuzz iteration took %s, max %s", elapsed, max)
	}
}

func wrapVLESSInboundSettings(settings []byte) []byte {
	out := make([]byte, 0, len(settings)+96)
	out = append(out, `{"protocol":"vless","listen":"127.0.0.1","port":443,"settings":`...)
	out = append(out, settings...)
	out = append(out, `}`...)
	return out
}

func wrapVLESSOutboundSettings(settings []byte) []byte {
	out := make([]byte, 0, len(settings)+64)
	out = append(out, `{"protocol":"vless","settings":`...)
	out = append(out, settings...)
	out = append(out, `}`...)
	return out
}

const minimalFullConfig = `{
  "log": {"loglevel": "warning"},
  "inbounds": [],
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`

const minimalVLESSInboundConfig = `{
  "inbounds": [{
    "tag": "vless-in",
    "listen": "127.0.0.1",
    "port": 443,
    "protocol": "vless",
    "settings": {
      "clients": [{"id": "11111111-1111-1111-1111-111111111111", "email": "seed@example"}],
      "decryption": "none"
    },
    "streamSettings": {"network": "tcp", "security": "none"},
    "sniffing": {"enabled": true, "destOverride": ["http", "tls"]}
  }],
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`

const fullConfigWithAPIStatsDNSRouting = `{
  "api": {"tag": "api", "services": ["HandlerService", "StatsService"]},
  "stats": {},
  "dns": {"servers": ["1.1.1.1"], "hosts": {"seed.example": "127.0.0.1"}},
  "routing": {"domainStrategy": "IPIfNonMatch", "rules": [{"type": "field", "domain": ["domain:seed.example"], "outboundTag": "direct"}]},
  "inbounds": [],
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`

const fullConfigWithVLESSWS = `{
  "inbounds": [{
    "listen": "127.0.0.1",
    "port": 8443,
    "protocol": "vless",
    "settings": {"clients": [{"id": "11111111-1111-1111-1111-111111111111"}], "decryption": "none"},
    "streamSettings": {"network": "ws", "security": "tls", "wsSettings": {"path": "/ws"}, "tlsSettings": {"serverName": "example.com"}}
  }],
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`

const fullConfigWithVLESSGRPC = `{
  "inbounds": [{
    "listen": "127.0.0.1",
    "port": 9443,
    "protocol": "vless",
    "settings": {"clients": [{"id": "11111111-1111-1111-1111-111111111111", "flow": "xtls-rprx-vision"}], "decryption": "none"},
    "streamSettings": {"network": "grpc", "grpcSettings": {"serviceName": "svc", "multiMode": true}}
  }],
  "outbounds": [{"protocol": "freedom", "tag": "direct"}]
}`

const minimalVLESSInboundObject = `{
  "tag": "vless-in",
  "listen": "127.0.0.1",
  "port": 443,
  "protocol": "vless",
  "settings": {"clients": [{"id": "11111111-1111-1111-1111-111111111111"}], "decryption": "none"}
}`

const minimalVLESSInboundSettings = `{
  "clients": [{"id": "11111111-1111-1111-1111-111111111111", "email": "seed@example"}],
  "decryption": "none"
}`

const vlessInboundWithFallback = `{
  "clients": [{"id": "11111111-1111-1111-1111-111111111111"}],
  "decryption": "none",
  "fallbacks": [{"path": "/fallback", "dest": 8080, "xver": 1}]
}`

const vlessInboundWithVisionFlow = `{
  "clients": [{"id": "11111111-1111-1111-1111-111111111111", "flow": "xtls-rprx-vision"}],
  "decryption": "none",
  "flow": "xtls-rprx-vision"
}`

const minimalFreedomOutboundObject = `{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"AsIs"}}`

const minimalVLESSOutboundObject = `{
  "protocol": "vless",
  "tag": "vless-out",
  "settings": {
    "vnext": [{
      "address": "example.com",
      "port": 443,
      "users": [{"id": "11111111-1111-1111-1111-111111111111", "encryption": "none"}]
    }]
  },
  "streamSettings": {"network": "tcp", "security": "none"}
}`

const minimalVLESSOutboundSettings = `{
  "vnext": [{
    "address": "example.com",
    "port": 443,
    "users": [{"id": "11111111-1111-1111-1111-111111111111", "encryption": "none"}]
  }]
}`
