package sub

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// N1 — externalProxyToEndpoint maps the scalar fields and carries the source
// entry so delegated TLS application reproduces the legacy presence-tracked
// overrides (absent key never clobbers an upstream value).
func TestExternalProxyToEndpoint(t *testing.T) {
	ep := map[string]any{
		"forceTls": "tls",
		"dest":     "cdn.example.com",
		"port":     float64(8443),
		"remark":   "R",
		"sni":      "s.example.com",
	}
	e := externalProxyToEndpoint(ep)
	if e.Address != "cdn.example.com" {
		t.Fatalf("Address = %q, want cdn.example.com", e.Address)
	}
	if e.Port != 8443 {
		t.Fatalf("Port = %d, want 8443", e.Port)
	}
	if e.ForceTls != "tls" {
		t.Fatalf("ForceTls = %q, want tls", e.ForceTls)
	}
	if e.Remark != "R" {
		t.Fatalf("Remark = %q, want R", e.Remark)
	}
	if e.ep == nil {
		t.Fatalf("ep not carried; delegated TLS application would lose the source entry")
	}
	// Delegation preserves the sni override and does not invent absent fields.
	params := map[string]string{}
	applyEndpointTLSParams(e, params, "tls")
	if params["sni"] != "s.example.com" {
		t.Fatalf("delegated sni = %q, want s.example.com", params["sni"])
	}
	if _, ok := params["fp"]; ok {
		t.Fatalf("absent fingerprint must not be set, got fp=%q", params["fp"])
	}
}

// N2 — inboundDefaultEndpoint reproduces the no-externalProxy default: resolved
// address + inbound port, forceTls "same", empty remark, no source entry.
func TestInboundDefaultEndpoint(t *testing.T) {
	in := &model.Inbound{Listen: "198.51.100.7", Port: 8080}
	s := &SubService{}
	e := s.inboundDefaultEndpoint(in)
	if e.Address != "198.51.100.7" {
		t.Fatalf("Address = %q, want 198.51.100.7", e.Address)
	}
	if e.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", e.Port)
	}
	if e.ForceTls != "same" {
		t.Fatalf("ForceTls = %q, want same", e.ForceTls)
	}
	if e.Remark != "" {
		t.Fatalf("Remark = %q, want empty", e.Remark)
	}
	if e.ep != nil {
		t.Fatalf("default endpoint must not carry a source externalProxy entry")
	}
}

// N3 — buildEndpointLinks renders the param-form path: one link per endpoint,
// TLS override applied for tls, fields stripped + security overridden for none,
// joined by "\n", in order.
func TestBuildEndpointLinks_ParamForm(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	params := map[string]string{"type": "tcp", "security": "tls", "sni": "base.sni", "fp": "chrome"}
	eps := []ShareEndpoint{
		externalProxyToEndpoint(map[string]any{"forceTls": "tls", "dest": "a.example.com", "port": float64(8443), "remark": "A", "sni": "a.sni"}),
		externalProxyToEndpoint(map[string]any{"forceTls": "none", "dest": "b.example.com", "port": float64(80), "remark": "B"}),
	}
	got := s.buildEndpointLinks(eps, params, "tls",
		func(e ShareEndpoint) string { return fmt.Sprintf("vless://uid@%s", joinHostPort(e.Address, e.Port)) },
		func(e ShareEndpoint) string { return s.genRemark(in, "user", e.Remark, "") },
	)
	want := "vless://uid@a.example.com:8443?fp=chrome&security=tls&sni=a.sni&type=tcp#ib-A-user\n" +
		"vless://uid@b.example.com:80?security=none&type=tcp#ib-B-user"
	if got != want {
		t.Fatalf("N3 mismatch.\n got: %q\nwant: %q", got, want)
	}
}

// N4 — buildEndpointVmessLinks renders the object-form path: base obj cloned per
// endpoint, add/port/tls rewritten, sni override applied, none-strip honored.
func TestBuildEndpointVmessLinks(t *testing.T) {
	s := &SubService{}
	in := &model.Inbound{Remark: "ib"}
	baseObj := map[string]any{
		"v": "2", "add": "base.example.com", "port": 443, "type": "none",
		"id": "uid", "scy": "auto", "net": "tcp",
		"tls": "tls", "sni": "base.sni", "alpn": "h2", "fp": "chrome",
	}
	eps := []ShareEndpoint{
		externalProxyToEndpoint(map[string]any{"forceTls": "same", "dest": "a.example.com", "port": float64(8443), "remark": "A", "sni": "a.sni"}),
		externalProxyToEndpoint(map[string]any{"forceTls": "none", "dest": "b.example.com", "port": float64(80), "remark": "B"}),
	}
	got := s.buildEndpointVmessLinks(eps, baseObj, in, "user", "tcp")
	want := "vmess://ewogICJhZGQiOiAiYS5leGFtcGxlLmNvbSIsCiAgImFscG4iOiAiaDIiLAogICJmcCI6ICJjaHJvbWUiLAogICJpZCI6ICJ1aWQiLAogICJuZXQiOiAidGNwIiwKICAicG9ydCI6IDg0NDMsCiAgInBzIjogImliLUEtdXNlciIsCiAgInNjeSI6ICJhdXRvIiwKICAic25pIjogImEuc25pIiwKICAidGxzIjogInRscyIsCiAgInR5cGUiOiAibm9uZSIsCiAgInYiOiAiMiIKfQ==\n" +
		"vmess://ewogICJhZGQiOiAiYi5leGFtcGxlLmNvbSIsCiAgImlkIjogInVpZCIsCiAgIm5ldCI6ICJ0Y3AiLAogICJwb3J0IjogODAsCiAgInBzIjogImliLUItdXNlciIsCiAgInNjeSI6ICJhdXRvIiwKICAidGxzIjogIm5vbmUiLAogICJ0eXBlIjogIm5vbmUiLAogICJ2IjogIjIiCn0="
	if got != want {
		t.Fatalf("N4 mismatch.\n got: %q\nwant: %q", got, want)
	}
}
