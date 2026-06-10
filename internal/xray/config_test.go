package xray

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

func makeConfig() *Config {
	return &Config{
		LogConfig:       json_util.RawMessage(`{"loglevel":"warning"}`),
		RouterConfig:    json_util.RawMessage(`{}`),
		OutboundConfigs: json_util.RawMessage(`[]`),
		Policy:          json_util.RawMessage(`{}`),
		API:             json_util.RawMessage(`{}`),
		Stats:           json_util.RawMessage(`{}`),
		Metrics:         json_util.RawMessage(`{}`),
		InboundConfigs: []InboundConfig{
			{
				Port:     1080,
				Protocol: "vless",
				Tag:      "inbound-1080",
				Listen:   json_util.RawMessage(`"0.0.0.0"`),
				Settings: json_util.RawMessage(`{"clients":[]}`),
			},
		},
	}
}

func TestConfigEquals_IdenticalConfigs(t *testing.T) {
	a := makeConfig()
	b := makeConfig()
	if !a.Equals(b) {
		t.Fatal("two identical configs should be Equals")
	}
}

func TestConfigEquals_DifferentInboundCount(t *testing.T) {
	a := makeConfig()
	b := makeConfig()
	b.InboundConfigs = append(b.InboundConfigs, InboundConfig{Port: 2080, Protocol: "vmess", Tag: "inbound-2080"})
	if a.Equals(b) {
		t.Fatal("configs with different inbound counts should not be Equals")
	}
}

func TestConfigEquals_DifferentInboundContent(t *testing.T) {
	a := makeConfig()
	b := makeConfig()
	b.InboundConfigs[0].Port = 9999
	if a.Equals(b) {
		t.Fatal("config with changed inbound port should not be Equals")
	}
}

func TestConfigEquals_DifferentLogConfig(t *testing.T) {
	a := makeConfig()
	b := makeConfig()
	b.LogConfig = json_util.RawMessage(`{"loglevel":"debug"}`)
	if a.Equals(b) {
		t.Fatal("config with changed log section should not be Equals")
	}
}

func TestConfigEquals_RawSectionsCompared(t *testing.T) {
	fields := []struct {
		name    string
		mutator func(c *Config)
	}{
		{"RouterConfig", func(c *Config) { c.RouterConfig = json_util.RawMessage(`{"changed":true}`) }},
		{"DNSConfig", func(c *Config) { c.DNSConfig = json_util.RawMessage(`{"servers":["1.1.1.1"]}`) }},
		{"OutboundConfigs", func(c *Config) { c.OutboundConfigs = json_util.RawMessage(`[{"tag":"x"}]`) }},
		{"Transport", func(c *Config) { c.Transport = json_util.RawMessage(`{"x":1}`) }},
		{"Policy", func(c *Config) { c.Policy = json_util.RawMessage(`{"levels":{}}`) }},
		{"API", func(c *Config) { c.API = json_util.RawMessage(`{"tag":"api"}`) }},
		{"Stats", func(c *Config) { c.Stats = json_util.RawMessage(`{"on":true}`) }},
		{"Reverse", func(c *Config) { c.Reverse = json_util.RawMessage(`{"bridges":[]}`) }},
		{"FakeDNS", func(c *Config) { c.FakeDNS = json_util.RawMessage(`[]`) }},
		{"Metrics", func(c *Config) { c.Metrics = json_util.RawMessage(`{"tag":"m"}`) }},
	}
	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			a := makeConfig()
			b := makeConfig()
			f.mutator(b)
			if a.Equals(b) {
				t.Fatalf("mutating %s should break Equals", f.name)
			}
		})
	}
}
