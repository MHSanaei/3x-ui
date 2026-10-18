package sub

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	yaml "github.com/goccy/go-yaml"
)

const remoteClashTemplateWithDNS = `
dns:
  enable: true
  nameserver:
    - https://8.8.8.8/dns-query
sniffer:
  enable: true
rules:
  - DOMAIN,remote.example,DIRECT
  - MATCH,PROXY
`

var remoteClashTemplateDNS = map[string]any{
	"enable":     true,
	"nameserver": []any{"https://8.8.8.8/dns-query"},
}

func remoteClashTestBase() map[string]any {
	return map[string]any{
		"proxies": []map[string]any{{"name": "vpn-node", "type": "vless"}},
		"proxy-groups": []map[string]any{{
			"name": "PROXY", "type": "select", "proxies": []string{"vpn-node", "DIRECT"},
		}},
		"rules": []string{"MATCH,PROXY"},
	}
}

func TestMergeRemoteClashRulesClientConfigIsOptIn(t *testing.T) {
	tests := []struct {
		name              string
		allowClientConfig bool
		wantDNS           any
		wantSniffer       any
	}{
		{name: "off drops dns and sniffer", allowClientConfig: false},
		{
			name:              "on copies dns and sniffer",
			allowClientConfig: true,
			wantDNS:           remoteClashTemplateDNS,
			wantSniffer:       map[string]any{"enable": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := remoteClashTestBase()
			if err := mergeRemoteClashRulesYAML(base, remoteClashTemplateWithDNS, tt.allowClientConfig); err != nil {
				t.Fatalf("merge: %v", err)
			}
			if !reflect.DeepEqual(base["dns"], tt.wantDNS) {
				t.Fatalf("dns = %#v, want %#v", base["dns"], tt.wantDNS)
			}
			if rules, _ := asAnySlice(base["rules"]); len(rules) == 0 || rules[0] != "DOMAIN,remote.example,DIRECT" {
				t.Fatalf("rules = %#v, want remote rules applied", base["rules"])
			}
			if !reflect.DeepEqual(base["sniffer"], tt.wantSniffer) {
				t.Fatalf("sniffer = %#v, want %#v", base["sniffer"], tt.wantSniffer)
			}
		})
	}
}

func TestRemoteClashMalformedClientConfigIsSkipped(t *testing.T) {
	tests := []struct {
		name   string
		remote string
	}{
		{name: "scalar dns", remote: "dns: 8.8.8.8\nrules:\n  - DOMAIN,remote.example,DIRECT\n  - MATCH,PROXY\n"},
		{name: "list dns", remote: "dns:\n  - 8.8.8.8\nrules:\n  - DOMAIN,remote.example,DIRECT\n  - MATCH,PROXY\n"},
		{name: "null dns", remote: "dns:\nrules:\n  - DOMAIN,remote.example,DIRECT\n  - MATCH,PROXY\n"},
		{name: "scalar sniffer", remote: "sniffer: true\nrules:\n  - DOMAIN,remote.example,DIRECT\n  - MATCH,PROXY\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := normalizeClashRouting([]byte(tt.remote)); err != nil {
				t.Fatalf("fetch-time err=%v, want nil", err)
			}
			base := remoteClashTestBase()
			if err := mergeRemoteClashRulesYAML(base, tt.remote, true); err != nil {
				t.Fatalf("merge err=%v, want nil", err)
			}
			if _, ok := base["dns"]; ok {
				t.Fatalf("dns = %#v, want absent", base["dns"])
			}
			if _, ok := base["sniffer"]; ok {
				t.Fatalf("sniffer = %#v, want absent", base["sniffer"])
			}
			if rules, _ := asAnySlice(base["rules"]); len(rules) == 0 || rules[0] != "DOMAIN,remote.example,DIRECT" {
				t.Fatalf("rules = %#v, want remote rules applied", base["rules"])
			}
		})
	}
}

func TestNormalizeClashRoutingRequiresRouteGraphKey(t *testing.T) {
	_, _, err := normalizeClashRouting([]byte("dns:\n  enable: true\nsniffer:\n  enable: true\n"))
	if err == nil || err.Error() != "Clash routing response has no supported routing keys" {
		t.Fatalf("err=%v, want no supported routing keys", err)
	}
}

func TestClashSubscriptionRemoteDNSFollowsSetting(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "rdns", 4601, 1, wsTLSStream)
	gin.SetMode(gin.TestMode)

	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })
	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(http.StatusOK, remoteClashTemplateWithDNS), nil
	}), false)
	const source = "https://example.com/mihomo/default.yaml"
	primeRemoteRouting(t, routingSourceResolver, remoteRoutingClash, source)

	tests := []struct {
		name      string
		remoteDNS bool
		wantDNS   any
	}{
		{name: "off keeps client dns", remoteDNS: false},
		{name: "on takes remote dns", remoteDNS: true, wantDNS: remoteClashTemplateDNS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			NewSUBController(
				router.Group("/"),
				WithSUBClashEnabled(true),
				WithSUBClashEnableRouting(true),
				WithSUBClashRules(source),
				WithSUBClashRemoteDNS(tt.remoteDNS),
			)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "http://sub.example.com/clash/s1", nil))
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			var config map[string]any
			if err := yaml.Unmarshal(resp.Body.Bytes(), &config); err != nil {
				t.Fatalf("decode clash yaml: %v\n%s", err, resp.Body.String())
			}
			if !reflect.DeepEqual(config["dns"], tt.wantDNS) {
				t.Fatalf("dns = %#v, want %#v\n%s", config["dns"], tt.wantDNS, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), "DOMAIN,remote.example,DIRECT") {
				t.Fatalf("remote rules were not applied:\n%s", resp.Body.String())
			}
		})
	}
}
