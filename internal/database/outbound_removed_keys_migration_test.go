package database

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestRewriteRemovedOutboundKeys(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantChanged  bool
		wantOutbound map[string]any
	}{
		{
			name:        "proxySettings.tag becomes sockopt.dialerProxy",
			raw:         `{"outbounds":[{"protocol":"vless","tag":"chain","settings":{},"proxySettings":{"tag":"hop","transportLayer":true}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "vless", "tag": "chain", "settings": map[string]any{},
				"streamSettings": map[string]any{"sockopt": map[string]any{"dialerProxy": "hop"}},
			},
		},
		{
			name:        "an existing dialerProxy wins over proxySettings",
			raw:         `{"outbounds":[{"protocol":"vless","tag":"chain","proxySettings":{"tag":"hop"},"streamSettings":{"network":"tcp","sockopt":{"dialerProxy":"keep"}}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "vless", "tag": "chain",
				"streamSettings": map[string]any{"network": "tcp", "sockopt": map[string]any{"dialerProxy": "keep"}},
			},
		},
		{
			name:        "freedom drops sockopt.addressPortStrategy",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","streamSettings":{"sockopt":{"addressPortStrategy":"SrvPortOnly","tcpFastOpen":true}}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct",
				"streamSettings": map[string]any{"sockopt": map[string]any{"tcpFastOpen": true}},
			},
		},
		{
			name:        "addressPortStrategy stays on other protocols",
			raw:         `{"outbounds":[{"protocol":"vless","tag":"proxy","streamSettings":{"sockopt":{"addressPortStrategy":"SrvPortOnly"}}}]}`,
			wantChanged: false,
			wantOutbound: map[string]any{
				"protocol": "vless", "tag": "proxy",
				"streamSettings": map[string]any{"sockopt": map[string]any{"addressPortStrategy": "SrvPortOnly"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed, err := rewriteRemovedOutboundKeys(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			var cfg struct {
				Outbounds []map[string]any `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(updated), &cfg); err != nil {
				t.Fatalf("rewritten template is not JSON: %v", err)
			}
			if len(cfg.Outbounds) != 1 {
				t.Fatalf("got %d outbounds, want 1", len(cfg.Outbounds))
			}
			got, _ := json.Marshal(cfg.Outbounds[0])
			want, _ := json.Marshal(tc.wantOutbound)
			if string(got) != string(want) {
				t.Fatalf("outbound = %s, want %s", got, want)
			}
		})
	}
}

func TestRewriteRemovedOutboundKeysSatisfiesCore(t *testing.T) {
	raw := `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{},"proxySettings":{"tag":"hop"},"streamSettings":{"sockopt":{"addressPortStrategy":"SrvPortOnly"}}}]}`
	var before struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(raw), &before); err != nil {
		t.Fatal(err)
	}
	if err := xray.ValidateOutboundConfig(before.Outbounds[0]); err == nil {
		t.Fatal("expected the vendored core to refuse the legacy outbound")
	}

	updated, changed, err := rewriteRemovedOutboundKeys(raw)
	if err != nil || !changed {
		t.Fatalf("rewrite: changed=%v err=%v", changed, err)
	}
	var after struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(updated), &after); err != nil {
		t.Fatal(err)
	}
	if err := xray.ValidateOutboundConfig(after.Outbounds[0]); err != nil {
		t.Fatalf("rewritten outbound still refused by xray-core: %v", err)
	}
}

func TestRewriteRemovedOutboundKeysInvalidJSON(t *testing.T) {
	_, changed, err := rewriteRemovedOutboundKeys("{not json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	if changed {
		t.Fatal("invalid JSON must not report a change")
	}
}
