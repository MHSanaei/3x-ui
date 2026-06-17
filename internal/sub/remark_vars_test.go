package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const gb = int64(1024 * 1024 * 1024)

// expandCtx builds a remarkContext from explicit pieces for token tests.
func expandCtx(client model.Client, stats xray.ClientTraffic, inbound *model.Inbound) remarkContext {
	return remarkContext{client: client, stats: stats, inbound: inbound}
}

func TestExpandRemarkVars(t *testing.T) {
	inbound := &model.Inbound{Remark: "Germany"}
	client := model.Client{
		Email:     "john@example.com",
		ID:        "3f2a9c1b-aaaa-bbbb-cccc-1234567890ab",
		TgID:      123456789,
		SubID:     "subABC",
		Comment:   "vip",
		Reset:     30,
		CreatedAt: 1_700_000_000_000,
	}
	// 50GB total, 8GB used (5 up + 3 down), enabled, no expiry.
	stats := xray.ClientTraffic{
		Enable: true,
		Total:  50 * gb,
		Up:     5 * gb,
		Down:   3 * gb,
	}
	ctx := expandCtx(client, stats, inbound)

	cases := []struct{ tmpl, want string }{
		{"{{EMAIL}}", "john@example.com"},
		{"{{USERNAME}}", "john@example.com"},
		{"{{INBOUND}}", "Germany"}, // no host remark in ctx → inbound remark
		{"{{HOST}}", ""},           // no host remark in ctx → empty
		{"{{ID}}", client.ID},
		{"{{SHORT_ID}}", "3f2a9c1b"},
		{"{{TELEGRAM_ID}}", "123456789"},
		{"{{SUB_ID}}", "subABC"},
		{"{{COMMENT}}", "vip"},
		{"{{RESET_DAYS}}", "30"},
		{"{{CREATED_UNIX}}", "1700000000"},
		{"{{TRAFFIC_USED}}", "8.00GB"},
		{"{{TRAFFIC_LEFT}}", "42.00GB"},
		{"{{TRAFFIC_TOTAL}}", "50.00GB"},
		{"{{TRAFFIC_USED_BYTES}}", "8589934592"},
		{"{{TRAFFIC_TOTAL_BYTES}}", "53687091200"},
		{"{{UP}}", "5.00GB"},
		{"{{DOWN}}", "3.00GB"},
		{"{{STATUS}}", "active"},
		{"{{EXPIRE_UNIX}}", "0"},  // no expiry
		{"{{EXPIRE_DATE}}", ""},   // no fixed date
		{"{{UNKNOWN_TOKEN}}", ""}, // unknown → empty, never literal
		{"DE {{EMAIL}} ok", "DE john@example.com ok"},
		{"{{EMAIL}}-{{SHORT_ID}}", "john@example.com-3f2a9c1b"},
		{"no tokens here", "no tokens here"},
	}
	for _, c := range cases {
		if got := expandRemarkVars(c.tmpl, ctx); got != c.want {
			t.Errorf("expandRemarkVars(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
	// The unlimited tokens still render ∞ at the value layer; expandRemarkVars
	// is what drops an all-unlimited segment (see TestExpandRemarkVars_DropUnlimitedSegments).
	if got := remarkVarValue("DAYS_LEFT", ctx); got != "∞" {
		t.Errorf("remarkVarValue(DAYS_LEFT) = %q, want ∞", got)
	}
}

func TestExpandRemarkVars_EdgeCases(t *testing.T) {
	// Unlimited total → ∞ for human forms, 0 bytes for *_BYTES left. Checked at
	// the value layer: expandRemarkVars would drop a bare ∞ segment.
	unlimited := expandCtx(model.Client{}, xray.ClientTraffic{Enable: true, Total: 0, Up: gb}, nil)
	if got := remarkVarValue("TRAFFIC_TOTAL", unlimited); got != "∞" {
		t.Errorf("unlimited TRAFFIC_TOTAL = %q, want ∞", got)
	}
	if got := remarkVarValue("TRAFFIC_LEFT", unlimited); got != "∞" {
		t.Errorf("unlimited TRAFFIC_LEFT = %q, want ∞", got)
	}
	if got := expandRemarkVars("{{TRAFFIC_LEFT_BYTES}}", unlimited); got != "0" {
		t.Errorf("unlimited TRAFFIC_LEFT_BYTES = %q, want 0", got)
	}
	// TgID zero → empty.
	if got := expandRemarkVars("{{TELEGRAM_ID}}", unlimited); got != "" {
		t.Errorf("zero TgID = %q, want empty", got)
	}
	// Over-quota usage clamps left to 0, not negative.
	over := expandCtx(model.Client{}, xray.ClientTraffic{Enable: true, Total: gb, Up: 2 * gb}, nil)
	if got := expandRemarkVars("{{TRAFFIC_LEFT_BYTES}}", over); got != "0" {
		t.Errorf("over-quota TRAFFIC_LEFT_BYTES = %q, want 0", got)
	}
	// Delayed-start (negative expiry) gives deterministic whole days.
	delayed := expandCtx(model.Client{}, xray.ClientTraffic{Enable: true, ExpiryTime: -864_000_000}, nil)
	if got := expandRemarkVars("{{DAYS_LEFT}}", delayed); got != "10" {
		t.Errorf("delayed-start DAYS_LEFT = %q, want 10", got)
	}
}

// An unlimited client drops the quota/expiry segments whole — decoration and the
// "|" separator included — instead of printing "📊∞|⏳∞D".
func TestExpandRemarkVars_DropUnlimitedSegments(t *testing.T) {
	const tmpl = "{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D"
	inbound := &model.Inbound{Remark: "host"}

	// No limit at all → only the name segment survives.
	unlimited := expandCtx(model.Client{}, xray.ClientTraffic{Enable: true}, inbound)
	if got := expandRemarkVars(tmpl, unlimited); got != "host" {
		t.Errorf("fully unlimited = %q, want %q", got, "host")
	}

	// Limited traffic but no expiry → traffic stays, the expiry segment drops.
	noExpiry := expandCtx(model.Client{}, xray.ClientTraffic{Enable: true, Total: 50 * gb, Up: 8 * gb}, inbound)
	if got := expandRemarkVars(tmpl, noExpiry); got != "host|📊42.00GB" {
		t.Errorf("no-expiry = %q, want %q", got, "host|📊42.00GB")
	}

	// A segment mixing an unlimited token with another value is kept whole,
	// decoration and ∞ included — only all-unlimited segments drop.
	mixed := expandCtx(model.Client{Email: "john"}, xray.ClientTraffic{Enable: true}, inbound)
	if got := expandRemarkVars("{{EMAIL}} 📊{{TRAFFIC_LEFT}}", mixed); got != "john 📊∞" {
		t.Errorf("mixed segment = %q, want %q", got, "john 📊∞")
	}
}

func TestClientStatus(t *testing.T) {
	cases := []struct {
		name string
		st   xray.ClientTraffic
		want string
	}{
		{"disabled", xray.ClientTraffic{Enable: false}, "disabled"},
		{"active", xray.ClientTraffic{Enable: true}, "active"},
		{"expired", xray.ClientTraffic{Enable: true, ExpiryTime: 1000}, "expired"}, // 1s past epoch
		{"depleted", xray.ClientTraffic{Enable: true, Total: gb, Up: gb}, "depleted"},
	}
	for _, c := range cases {
		if got := clientStatus(c.st); got != c.want {
			t.Errorf("%s: clientStatus = %q, want %q", c.name, got, c.want)
		}
	}
}

// hostRemarkService builds a SubService + inbound + client/stats for remark tests.
func hostRemarkService(template string) (*SubService, *model.Inbound, model.Client) {
	s := &SubService{remarkTemplate: template, subscriptionBody: true}
	inbound := &model.Inbound{
		Remark: "DE",
		ClientStats: []xray.ClientTraffic{{
			Email:      "john@example.com",
			Enable:     true,
			Total:      100 * gb,
			Up:         15 * gb,
			Down:       5 * gb,
			ExpiryTime: -864_000_000, // delayed-start: deterministic 10 days
		}},
	}
	client := model.Client{Email: "john@example.com"}
	return s, inbound, client
}

// The config name prefers the host endpoint's own remark; the inbound's remark is
// the fallback, used only when the host has none.
func TestGenHostRemark_ConfigNameHostWins(t *testing.T) {
	s, inbound, client := hostRemarkService("") // no template → config name only
	if got := s.genHostRemark(inbound, client, "Relay"); got != "Relay" {
		t.Fatalf("genHostRemark = %q, want %q (host remark wins)", got, "Relay")
	}
	if got := s.genHostRemark(inbound, client, ""); got != "DE" {
		t.Fatalf("genHostRemark (no host remark) = %q, want %q (inbound fallback)", got, "DE")
	}
}

// In the body the template applies: {{INBOUND}} is the config name (host remark
// first, inbound fallback) and {{HOST}} is always the host's own remark.
func TestGenHostRemark_GlobalTemplate(t *testing.T) {
	// Host remark set → {{INBOUND}} resolves to it (host wins over the inbound).
	s, inbound, client := hostRemarkService("{{INBOUND}} | {{TRAFFIC_LEFT}} | {{DAYS_LEFT}}d")
	if got := s.genHostRemark(inbound, client, "CDN"); got != "CDN | 80.00GB | 10d" {
		t.Fatalf("global template (host wins) = %q", got)
	}
	// No host remark → {{INBOUND}} falls back to the inbound's own remark.
	s2, inbound2, client2 := hostRemarkService("{{INBOUND}} | {{TRAFFIC_LEFT}}")
	if got := s2.genHostRemark(inbound2, client2, ""); got != "DE | 80.00GB" {
		t.Fatalf("global template (inbound fallback) = %q", got)
	}
	// {{HOST}} is the host's own remark even when the inbound has one of its own.
	s3, inbound3, client3 := hostRemarkService("{{HOST}}")
	if got := s3.genHostRemark(inbound3, client3, "CDN"); got != "CDN" {
		t.Fatalf("{{HOST}} token = %q, want CDN", got)
	}
}

// A global template also drives non-host links via genRemark; {{HOST}} = the
// legacy externalProxy remark passed as extra.
func TestGenRemark_GlobalTemplate(t *testing.T) {
	s, inbound, _ := hostRemarkService("{{EMAIL}} | {{TRAFFIC_LEFT}}")
	got := s.genRemark(inbound, "john@example.com", "")
	if got != "john@example.com | 80.00GB" {
		t.Fatalf("global template (non-host) = %q", got)
	}
}

// With no template, genRemark composes the fallback model and adds no suffix.
func TestGenRemark_NoTemplate_NoSuffix(t *testing.T) {
	s, inbound, _ := hostRemarkService("")
	got := s.genRemark(inbound, "john@example.com", "Relay")
	if got != "DE-Relay" {
		t.Fatalf("genRemark = %q, want %q (no suffix)", got, "DE-Relay")
	}
}

// The per-client info part of the template renders only on a client's first
// link of the request; later links show the name-only template.
func TestUsageOnFirstLinkOnly(t *testing.T) {
	s, inbound, client := hostRemarkService("{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D")
	first := s.genHostRemark(inbound, client, "")
	second := s.genHostRemark(inbound, client, "")
	if !strings.Contains(first, "📊") || !strings.Contains(first, "80.00GB") {
		t.Fatalf("first link should carry usage: %q", first)
	}
	if strings.ContainsAny(second, "📊⏳") {
		t.Fatalf("second link must not carry usage: %q", second)
	}
	if second != "DE" {
		t.Fatalf("second link = %q, want name-only %q", second, "DE")
	}
}

// Outside the subscription body (panel link/QR displays, sub info page) the
// template is bypassed entirely — links show just the config name, with no
// per-client email or usage info.
func TestRemarkInDisplayContext(t *testing.T) {
	s, inbound, client := hostRemarkService("{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D")
	s.subscriptionBody = false
	// A host link in a display shows only the config name — host remark wins, with
	// no per-client email or usage info.
	if got := s.genHostRemark(inbound, client, "CDN"); got != "CDN" {
		t.Fatalf("display host link = %q, want config name %q (host wins)", got, "CDN")
	}
	// With no host remark, the config name is the inbound's own remark.
	if got := s.genHostRemark(inbound, client, ""); got != "DE" {
		t.Fatalf("display host link (no host) = %q, want %q", got, "DE")
	}
	// genRemark (non-host) likewise drops the template in display context.
	if got := s.genRemark(inbound, client.Email, ""); got != "DE" {
		t.Fatalf("display genRemark = %q, want %q", got, "DE")
	}
}

// nameOnlyTemplate drops the info part (and its leading decoration), keeping name.
func TestNameOnlyTemplate(t *testing.T) {
	cases := map[string]string{
		"{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D": "{{INBOUND}}",           // the default → name only
		"{{EMAIL}} {{INBOUND}} ⏳{{DAYS_LEFT}}":          "{{EMAIL}} {{INBOUND}}", // multi-token name survives the trim
		"{{INBOUND}} | {{STATUS}}":                      "{{INBOUND}}",
		"{{INBOUND}}-{{EMAIL}}":                         "{{INBOUND}}-{{EMAIL}}", // no info tokens → unchanged
		"{{TRAFFIC_LEFT}}":                              "",                      // info only → empty
	}
	for tmpl, want := range cases {
		if got := nameOnlyTemplate(tmpl); got != want {
			t.Errorf("nameOnlyTemplate(%q) = %q, want %q", tmpl, got, want)
		}
	}
}

// Two clients through the same global template get distinct, per-client remarks.
func TestGenHostRemark_PerClient(t *testing.T) {
	s := &SubService{remarkTemplate: "{{EMAIL}}", subscriptionBody: true}
	inbound := &model.Inbound{}
	a := s.genHostRemark(inbound, model.Client{Email: "alice@x"}, "")
	b := s.genHostRemark(inbound, model.Client{Email: "bob@x"}, "")
	if a != "alice@x" || b != "bob@x" {
		t.Fatalf("per-client expansion failed: a=%q b=%q", a, b)
	}
}
