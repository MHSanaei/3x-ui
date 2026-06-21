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

// The config name is always the inbound's own remark; the host endpoint's remark
// never substitutes it (it is reachable only through {{HOST}}).
func TestGenHostRemark_ConfigNameUsesInbound(t *testing.T) {
	s, inbound, client := hostRemarkService("") // no template → config name only
	if got := s.genHostRemark(inbound, client, "Relay", ""); got != "DE" {
		t.Fatalf("genHostRemark = %q, want %q (inbound remark, host ignored)", got, "DE")
	}
	if got := s.genHostRemark(inbound, client, "", ""); got != "DE" {
		t.Fatalf("genHostRemark (no host remark) = %q, want %q", got, "DE")
	}
}

// In the body the template applies: {{INBOUND}} is always the inbound's remark
// and {{HOST}} the host's own remark, so the two can be shown side by side.
func TestGenHostRemark_GlobalTemplate(t *testing.T) {
	// {{INBOUND}} resolves to the inbound remark regardless of the host remark.
	s, inbound, client := hostRemarkService("{{INBOUND}} | {{TRAFFIC_LEFT}} | {{DAYS_LEFT}}d")
	if got := s.genHostRemark(inbound, client, "CDN", ""); got != "DE | 80.00GB | 10d" {
		t.Fatalf("global template ({{INBOUND}} = inbound) = %q", got)
	}
	// {{INBOUND}} and {{HOST}} side by side show both, distinctly (#5443).
	s2, inbound2, client2 := hostRemarkService("{{INBOUND}}|{{HOST}}|{{TRAFFIC_LEFT}}")
	if got := s2.genHostRemark(inbound2, client2, "CDN", ""); got != "DE|CDN|80.00GB" {
		t.Fatalf("global template (inbound + host) = %q, want %q", got, "DE|CDN|80.00GB")
	}
	// {{HOST}} is the host's own remark even when the inbound has one of its own.
	s3, inbound3, client3 := hostRemarkService("{{HOST}}")
	if got := s3.genHostRemark(inbound3, client3, "CDN", ""); got != "CDN" {
		t.Fatalf("{{HOST}} token = %q, want CDN", got)
	}
}

// A global template also drives non-host links via genRemark; {{HOST}} = the
// legacy externalProxy remark passed as extra.
func TestGenRemark_GlobalTemplate(t *testing.T) {
	s, inbound, _ := hostRemarkService("{{EMAIL}} | {{TRAFFIC_LEFT}}")
	got := s.genRemark(inbound, "john@example.com", "", "")
	if got != "john@example.com | 80.00GB" {
		t.Fatalf("global template (non-host) = %q", got)
	}
}

// With no template, genRemark composes the fallback model and adds no suffix.
func TestGenRemark_NoTemplate_NoSuffix(t *testing.T) {
	s, inbound, _ := hostRemarkService("")
	got := s.genRemark(inbound, "john@example.com", "Relay", "")
	if got != "DE-Relay" {
		t.Fatalf("genRemark = %q, want %q (no suffix)", got, "DE-Relay")
	}
}

// The per-client info part of the template renders only on a client's first
// link of the request; later links show the name-only template.
func TestUsageOnFirstLinkOnly(t *testing.T) {
	s, inbound, client := hostRemarkService("{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D")
	first := s.genHostRemark(inbound, client, "", "")
	second := s.genHostRemark(inbound, client, "", "")
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
	// A host link in a display shows only the config name — the inbound's remark,
	// with no per-client email or usage info and the host remark ignored.
	if got := s.genHostRemark(inbound, client, "CDN", ""); got != "DE" {
		t.Fatalf("display host link = %q, want config name %q", got, "DE")
	}
	// With no host remark, the config name is likewise the inbound's own remark.
	if got := s.genHostRemark(inbound, client, "", ""); got != "DE" {
		t.Fatalf("display host link (no host) = %q, want %q", got, "DE")
	}
	// genRemark (non-host) likewise drops the template in display context.
	if got := s.genRemark(inbound, client.Email, "", ""); got != "DE" {
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

// statsForClient resolves usage from the per-request statsByEmail map when the
// link's own inbound doesn't carry the client's (globally unique) traffic row —
// the multi-inbound case that made {{TRAFFIC_LEFT}} show the full quota (#5443).
func TestStatsForClient_CrossInboundFallback(t *testing.T) {
	s := &SubService{
		statsByEmail: map[string]xray.ClientTraffic{
			"john@example.com": {Email: "john@example.com", Total: 100 * gb, Up: 15 * gb, Down: 5 * gb},
		},
	}
	// Inbound B carries no ClientStats for john (his row is owned by inbound A).
	inboundB := &model.Inbound{Remark: "B"}
	st := s.statsForClient(inboundB, model.Client{Email: "john@example.com"})
	if used := st.Up + st.Down; used != 20*gb {
		t.Fatalf("statsForClient used = %d, want %d (cross-inbound fallback)", used, 20*gb)
	}
	if got := remarkVarValue("TRAFFIC_LEFT", remarkContext{stats: st}); got != "80.00GB" {
		t.Fatalf("TRAFFIC_LEFT = %q, want 80.00GB (remaining, not total)", got)
	}
}

// Two clients through the same global template get distinct, per-client remarks.
func TestGenHostRemark_PerClient(t *testing.T) {
	s := &SubService{remarkTemplate: "{{EMAIL}}", subscriptionBody: true}
	inbound := &model.Inbound{}
	a := s.genHostRemark(inbound, model.Client{Email: "alice@x"}, "", "")
	b := s.genHostRemark(inbound, model.Client{Email: "bob@x"}, "", "")
	if a != "alice@x" || b != "bob@x" {
		t.Fatalf("per-client expansion failed: a=%q b=%q", a, b)
	}
}

func TestStatusEmoji(t *testing.T) {
	cases := []struct {
		stats xray.ClientTraffic
		want  string
	}{
		{xray.ClientTraffic{Enable: true, Total: 10 * gb, Up: gb}, "✅"},
		{xray.ClientTraffic{Enable: true, Total: 10 * gb, Up: 10 * gb, Down: 1}, "🚫"},
		{xray.ClientTraffic{Enable: false}, "🚫"},
		{xray.ClientTraffic{Enable: true, ExpiryTime: 1000}, "⏳"},
	}
	for _, c := range cases {
		if got := statusEmoji(c.stats); got != c.want {
			t.Errorf("statusEmoji(%+v) = %q, want %q", c.stats, got, c.want)
		}
	}
}

func TestUsagePercentage(t *testing.T) {
	if got := usagePercentage(xray.ClientTraffic{Total: 100 * gb, Up: 25 * gb, Down: 25 * gb}); got != "50.0%" {
		t.Errorf("usagePercentage 50%% = %q", got)
	}
	if got := usagePercentage(xray.ClientTraffic{Total: 0}); got != "" {
		t.Errorf("usagePercentage unlimited = %q, want empty", got)
	}
	if got := usagePercentage(xray.ClientTraffic{Total: 10 * gb, Up: 10 * gb}); got != "100.0%" {
		t.Errorf("usagePercentage 100%% = %q", got)
	}
	// Over-quota usage clamps to 100%, consistent with TRAFFIC_LEFT.
	if got := usagePercentage(xray.ClientTraffic{Total: 10 * gb, Up: 25 * gb}); got != "100.0%" {
		t.Errorf("usagePercentage over-quota = %q, want 100.0%%", got)
	}
}

func TestTimeLeftLabel(t *testing.T) {
	if got := timeLeftLabel(0); got != "∞" {
		t.Errorf("timeLeftLabel(0) = %q, want ∞", got)
	}
	// Delayed-start: negative expiry = duration in ms. 1000ms = 1 second = "0m".
	if got := timeLeftLabel(-1000); got != "0m" {
		t.Errorf("timeLeftLabel(-1000) = %q, want 0m", got)
	}
}

func TestGregorianToJalali(t *testing.T) {
	cases := []struct {
		gy, gm, gd int
		jy, jm, jd int
	}{
		{2024, 1, 1, 1402, 10, 11},
		{2000, 3, 20, 1379, 1, 1},
		{1979, 2, 11, 1357, 11, 22},
	}
	for _, c := range cases {
		jy, jm, jd := gregorianToJalali(c.gy, c.gm, c.gd)
		if jy != c.jy || jm != c.jm || jd != c.jd {
			t.Errorf("gregorianToJalali(%d,%d,%d) = (%d,%d,%d), want (%d,%d,%d)",
				c.gy, c.gm, c.gd, jy, jm, jd, c.jy, c.jm, c.jd)
		}
	}
}

func TestJalaliExpireDateLabel(t *testing.T) {
	if got := jalaliExpireDateLabel(0); got != "" {
		t.Errorf("jalaliExpireDateLabel(0) = %q, want empty", got)
	}
	if got := jalaliExpireDateLabel(-1000); got != "" {
		t.Errorf("jalaliExpireDateLabel(-1000) = %q, want empty", got)
	}
}

func TestExpandNewTokensInTemplate(t *testing.T) {
	inbound := &model.Inbound{Remark: "DE", Protocol: "vless"}
	client := model.Client{Email: "alice@test.com", ID: "abc-123"}
	stats := xray.ClientTraffic{Enable: true, Total: 100 * gb, Up: 50 * gb, Down: 0}
	ctx := remarkContext{
		client:    client,
		stats:     stats,
		inbound:   inbound,
		transport: "ws",
	}

	cases := []struct{ tmpl, want string }{
		{"{{STATUS_EMOJI}}", "✅"},
		{"{{USAGE_PERCENTAGE}}", "50.0%"},
		{"{{PROTOCOL}}", "VLESS"},
		{"{{TRANSPORT}}", "ws"},
		{"{{STATUS_EMOJI}} {{INBOUND}}", "✅ DE"},
	}
	for _, c := range cases {
		if got := expandRemarkVars(c.tmpl, ctx); got != c.want {
			t.Errorf("expandRemarkVars(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
}

func TestTranslateUISingleBrackets(t *testing.T) {
	cases := []struct{ in, want string }{
		{"{EMAIL}", "{{EMAIL}}"},
		{"{DATA_LEFT}", "{{TRAFFIC_LEFT}}"},
		{"{DATA_LEFT} of {DATA_LIMIT}", "{{TRAFFIC_LEFT}} of {{TRAFFIC_TOTAL}}"},
		{"{STATUS_EMOJI} {INBOUND}", "{{STATUS_EMOJI}} {INBOUND}"},
		{"{UNKNOWN_TOKEN}", "{UNKNOWN_TOKEN}"},
		{"no braces", "no braces"},
		{"{{TRAFFIC_LEFT}}", "{{TRAFFIC_LEFT}}"},
		{"{username}", "{username}"},
	}
	for _, c := range cases {
		if got := translateUISingleBrackets(c.in); got != c.want {
			t.Errorf("translateUISingleBrackets(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpandRemarkVars_SingleBracketUI(t *testing.T) {
	inbound := &model.Inbound{Remark: "DE", Protocol: "vless"}
	stats := xray.ClientTraffic{Enable: true, Total: 100 * gb, Up: 50 * gb, Down: 0}
	ctx := remarkContext{
		client:    model.Client{Email: "alice@test.com"},
		stats:     stats,
		inbound:   inbound,
		transport: "ws",
	}
	cases := []struct{ tmpl, want string }{
		{"{EMAIL}", "alice@test.com"},
		{"{DATA_LEFT}", "50.00GB"},
		{"{DATA_USAGE}", "50.00GB"},
		{"{DATA_LIMIT}", "100.00GB"},
		{"{STATUS_EMOJI}", "✅"},
		{"{USAGE_PERCENTAGE}", "50.0%"},
		{"{PROTOCOL}", "VLESS"},
		{"{TRANSPORT}", "ws"},
	}
	for _, c := range cases {
		if got := expandRemarkVars(c.tmpl, ctx); got != c.want {
			t.Errorf("expandRemarkVars(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
}

func TestUsageOnFirstLinkOnly_SingleBracket(t *testing.T) {
	s := &SubService{
		remarkTemplate:   "{STATUS_EMOJI} {{INBOUND}}|📊{{TRAFFIC_LEFT}}",
		subscriptionBody: true,
		usageShown:       map[string]bool{},
	}
	inbound := &model.Inbound{
		Remark: "DE",
		ClientStats: []xray.ClientTraffic{{
			Email:  "alice@x",
			Enable: true,
			Total:  100 * gb,
			Up:     20 * gb,
			Down:   10 * gb,
		}},
	}
	client := model.Client{Email: "alice@x"}
	first := s.genTemplatedRemark(inbound, client, "", "ws")
	s.usageShown["alice@x"] = true
	second := s.genTemplatedRemark(inbound, client, "", "ws")
	if !strings.Contains(first, "📊") {
		t.Fatalf("first link should carry usage: %q", first)
	}
	if strings.Contains(second, "📊") {
		t.Fatalf("second link must not carry usage: %q", second)
	}
}
