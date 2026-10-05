package sub

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// #6738: without the host's Description on hysteria(2):// links, Happ falls back
// to its own "Hysteria | hysteria | TLS" caption on a host that also serves VLESS.
func TestGenHysteriaLinkAppendsHostServerDescription(t *testing.T) {
	tests := map[string]struct {
		version int
		scheme  string
	}{
		"hysteria v1": {version: 1, scheme: "hysteria://"},
		"hysteria v2": {version: 2, scheme: "hysteria2://"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			host := &model.Host{
				Address: "hy.example.com", Port: 443,
				Remark: "Poland", ServerDescription: "Wi-Fi",
			}
			stream := map[string]any{
				"security":      "tls",
				"externalProxy": []any{hostToExternalProxyMap(host, "hy.example.com", 443)},
			}
			rawStream, err := json.Marshal(stream)
			if err != nil {
				t.Fatalf("marshal stream settings: %v", err)
			}
			// The inbound's own `version` picks the hysteria vs hysteria2 scheme.
			rawSettings, err := json.Marshal(map[string]any{
				"version": tc.version,
				"clients": []any{map[string]any{"auth": "secret", "email": "user"}},
			})
			if err != nil {
				t.Fatalf("marshal inbound settings: %v", err)
			}
			in := &model.Inbound{
				Id: 920010, Listen: "203.0.113.1", Port: 443, Protocol: model.Hysteria,
				Remark: "hy", StreamSettings: string(rawStream), Settings: string(rawSettings),
			}
			got := (&SubService{}).genHysteriaLink(in, "user")
			if !strings.HasPrefix(got, tc.scheme) {
				t.Fatalf("link scheme changed.\n got: %s\nwant prefix: %s", got, tc.scheme)
			}
			// base64("Wi-Fi"), matching the reporter's subscription output.
			if !strings.HasSuffix(got, "?serverDescription=V2ktRmk=") {
				t.Fatalf("host serverDescription missing from fragment.\n got: %s\nwant suffix: ?serverDescription=V2ktRmk=", got)
			}
		})
	}
}
