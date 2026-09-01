package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func b64Std(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
func b64URL(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func fullRoutingPayload() map[string]any {
	return map[string]any{
		"Name":              "RoscomVPN",
		"DomainStrategy":    "IPIfNonMatch",
		"RemoteDNSDomain":   "https://8.8.8.8/dns-query",
		"RemoteDNSIP":       "8.8.8.8",
		"DomesticDNSDomain": "https://77.88.8.8/dns-query",
		"DomesticDNSIP":     "77.88.8.8",
		"DnsHosts":          map[string]any{"lknpd.nalog.ru": "213.24.64.181"},
		"RouteOrder":        "block-proxy-direct",
		"DirectSites":       []any{"geosite:category-ru", "geosite:private"},
		"DirectIp":          []any{"geoip:private"},
		"ProxySites":        []any{"geosite:youtube"},
		"ProxyIp":           []any{},
		"BlockSites":        []any{"geosite:category-ads"},
		"BlockIp":           []any{},
	}
}

func TestParseJsonRoutingSpecMapsAllFields(t *testing.T) {
	spec, remote, err := parseJsonRoutingSpec(mustMarshal(t, fullRoutingPayload()))
	if err != nil || remote {
		t.Fatalf("parse: err=%v remote=%v", err, remote)
	}
	want := jsonRoutingSpec{
		DomainStrategy:    "IPIfNonMatch",
		RemoteDNSDomain:   "https://8.8.8.8/dns-query",
		RemoteDNSIP:       "8.8.8.8",
		DomesticDNSDomain: "https://77.88.8.8/dns-query",
		DomesticDNSIP:     "77.88.8.8",
		DnsHosts:          map[string]string{"lknpd.nalog.ru": "213.24.64.181"},
		RouteOrder:        []string{"block", "proxy", "direct"},
		DirectSites:       []string{"geosite:category-ru", "geosite:private"},
		DirectIp:          []string{"geoip:private"},
		ProxySites:        []string{"geosite:youtube"},
		BlockSites:        []string{"geosite:category-ads"},
	}
	if spec.DomainStrategy != want.DomainStrategy || spec.RemoteDNSIP != want.RemoteDNSIP ||
		spec.DomesticDNSDomain != want.DomesticDNSDomain || len(spec.DnsHosts) != 1 || spec.DnsHosts["lknpd.nalog.ru"] != "213.24.64.181" ||
		strings.Join(spec.RouteOrder, ",") != strings.Join(want.RouteOrder, ",") ||
		strings.Join(spec.DirectSites, ",") != strings.Join(want.DirectSites, ",") ||
		strings.Join(spec.DirectIp, ",") != strings.Join(want.DirectIp, ",") ||
		strings.Join(spec.ProxySites, ",") != strings.Join(want.ProxySites, ",") ||
		strings.Join(spec.BlockSites, ",") != strings.Join(want.BlockSites, ",") {
		t.Fatalf("spec = %+v\nwant %+v", spec, want)
	}
}

func TestParseJsonRoutingSpecPartialPayload(t *testing.T) {
	spec, _, err := parseJsonRoutingSpec(`{"DirectSites":["geosite:private"],"DomainStrategy":"AsIs"}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.DomainStrategy != "AsIs" || len(spec.DirectSites) != 1 || spec.DirectSites[0] != "geosite:private" {
		t.Fatalf("spec = %+v", spec)
	}
	if len(spec.RouteOrder) != 0 || len(spec.DnsHosts) != 0 || spec.RemoteDNSIP != "" {
		t.Fatalf("unset fields must stay zero: %+v", spec)
	}
	if spec.empty() {
		t.Fatalf("empty() must report false when any field is set: %+v", spec)
	}
}

func TestParseJsonRoutingSpecRouteOrderUnknownSegments(t *testing.T) {
	spec, _, err := parseJsonRoutingSpec(`{"RouteOrder":"block-foo-direct"}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Join(spec.RouteOrder, ",") != "block,direct" {
		t.Fatalf("RouteOrder = %v", spec.RouteOrder)
	}
}

func TestParseJsonRoutingSpecDeeplinks(t *testing.T) {
	payload := mustMarshal(t, fullRoutingPayload())
	cases := []string{
		"happ://routing/onadd/" + b64Std(payload),
		"incy://routing/onadd/" + b64Std(payload),
		"happ://routing/onadd/" + b64URL(payload),
	}
	for _, raw := range cases {
		spec, _, err := parseJsonRoutingSpec(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw[:32], err)
		}
		if spec.DomainStrategy != "IPIfNonMatch" || len(spec.DirectSites) != 2 || spec.RouteOrder[1] != "proxy" {
			t.Fatalf("spec from %q = %+v", raw[:32], spec)
		}
	}
}

func TestParseJsonRoutingSpecRejectsBadPayloads(t *testing.T) {
	cases := []string{
		"not json at all",
		"[1,2,3]",
		`{"DirectSites":"geosite:private"}`,
		`{"DirectSites":["a",1]}`,
		`{"DnsHosts":{"a":1}}`,
		`{"DomainStrategy":5}`,
		"happ://routing/onadd/!!!!not-base64!!!!",
	}
	for _, raw := range cases {
		if _, _, err := parseJsonRoutingSpec(raw); err == nil {
			t.Fatalf("payload %q was accepted", raw)
		}
	}
}

func TestParseJsonRoutingSpecEmpty(t *testing.T) {
	for _, raw := range []string{"", "   ", "\n"} {
		spec, remote, err := parseJsonRoutingSpec(raw)
		if err != nil || remote || !spec.empty() {
			t.Fatalf("raw=%q spec=%+v remote=%v err=%v", raw, spec, remote, err)
		}
	}
}

func TestParseJsonRoutingSpecRemoteURL(t *testing.T) {
	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })
	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(200, mustMarshal(t, fullRoutingPayload())), nil
	}), false)

	const source = "https://example.com/DEFAULT.JSON"
	primeRemoteRouting(t, routingSourceResolver, remoteRoutingHapp, source)
	spec, _, err := parseJsonRoutingSpec(source)
	if err != nil {
		t.Fatalf("parse: err=%v", err)
	}
	if spec.DomainStrategy != "IPIfNonMatch" || len(spec.BlockSites) != 1 {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestParseJsonRoutingSpecRemoteUnavailable(t *testing.T) {
	oldResolver := routingSourceResolver
	t.Cleanup(func() { routingSourceResolver = oldResolver })
	routingSourceResolver = newRemoteRoutingResolver(remoteRoutingTestClient(func(*http.Request) (*http.Response, error) {
		return remoteRoutingResponse(200, "routing.help"), nil
	}), false)
	if _, _, err := parseJsonRoutingSpec("https://example.com/bad"); err == nil {
		t.Fatal("unavailable remote source must error")
	}
}
