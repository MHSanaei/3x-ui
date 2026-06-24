package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestRemoteDo_RejectsOversizeResponse: a node streaming a body larger than
// maxRemoteResponseBytes must error out instead of the master buffering it
// unbounded.
func TestRemoteDo_RejectsOversizeResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		chunk := bytes.Repeat([]byte("a"), 1<<20) // 1 MiB
		for written := 0; written <= maxRemoteResponseBytes; written += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return // client stopped reading at the cap
			}
		}
	}))
	defer srv.Close()

	r := NewRemote(nodeForServer(t, srv, "skip", ""), nil)
	if _, err := r.do(context.Background(), http.MethodGet, "/probe", nil); !errors.Is(err, errRemoteResponseTooLarge) {
		t.Fatalf("do() error = %v, want errRemoteResponseTooLarge", err)
	}
}

// TestRemoteDo_AcceptsNormalResponse confirms the cap does not break a normal
// under-limit envelope.
func TestRemoteDo_AcceptsNormalResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"msg":"ok","obj":{"x":1}}`))
	}))
	defer srv.Close()

	r := NewRemote(nodeForServer(t, srv, "skip", ""), nil)
	env, err := r.do(context.Background(), http.MethodGet, "/probe", nil)
	if err != nil {
		t.Fatalf("do() unexpected error: %v", err)
	}
	if env == nil || !env.Success {
		t.Fatalf("env = %+v, want Success=true", env)
	}
}

// TestReadCappedBody_Boundary pins the cap+1 contract cheaply (no large allocs):
// a body of exactly limit is accepted; limit+1 and beyond are rejected.
func TestReadCappedBody_Boundary(t *testing.T) {
	const limit = 8
	cases := []struct {
		name    string
		n       int
		wantErr bool
	}{
		{"under", limit - 1, false},
		{"exact", limit, false},
		{"over-by-one", limit + 1, true},
		{"way-over", limit * 4, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := readCappedBody(bytes.NewReader(bytes.Repeat([]byte("x"), c.n)), limit)
			if c.wantErr {
				if !errors.Is(err, errRemoteResponseTooLarge) {
					t.Fatalf("n=%d: err=%v, want errRemoteResponseTooLarge", c.n, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("n=%d: unexpected err %v", c.n, err)
			}
			if len(raw) != c.n {
				t.Fatalf("n=%d: read %d bytes, want %d", c.n, len(raw), c.n)
			}
		})
	}
}

// TestRemoteDo_NonOKStatusReturnsHTTPError confirms a non-OK status is reported
// as an HTTP error (with a bounded diagnostic snippet) rather than being read as
// a success payload — i.e. status precedence over the body.
func TestRemoteDo_NonOKStatusReturnsHTTPError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	r := NewRemote(nodeForServer(t, srv, "skip", ""), nil)
	_, err := r.do(context.Background(), http.MethodGet, "/probe", nil)
	if err == nil {
		t.Fatal("do() error = nil, want HTTP 500 error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %q, want it to mention HTTP 500 and the body snippet", err)
	}
}

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
	}, 0)

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
	with := wireInbound(&model.Inbound{TrafficReset: "daily"}, 0)
	if got := with.Get("trafficReset"); got != "daily" {
		t.Fatalf("trafficReset = %q, want daily", got)
	}
	// Empty TrafficReset must be omitted entirely, not sent as an empty field.
	without := wireInbound(&model.Inbound{}, 0)
	if without.Has("trafficReset") {
		t.Fatalf("trafficReset must be omitted when empty, got %q", without.Get("trafficReset"))
	}
}

func TestWireInboundDefaultsShareAddressStrategy(t *testing.T) {
	values := wireInbound(&model.Inbound{}, 0)

	if got := values.Get("shareAddrStrategy"); got != "node" {
		t.Fatalf("shareAddrStrategy = %q, want node", got)
	}

	values = wireInbound(&model.Inbound{ShareAddrStrategy: "auto"}, 0)
	if got := values.Get("shareAddrStrategy"); got != "node" {
		t.Fatalf("invalid shareAddrStrategy = %q, want node", got)
	}
}

func TestStripNodeInboundTagPrefix(t *testing.T) {
	cases := []struct {
		nodeID int
		tag    string
		want   string
	}{
		{2, "n2-in-443-tcp", "in-443-tcp"},
		{2, "in-443-tcp", "in-443-tcp"},
		{2, "my-custom", "my-custom"},
		{2, "n3-in-443-tcp", "n3-in-443-tcp"},
		{0, "n2-in-443-tcp", "n2-in-443-tcp"},
	}
	for _, c := range cases {
		if got := stripNodeInboundTagPrefix(c.nodeID, c.tag); got != c.want {
			t.Fatalf("stripNodeInboundTagPrefix(%d, %q) = %q, want %q", c.nodeID, c.tag, got, c.want)
		}
	}
}

func TestWireInboundStripsNodeTagOnPush(t *testing.T) {
	values := wireInbound(&model.Inbound{Tag: "n2-in-443-tcp"}, 2)
	if got := values.Get("tag"); got != "in-443-tcp" {
		t.Fatalf("tag = %q, want in-443-tcp", got)
	}
	values = wireInbound(&model.Inbound{Tag: "n2-in-443-tcp"}, 0)
	if got := values.Get("tag"); got != "n2-in-443-tcp" {
		t.Fatalf("nodeID 0 must not strip, got %q", got)
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
