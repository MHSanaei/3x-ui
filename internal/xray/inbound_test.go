package xray

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

func makeInbound() InboundConfig {
	return InboundConfig{
		Listen:         json_util.RawMessage(`"0.0.0.0"`),
		Port:           1234,
		Protocol:       "vless",
		Settings:       json_util.RawMessage(`{"clients":[{"id":"abc"}]}`),
		StreamSettings: json_util.RawMessage(`{"network":"tcp"}`),
		Tag:            "inbound-1234",
		Sniffing:       json_util.RawMessage(`{"enabled":false}`),
	}
}

func TestInboundConfigEquals_Identical(t *testing.T) {
	a := makeInbound()
	b := makeInbound()
	if !a.Equals(&b) {
		t.Fatal("two identical inbounds should be Equals")
	}
}

func TestInboundConfigEquals_MutationsBreakEquality(t *testing.T) {
	cases := []struct {
		name    string
		mutator func(c *InboundConfig)
	}{
		{"Listen", func(c *InboundConfig) { c.Listen = json_util.RawMessage(`"127.0.0.1"`) }},
		{"Port", func(c *InboundConfig) { c.Port = 9999 }},
		{"Protocol", func(c *InboundConfig) { c.Protocol = "vmess" }},
		{"Settings", func(c *InboundConfig) { c.Settings = json_util.RawMessage(`{"clients":[]}`) }},
		{"StreamSettings", func(c *InboundConfig) { c.StreamSettings = json_util.RawMessage(`{"network":"ws"}`) }},
		{"Tag", func(c *InboundConfig) { c.Tag = "inbound-other" }},
		{"Sniffing", func(c *InboundConfig) { c.Sniffing = json_util.RawMessage(`{"enabled":true}`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := makeInbound()
			b := makeInbound()
			tc.mutator(&b)
			if a.Equals(&b) {
				t.Fatalf("mutating %s should break Equals", tc.name)
			}
		})
	}
}
