package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type stubEgress struct{ url string }

func (s stubEgress) NodeEgressProxyURL(int) string { return s.url }

// cacheGetTag must resolve a remote inbound id even when the n<id>- prefix
// sits on only one side: the node may store the bare tag while the central
// panel pushes the prefixed form, or vice versa. Without this a mismatch makes
// the push create a duplicate inbound on the node.
func TestCacheGetTag_PrefixAgnostic(t *testing.T) {
	cases := []struct {
		name      string
		cacheTag  string
		lookup    string
		wantID    int
		wantFound bool
	}{
		{"exact", "n1-in-443-tcp", "n1-in-443-tcp", 7, true},
		{"node bare, lookup prefixed", "in-443-tcp", "n1-in-443-tcp", 7, true},
		{"node prefixed, lookup bare", "n1-in-443-tcp", "in-443-tcp", 7, true},
		{"unrelated tag", "in-443-tcp", "in-999-tcp", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRemote(&model.Node{Id: 1, Name: "n1"}, nil)
			r.cacheSet(c.cacheTag, 7)
			id, ok := r.cacheGetTag(c.lookup)
			if ok != c.wantFound || id != c.wantID {
				t.Fatalf("cacheGetTag(%q) = (%d, %v), want (%d, %v)", c.lookup, id, ok, c.wantID, c.wantFound)
			}
		})
	}
}

func TestWireInboundIncludesShareAddressFields(t *testing.T) {
	values := wireInbound(&model.Inbound{
		ShareAddrStrategy: "custom",
		ShareAddr:         "edge.example.com",
	})

	if got := values.Get("shareAddrStrategy"); got != "custom" {
		t.Fatalf("shareAddrStrategy = %q, want custom", got)
	}
	if got := values.Get("shareAddr"); got != "edge.example.com" {
		t.Fatalf("shareAddr = %q, want edge.example.com", got)
	}
}

func TestRemoteHTTPClientEgressProxy(t *testing.T) {
	// OutboundTag + a resolver → a dedicated proxy client (not the shared default).
	withTag := NewRemote(&model.Node{Id: 1, Scheme: "https", TlsVerifyMode: "verify", OutboundTag: "warp"}, stubEgress{url: "socks5://127.0.0.1:1080"})
	c, err := withTag.httpClient()
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}
	if c == defaultNodeHTTPClient {
		t.Fatal("OutboundTag + resolver must produce a dedicated egress client, not the shared default")
	}
	// No OutboundTag → no egress proxy → shared default client (verify mode).
	noTag := NewRemote(&model.Node{Id: 2, Scheme: "https", TlsVerifyMode: "verify"}, stubEgress{url: "socks5://127.0.0.1:1080"})
	c2, err := noTag.httpClient()
	if err != nil {
		t.Fatalf("httpClient: %v", err)
	}
	if c2 != defaultNodeHTTPClient {
		t.Fatal("no OutboundTag must use the shared default client")
	}
}

func TestRemoteDoSetsContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForServer(t, srv, "skip", ""), nil)
	if _, err := r.do(context.Background(), http.MethodPost, "x", url.Values{"a": {"b"}}); err != nil {
		t.Fatalf("do: %v", err)
	}
	if gotCT != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", gotCT)
	}
}

func TestRemoteBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		scheme  string
		port    int
		bp      string
		want    string
		wantErr bool
	}{
		{"https default path", "https", 443, "", "https://example.com:443/", false},
		{"http custom path gets trailing slash", "http", 8080, "/panel", "http://example.com:8080/panel/", false},
		{"empty scheme defaults to https", "", 2096, "/", "https://example.com:2096/", false},
		{"invalid scheme defaults to https", "ftp", 2096, "/", "https://example.com:2096/", false},
		{"port zero rejected", "https", 0, "/", "", true},
		{"port above range rejected", "https", 65536, "/", "", true},
		{"negative port rejected", "https", -1, "/", "", true},
		{"max port accepted", "https", 65535, "/", "https://example.com:65535/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewRemote(&model.Node{Address: "example.com", Scheme: c.scheme, Port: c.port, BasePath: c.bp}, nil)
			got, err := r.baseURL()
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for scheme=%q port=%d", c.scheme, c.port)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("baseURL = %q, want %q", got, c.want)
			}
		})
	}
}

func TestIsNonEmptySlice(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{"non-empty slice", []any{1}, true},
		{"empty slice", []any{}, false},
		{"nil slice", []any(nil), false},
		{"not a slice", "x", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isNonEmptySlice(c.in); got != c.want {
				t.Fatalf("isNonEmptySlice(%#v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestWireInboundTrafficReset(t *testing.T) {
	with := wireInbound(&model.Inbound{TrafficReset: "daily"})
	if got := with.Get("trafficReset"); got != "daily" {
		t.Fatalf("trafficReset = %q, want daily", got)
	}
	// Empty TrafficReset must be omitted entirely, not sent as an empty field.
	without := wireInbound(&model.Inbound{})
	if without.Has("trafficReset") {
		t.Fatalf("trafficReset must be omitted when empty, got %q", without.Get("trafficReset"))
	}
}

func TestWireInboundDefaultsShareAddressStrategy(t *testing.T) {
	values := wireInbound(&model.Inbound{})

	if got := values.Get("shareAddrStrategy"); got != "node" {
		t.Fatalf("shareAddrStrategy = %q, want node", got)
	}

	values = wireInbound(&model.Inbound{ShareAddrStrategy: "auto"})
	if got := values.Get("shareAddrStrategy"); got != "node" {
		t.Fatalf("invalid shareAddrStrategy = %q, want node", got)
	}
}

func TestSanitizeStreamSettingsForRemote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// wantCertFile / wantKeyFile: expected presence after sanitize
		wantCertFile bool
		wantKeyFile  bool
	}{
		{
			name: "file paths only — kept intact (remote node paths)",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificateFile": "/etc/ssl/cert.crt",
						"keyFile": "/etc/ssl/key.key"
					}]
				}
			}`,
			wantCertFile: true,
			wantKeyFile:  true,
		},
		{
			name: "inline content only — unchanged",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificate": ["-----BEGIN CERTIFICATE-----"],
						"key": ["-----BEGIN PRIVATE KEY-----"]
					}]
				}
			}`,
			wantCertFile: false,
			wantKeyFile:  false,
		},
		{
			name: "both file paths and inline content — file paths stripped (redundant)",
			input: `{
				"tlsSettings": {
					"certificates": [{
						"certificateFile": "/etc/ssl/cert.crt",
						"keyFile": "/etc/ssl/key.key",
						"certificate": ["-----BEGIN CERTIFICATE-----"],
						"key": ["-----BEGIN PRIVATE KEY-----"]
					}]
				}
			}`,
			wantCertFile: false,
			wantKeyFile:  false,
		},
		{
			name:  "empty stream settings",
			input: "",
			// empty input returns empty, nothing to check
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.input == "" {
				if got := sanitizeStreamSettingsForRemote(tc.input); got != "" {
					t.Errorf("expected empty string, got %q", got)
				}
				return
			}
			got := sanitizeStreamSettingsForRemote(tc.input)
			var out map[string]any
			if err := json.Unmarshal([]byte(got), &out); err != nil {
				t.Fatalf("output is not valid JSON: %v\noutput: %s", err, got)
			}

			tls, _ := out["tlsSettings"].(map[string]any)
			certs, _ := tls["certificates"].([]any)
			if len(certs) == 0 {
				t.Fatal("certificates array missing in output")
			}
			cert, _ := certs[0].(map[string]any)

			_, hasCertFile := cert["certificateFile"]
			_, hasKeyFile := cert["keyFile"]

			if hasCertFile != tc.wantCertFile {
				t.Errorf("certificateFile present=%v, want %v", hasCertFile, tc.wantCertFile)
			}
			if hasKeyFile != tc.wantKeyFile {
				t.Errorf("keyFile present=%v, want %v", hasKeyFile, tc.wantKeyFile)
			}
		})
	}
}
