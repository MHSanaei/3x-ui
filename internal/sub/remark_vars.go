package sub

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// remarkContext carries the per-client data a remark template can interpolate.
// stats holds the live traffic record when one exists; when it doesn't, the
// caller synthesizes a minimal one from the client so expiry/total/status tokens
// still resolve. hostRemark is the host endpoint's own remark; it backs the
// {{HOST}} token only — it never substitutes the inbound's remark as the config
// name (use {{INBOUND}} and {{HOST}} side by side to show both).
type remarkContext struct {
	client     model.Client
	stats      xray.ClientTraffic
	inbound    *model.Inbound
	hostRemark string
	transport  string
}

// configName is the display name for a link: always the inbound's own remark.
// The host endpoint's remark is surfaced only through the {{HOST}} token.
func (ctx remarkContext) configName() string {
	if ctx.inbound != nil {
		return ctx.inbound.Remark
	}
	return ""
}

// remarkVarRe matches a {{TOKEN}} placeholder. Tokens are uppercase letters and
// underscores only, so ordinary braces in a remark are left untouched.
var remarkVarRe = regexp.MustCompile(`\{\{([A-Z_]+)\}\}`)

// unlimitedMark is the value the human-readable quota/expiry tokens render when
// the client has no limit. A segment built only around such a token carries no
// information, so it is dropped rather than printed as "∞" (see expandRemarkVars).
const unlimitedMark = "∞"

// unlimitedDropTokens are the tokens that render unlimitedMark for an unlimited
// client. A "|"-separated segment whose only value comes from one of these is
// dropped whole when unlimited, so the operator never sees "📊∞|⏳∞D".
var unlimitedDropTokens = map[string]bool{
	"TRAFFIC_LEFT":  true,
	"TRAFFIC_TOTAL": true,
	"DAYS_LEFT":     true,
	"TIME_LEFT":     true,
}

// uiTokenMap translates user-friendly single-brace tokens (used in the frontend
// Remark/Host Name fields) to their internal double-brace equivalents. Tokens
// not present in this map are left untouched.
var uiTokenMap = map[string]string{
	"EMAIL":              "EMAIL",
	"DATA_USAGE":         "TRAFFIC_USED",
	"DATA_LEFT":          "TRAFFIC_LEFT",
	"DATA_LIMIT":         "TRAFFIC_TOTAL",
	"DAYS_LEFT":          "DAYS_LEFT",
	"EXPIRE_DATE":        "EXPIRE_DATE",
	"JALALI_EXPIRE_DATE": "JALALI_EXPIRE_DATE",
	"TIME_LEFT":          "TIME_LEFT",
	"STATUS_EMOJI":       "STATUS_EMOJI",
	"USAGE_PERCENTAGE":   "USAGE_PERCENTAGE",
	"PROTOCOL":           "PROTOCOL",
	"TRANSPORT":          "TRANSPORT",
}

// translateUISingleBrackets converts user-friendly single-brace tokens to the
// internal double-brace format before regex expansion. Only {TOKEN} patterns
// that are NOT part of {{TOKEN}} are translated. Unknown tokens stay as-is.
func translateUISingleBrackets(template string) string {
	var result strings.Builder
	i := 0
	for i < len(template) {
		if template[i] == '{' && (i == 0 || template[i-1] != '{') {
			j := i + 1
			for j < len(template) && template[j] != '}' {
				j++
			}
			if j < len(template) && template[j] == '}' {
				token := template[i+1 : j]
				if internal, ok := uiTokenMap[token]; ok {
					result.WriteString("{{")
					result.WriteString(internal)
					result.WriteString("}}")
					i = j + 1
					continue
				}
			}
		}
		result.WriteByte(template[i])
		i++
	}
	return result.String()
}

// expandRemarkVars substitutes every {{TOKEN}} in template with its per-client
// value. Unknown tokens resolve to "" (never the literal text). The template is
// split on "|" into segments: a segment whose only value is an unlimited quota
// or expiry (∞) drops out whole — decoration and separator included — so an
// unlimited client gets "host" instead of "host|📊∞|⏳∞D".
func expandRemarkVars(template string, ctx remarkContext) string {
	template = translateUISingleBrackets(template)
	if !strings.Contains(template, "{{") {
		return template
	}
	segments := strings.Split(template, "|")
	kept := make([]string, 0, len(segments))
	for _, seg := range segments {
		if out, drop := expandSegment(seg, ctx); !drop {
			kept = append(kept, out)
		}
	}
	return strings.Join(kept, "|")
}

// expandSegment expands one "|" segment and reports whether it should be dropped.
// It drops only when the segment carries an unlimited (∞) quota/expiry token and
// no other token in it resolves to a non-empty value — so a segment mixing, say,
// {{EMAIL}} with {{TRAFFIC_LEFT}} is always kept.
func expandSegment(seg string, ctx remarkContext) (string, bool) {
	hasUnlimited, hasOtherValue := false, false
	out := remarkVarRe.ReplaceAllStringFunc(seg, func(m string) string {
		token := m[2 : len(m)-2]
		val := remarkVarValue(token, ctx)
		switch {
		case unlimitedDropTokens[token] && val == unlimitedMark:
			hasUnlimited = true
		case val != "":
			hasOtherValue = true
		}
		return val
	})
	return out, hasUnlimited && !hasOtherValue
}

func remarkVarValue(token string, ctx remarkContext) string {
	c := ctx.client
	st := ctx.stats
	used := st.Up + st.Down
	switch token {
	case "EMAIL", "USERNAME":
		return c.Email
	case "INBOUND":
		return ctx.configName()
	case "HOST":
		return ctx.hostRemark
	case "ID":
		return c.ID
	case "SHORT_ID":
		if len(c.ID) >= 8 {
			return c.ID[:8]
		}
		return c.ID
	case "TELEGRAM_ID":
		if c.TgID != 0 {
			return strconv.FormatInt(c.TgID, 10)
		}
		return ""
	case "SUB_ID":
		return c.SubID
	case "COMMENT":
		return c.Comment
	case "STATUS":
		return clientStatus(st)
	case "DAYS_LEFT":
		return daysLeftLabel(st.ExpiryTime)
	case "EXPIRE_DATE":
		return expireDateLabel(st.ExpiryTime)
	case "EXPIRE_UNIX":
		if st.ExpiryTime <= 0 {
			return "0"
		}
		return strconv.FormatInt(st.ExpiryTime/1000, 10)
	case "CREATED_UNIX":
		if c.CreatedAt == 0 {
			return ""
		}
		return strconv.FormatInt(c.CreatedAt/1000, 10)
	case "TRAFFIC_USED":
		return common.FormatTraffic(used)
	case "TRAFFIC_LEFT":
		if st.Total <= 0 {
			return unlimitedMark
		}
		return common.FormatTraffic(max64(st.Total-used, 0))
	case "TRAFFIC_TOTAL":
		if st.Total <= 0 {
			return unlimitedMark
		}
		return common.FormatTraffic(st.Total)
	case "TRAFFIC_USED_BYTES":
		return strconv.FormatInt(used, 10)
	case "TRAFFIC_LEFT_BYTES":
		if st.Total <= 0 {
			return "0"
		}
		return strconv.FormatInt(max64(st.Total-used, 0), 10)
	case "TRAFFIC_TOTAL_BYTES":
		return strconv.FormatInt(st.Total, 10)
	case "UP":
		return common.FormatTraffic(st.Up)
	case "DOWN":
		return common.FormatTraffic(st.Down)
	case "RESET_DAYS":
		if c.Reset > 0 {
			return strconv.Itoa(c.Reset)
		}
		return ""
	case "STATUS_EMOJI":
		return statusEmoji(st)
	case "USAGE_PERCENTAGE":
		return usagePercentage(st)
	case "PROTOCOL":
		if ctx.inbound != nil {
			return strings.ToUpper(string(ctx.inbound.Protocol))
		}
		return ""
	case "TRANSPORT":
		return ctx.transport
	case "TIME_LEFT":
		return timeLeftLabel(st.ExpiryTime)
	case "JALALI_EXPIRE_DATE":
		return jalaliExpireDateLabel(st.ExpiryTime)
	}
	return ""
}

// clientStatus collapses enable/expiry/quota into a single word.
func clientStatus(st xray.ClientTraffic) string {
	if !st.Enable {
		return "disabled"
	}
	if st.ExpiryTime > 0 && st.ExpiryTime/1000 < time.Now().Unix() {
		return "expired"
	}
	if st.Total > 0 && st.Up+st.Down >= st.Total {
		return "depleted"
	}
	return "active"
}

// daysLeftLabel is the whole-days form of remainingTimeLabel: "∞" for unlimited,
// "0" once past expiry.
func daysLeftLabel(expiryMs int64) string {
	if expiryMs == 0 {
		return unlimitedMark
	}
	exp := expiryMs / 1000
	var secs int64
	if exp > 0 {
		secs = exp - time.Now().Unix()
	} else {
		secs = -exp // delayed-start: value is the duration itself
	}
	days := secs / 86400
	if days < 0 {
		return "0"
	}
	return strconv.FormatInt(days, 10)
}

// expireDateLabel renders a fixed expiry as YYYY-MM-DD (local time). Unlimited
// and delayed-start (no fixed calendar date yet) expiries yield "".
func expireDateLabel(expiryMs int64) string {
	if expiryMs <= 0 {
		return ""
	}
	return time.Unix(expiryMs/1000, 0).In(time.Local).Format("2006-01-02")
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// statusEmoji maps clientStatus to a single emoji character.
func statusEmoji(st xray.ClientTraffic) string {
	switch clientStatus(st) {
	case "active":
		return "✅"
	case "expired":
		return "⏳"
	case "depleted":
		return "🚫"
	case "disabled":
		return "🚫"
	default:
		return ""
	}
}

// usagePercentage computes the traffic usage as a percentage string (e.g. "52.3%").
// Returns "" when the client has no traffic limit.
func usagePercentage(st xray.ClientTraffic) string {
	if st.Total <= 0 {
		return ""
	}
	used := st.Up + st.Down
	pct := float64(used) / float64(st.Total) * 100
	if pct > 100 {
		pct = 100 // clamp over-quota usage, consistent with TRAFFIC_LEFT
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// timeLeftLabel renders remaining time as "Xd Xh Xm" (or shorter when days/hours
// are zero). Returns "∞" for unlimited and "0" when past expiry.
func timeLeftLabel(expiryMs int64) string {
	if expiryMs == 0 {
		return unlimitedMark
	}
	exp := expiryMs / 1000
	var secs int64
	if exp > 0 {
		secs = exp - time.Now().Unix()
	} else {
		secs = -exp
	}
	if secs <= 0 {
		return "0"
	}
	days := secs / 86400
	hours := (secs % 86400) / 3600
	mins := (secs % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// jalaliExpireDateLabel converts a Gregorian expiry timestamp to Jalali
// (Persian/Solar Hijri) date format "YYYY/MM/DD". Returns "" for unlimited
// or delayed-start expiries.
func jalaliExpireDateLabel(expiryMs int64) string {
	if expiryMs <= 0 {
		return ""
	}
	t := time.Unix(expiryMs/1000, 0).In(time.Local)
	y, m, d := gregorianToJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%d/%02d/%02d", y, m, d)
}

// gregorianToJalali converts a Gregorian date to Jalali (Solar Hijri) date.
// Uses a reference-date approach: counts days from a known reference point
// (2024-01-01 = 1402-10-11 JAL) and walks the Jalali calendar forward/backward.
func gregorianToJalali(gy, gm, gd int) (jy, jm, jd int) {
	// Compute Julian Day Number for the input Gregorian date
	a := (14 - gm) / 12
	y := gy + 4800 - a
	m := gm + 12*a - 3
	jdn := gd + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045

	// Reference: 2024-01-01 = JDN 2460311 = 1402-10-11 JAL
	refJDN := 2460311
	days := int64(jdn - refJDN)
	jy, jm, jd = 1402, 10, 11

	// Walk forward
	for days > 0 {
		remaining := int64(jalaliMonthDays(jy, jm) - jd + 1)
		if days < remaining {
			jd += int(days)
			return
		}
		days -= remaining
		jm++
		if jm > 12 {
			jm = 1
			jy++
		}
		jd = 1
	}
	// Walk backward
	for days < 0 {
		jd += int(days)
		for jd < 1 {
			jm--
			if jm < 1 {
				jm = 12
				jy--
			}
			jd += jalaliMonthDays(jy, jm)
		}
		days = 0
	}
	return
}

func jalaliMonthDays(y, m int) int {
	if m <= 6 {
		return 31
	}
	if m <= 11 {
		return 30
	}
	if isJalaliLeap(y) {
		return 30
	}
	return 29
}

// isJalaliLeap reports whether the given Jalali year is a leap year.
// The leap pattern repeats every 33 years with 8 leap years.
func isJalaliLeap(y int) bool {
	switch y % 33 {
	case 1, 5, 9, 13, 17, 22, 26, 30:
		return true
	}
	return false
}

// statsForClient returns the client's live traffic record, or a minimal one
// synthesized from the client (enable/expiry/total) when no live stats exist —
// so expiry/total/status tokens still resolve on links that have no counters yet.
func (s *SubService) statsForClient(inbound *model.Inbound, client model.Client) xray.ClientTraffic {
	if stats, ok := s.findClientStats(inbound, client.Email); ok {
		return stats
	}
	// client_traffics.email is globally unique, so a client shared across several
	// inbounds of one subscription has a single traffic row owned by exactly one
	// inbound. On every other inbound's link findClientStats misses; fall back to
	// the per-request map built from all the subscription's inbounds so
	// {{TRAFFIC_*}} reflect real usage instead of the full quota (#5443).
	if stats, ok := s.statsByEmail[client.Email]; ok {
		return stats
	}
	return xray.ClientTraffic{
		Enable:     client.Enable,
		ExpiryTime: client.ExpiryTime,
		Total:      client.TotalGB,
	}
}

// lookupClient resolves the full client (TgID, SubID, comment, …) for an email,
// needed when a global remark template references client-only tokens. Falls back
// to an email-only client if not found.
func (s *SubService) lookupClient(inbound *model.Inbound, email string) model.Client {
	clients, _ := s.inboundService.GetClients(inbound)
	for _, c := range clients {
		if c.Email == email {
			return c
		}
	}
	return model.Client{Email: email}
}

// usageInfoTokens are the per-client status tokens. On every link of a
// subscription except the client's first, these (and the decoration leading
// into them) are dropped, so the traffic/expiry info shows once instead of on
// every server.
var usageInfoTokens = []string{
	"TRAFFIC_USED", "TRAFFIC_LEFT", "TRAFFIC_TOTAL",
	"TRAFFIC_USED_BYTES", "TRAFFIC_LEFT_BYTES", "TRAFFIC_TOTAL_BYTES",
	"UP", "DOWN", "DAYS_LEFT", "EXPIRE_DATE", "EXPIRE_UNIX", "STATUS",
	"STATUS_EMOJI", "USAGE_PERCENTAGE", "TIME_LEFT", "JALALI_EXPIRE_DATE",
}

// nameOnlyTemplate returns template with the trailing per-client info part
// removed: everything from the first usage token (and the decoration — emojis,
// spaces, separators — leading into it) onward is dropped, leaving the config
// name. Returns "" when the template is info-only.
func nameOnlyTemplate(template string) string {
	idx := -1
	for _, tok := range usageInfoTokens {
		if i := strings.Index(template, "{{"+tok+"}}"); i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}
	if idx < 0 {
		return template
	}
	return strings.TrimRightFunc(template[:idx], func(r rune) bool {
		return r != '}' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// effectiveTemplate picks which template to expand for one body link: the full
// template (with the per-client info) for a client's first link, and the
// name-only template for every link thereafter — so the info shows once. Only
// called in the subscription-body context (displays bypass the template).
func (s *SubService) effectiveTemplate(email string) string {
	translated := translateUISingleBrackets(s.remarkTemplate)
	if s.usageShown == nil {
		s.usageShown = map[string]bool{}
	}
	if s.usageShown[email] {
		return nameOnlyTemplate(translated)
	}
	s.usageShown[email] = true
	return translated
}

// genTemplatedRemark expands the remark template for one client. hostRemark is
// the host endpoint's remark (empty for a plain inbound); it backs the {{HOST}}
// token only and never substitutes the inbound remark as the config name.
func (s *SubService) genTemplatedRemark(inbound *model.Inbound, client model.Client, hostRemark string, transport string) string {
	ctx := remarkContext{
		client:     client,
		stats:      s.statsForClient(inbound, client),
		inbound:    inbound,
		hostRemark: hostRemark,
		transport:  transport,
	}
	tmpl := s.effectiveTemplate(client.Email)
	// Fall back to the config name when the template is empty or expands to
	// nothing (e.g. an all-unlimited template whose only segments dropped out).
	if out := expandRemarkVars(tmpl, ctx); strings.TrimSpace(out) != "" {
		return out
	}
	return ctx.configName()
}

// genHostRemark builds one host endpoint's remark for a specific client. The
// config name is always the inbound's own remark; the host's remark is surfaced
// only through the {{HOST}} token. In the subscription body the rest of the
// remark template still applies; displays show just the config name.
func (s *SubService) genHostRemark(inbound *model.Inbound, client model.Client, hostRemark string, transport string) string {
	if !s.subscriptionBody {
		return remarkContext{inbound: inbound, hostRemark: hostRemark}.configName()
	}
	return s.genTemplatedRemark(inbound, client, hostRemark, transport)
}
