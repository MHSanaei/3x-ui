package mtproto

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

// Options are the mtg-multi knobs an MTProto inbound exposes beyond the core
// ones Instance carries (listener, fronting, routing, secured, warm pool).
// They live in the inbound settings under the JSON keys below. A zero value
// means "not set": the key is left out of the generated config and mtg uses
// its own default, so an inbound that sets none of them renders the exact
// config it always did.
type Options struct {
	Concurrency              int    `json:"concurrency,omitempty"`
	TolerateTimeSkewness     string `json:"tolerateTimeSkewness,omitempty"`
	AllowFallbackOnUnknownDC bool   `json:"allowFallbackOnUnknownDc,omitempty"`
	AutoUpdate               bool   `json:"autoUpdate,omitempty"`
	ThrottleCheckInterval    string `json:"throttleCheckInterval,omitempty"`

	Network NetworkOptions `json:"network"`
	Defense DefenseOptions `json:"defense"`
	DCPool  DCPoolOptions  `json:"dcPool"`
	Stats   StatsOptions   `json:"stats"`
	Web     WebOptions     `json:"web"`

	// ExtraTOML is free-form mtg config merged under the generated one: a key
	// the panel already writes wins, and the keys that carry the client set or
	// the panel's own API access (see forbiddenExtraKeys) are dropped.
	ExtraTOML string `json:"extraToml,omitempty"`
}

type NetworkOptions struct {
	DNS             string           `json:"dns,omitempty"`
	Proxies         []string         `json:"proxies,omitempty"`
	TCPNotSentLowat string           `json:"tcpNotSentLowat,omitempty"`
	Timeout         TimeoutOptions   `json:"timeout"`
	KeepAlive       KeepAliveOptions `json:"keepAlive"`
}

type TimeoutOptions struct {
	TCP       string `json:"tcp,omitempty"`
	HTTP      string `json:"http,omitempty"`
	Idle      string `json:"idle,omitempty"`
	Handshake string `json:"handshake,omitempty"`
}

type KeepAliveOptions struct {
	Disabled bool   `json:"disabled,omitempty"`
	Idle     string `json:"idle,omitempty"`
	Interval string `json:"interval,omitempty"`
	Count    int    `json:"count,omitempty"`
}

type DefenseOptions struct {
	AntiReplay        AntiReplayOptions        `json:"antiReplay"`
	Blocklist         IPListOptions            `json:"blocklist"`
	Allowlist         IPListOptions            `json:"allowlist"`
	Doppelganger      DoppelgangerOptions      `json:"doppelganger"`
	PendingHandshakes PendingHandshakesOptions `json:"pendingHandshakes"`
}

type AntiReplayOptions struct {
	Enabled   bool    `json:"enabled,omitempty"`
	MaxSize   string  `json:"maxSize,omitempty"`
	ErrorRate float64 `json:"errorRate,omitempty"`
}

type IPListOptions struct {
	Enabled             bool     `json:"enabled,omitempty"`
	URLs                []string `json:"urls,omitempty"`
	UpdateEach          string   `json:"updateEach,omitempty"`
	DownloadConcurrency int      `json:"downloadConcurrency,omitempty"`
}

type DoppelgangerOptions struct {
	URLs           []string `json:"urls,omitempty"`
	RepeatsPerRaid int      `json:"repeatsPerRaid,omitempty"`
	RaidEach       string   `json:"raidEach,omitempty"`
	DRS            bool     `json:"drs,omitempty"`
}

type PendingHandshakesOptions struct {
	MaxPerIP int  `json:"maxPerIp,omitempty"`
	DryRun   bool `json:"dryRun,omitempty"`
}

// DCPoolOptions holds the pool knobs beyond enabled/size, which Instance
// carries directly for compatibility with the first version of the settings.
type DCPoolOptions struct {
	DCs []int `json:"dcs,omitempty"`
}

type StatsOptions struct {
	Prometheus PrometheusOptions `json:"prometheus"`
	StatsD     StatsDOptions     `json:"statsd"`
}

type PrometheusOptions struct {
	Enabled      bool   `json:"enabled,omitempty"`
	BindTo       string `json:"bindTo,omitempty"`
	HTTPPath     string `json:"httpPath,omitempty"`
	MetricPrefix string `json:"metricPrefix,omitempty"`
}

type StatsDOptions struct {
	Enabled      bool   `json:"enabled,omitempty"`
	Address      string `json:"address,omitempty"`
	MetricPrefix string `json:"metricPrefix,omitempty"`
	TagFormat    string `json:"tagFormat,omitempty"`
}

type WebOptions struct {
	BindTo         string   `json:"bindTo,omitempty"`
	Host           string   `json:"host,omitempty"`
	SecretMode     string   `json:"secretMode,omitempty"`
	DecoyDir       string   `json:"decoyDir,omitempty"`
	TrustedProxies []string `json:"trustedProxies,omitempty"`
	MaxSessions    int      `json:"maxSessions,omitempty"`
	MaxPending     int      `json:"maxPending,omitempty"`
	Diag           bool     `json:"diag,omitempty"`
}

// Enabled reports whether the WEB listener is configured: mtg starts it only
// with a loopback bind address and the link domain.
func (w WebOptions) Enabled() bool {
	return w.BindTo != "" && w.Host != ""
}

// Limits mirror what mtg-multi accepts, so a value the panel lets through
// never makes mtg reject the whole config.
const (
	// maxUint16 is the upper bound of mtg's TypeConcurrency (16-bit, > 0).
	maxUint16 = math.MaxUint16
	// DCPoolMaxDCs caps the DC list of the warm pool (mtg-multi DCPoolMaxDCs).
	DCPoolMaxDCs = 32
	// maxListEntries caps URL, proxy and CIDR lists so one inbound cannot make
	// mtg fan out to an unbounded number of downloads or dialers.
	maxListEntries = 32
	// MaxExtraTOMLBytes caps the free-form config block.
	MaxExtraTOMLBytes = 64 << 10
)

// forbiddenExtraKeys are top-level keys the extra TOML may not set: the client
// set and its limits are owned by the panel (and hot-reloaded through the API),
// and the listener and API endpoint/token are what the panel uses to manage
// the process.
var forbiddenExtraKeys = []string{
	"secret", "secrets", "secret-limits", "secret-ad-tags",
	"bind-to", "api-bind-to", "api-token",
}

var (
	metricPrefixRe = regexp.MustCompile(`^[a-z0-9]+$`)
	bytesRe        = regexp.MustCompile(`(?i)^[0-9]+\s*(b|kb|kib|mb|mib|gb|gib|tb|tib)$`)
	hostnameRe     = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

// optionCheck is one validation rule: when bad, ValidateSettings fails with
// path and msg, while sanitize clears the offending value with reset.
type optionCheck struct {
	path  string
	msg   string
	bad   bool
	reset func(*Options)
}

// checks lists every rule for o. routeThroughXray is the inbound's existing
// switch: mtg's proxies list is then owned by the Xray bridge.
func (o *Options) checks(routeThroughXray bool) []optionCheck {
	n, d, s, w := &o.Network, &o.Defense, &o.Stats, &o.Web
	c := []optionCheck{
		{"concurrency", fmt.Sprintf("must be between 1 and %d", maxUint16), !validCount(o.Concurrency), func(o *Options) { o.Concurrency = 0 }},
		{"tolerateTimeSkewness", "must be a positive duration like 5s", !validDuration(o.TolerateTimeSkewness), func(o *Options) { o.TolerateTimeSkewness = "" }},
		{"throttleCheckInterval", "must be a positive duration like 5s", !validDuration(o.ThrottleCheckInterval), func(o *Options) { o.ThrottleCheckInterval = "" }},

		{"network.dns", "must be an IP, udp://IP, tls://host or https:// URL", !validDNS(n.DNS), func(o *Options) { o.Network.DNS = "" }},
		{"network.proxies", fmt.Sprintf("must be at most %d socks5:// URLs", maxListEntries), !validList(n.Proxies, validProxyURL), func(o *Options) { o.Network.Proxies = nil }},
		{"network.proxies", "cannot be combined with routing through Xray", routeThroughXray && len(n.Proxies) > 0, func(o *Options) { o.Network.Proxies = nil }},
		{"network.tcpNotSentLowat", "must be a size like 128kib", !validBytes(n.TCPNotSentLowat), func(o *Options) { o.Network.TCPNotSentLowat = "" }},
		{"network.timeout.tcp", "must be a positive duration", !validDuration(n.Timeout.TCP), func(o *Options) { o.Network.Timeout.TCP = "" }},
		{"network.timeout.http", "must be a positive duration", !validDuration(n.Timeout.HTTP), func(o *Options) { o.Network.Timeout.HTTP = "" }},
		{"network.timeout.idle", "must be a positive duration", !validDuration(n.Timeout.Idle), func(o *Options) { o.Network.Timeout.Idle = "" }},
		{"network.timeout.handshake", "must be a positive duration", !validDuration(n.Timeout.Handshake), func(o *Options) { o.Network.Timeout.Handshake = "" }},
		{"network.keepAlive.idle", "must be a positive duration", !validDuration(n.KeepAlive.Idle), func(o *Options) { o.Network.KeepAlive.Idle = "" }},
		{"network.keepAlive.interval", "must be a positive duration", !validDuration(n.KeepAlive.Interval), func(o *Options) { o.Network.KeepAlive.Interval = "" }},
		{"network.keepAlive.count", fmt.Sprintf("must be between 1 and %d", maxUint16), !validCount(n.KeepAlive.Count), func(o *Options) { o.Network.KeepAlive.Count = 0 }},

		{"defense.antiReplay.maxSize", "must be a size like 1mib", !validBytes(d.AntiReplay.MaxSize), func(o *Options) { o.Defense.AntiReplay.MaxSize = "" }},
		{"defense.antiReplay.errorRate", "must be greater than 0 and less than 100", d.AntiReplay.ErrorRate < 0 || d.AntiReplay.ErrorRate >= 100, func(o *Options) { o.Defense.AntiReplay.ErrorRate = 0 }},
	}
	c = append(c, ipListChecks("blocklist", d.Blocklist, func(o *Options) *IPListOptions { return &o.Defense.Blocklist })...)
	c = append(c, ipListChecks("allowlist", d.Allowlist, func(o *Options) *IPListOptions { return &o.Defense.Allowlist })...)
	c = append(c,
		optionCheck{"defense.doppelganger.urls", fmt.Sprintf("must be at most %d https:// URLs", maxListEntries), !validList(d.Doppelganger.URLs, validHTTPSURL), func(o *Options) { o.Defense.Doppelganger.URLs = nil }},
		optionCheck{"defense.doppelganger.repeatsPerRaid", fmt.Sprintf("must be between 1 and %d", maxUint16), !validCount(d.Doppelganger.RepeatsPerRaid), func(o *Options) { o.Defense.Doppelganger.RepeatsPerRaid = 0 }},
		optionCheck{"defense.doppelganger.raidEach", "must be a positive duration", !validDuration(d.Doppelganger.RaidEach), func(o *Options) { o.Defense.Doppelganger.RaidEach = "" }},
		optionCheck{"defense.pendingHandshakes.maxPerIp", fmt.Sprintf("must be between 0 and %d", maxUint16), d.PendingHandshakes.MaxPerIP < 0 || d.PendingHandshakes.MaxPerIP > maxUint16, func(o *Options) { o.Defense.PendingHandshakes.MaxPerIP = 0 }},

		optionCheck{"dcPool.dcs", fmt.Sprintf("must hold at most %d distinct non-zero DC ids between -32768 and 32767", DCPoolMaxDCs), !validDCs(o.DCPool.DCs), func(o *Options) { o.DCPool.DCs = nil }},

		optionCheck{"stats.prometheus.bindTo", "must be a loopback IP:port like 127.0.0.1:3129", s.Prometheus.BindTo != "" && !validLoopbackHostPort(s.Prometheus.BindTo), func(o *Options) { o.Stats.Prometheus = PrometheusOptions{} }},
		optionCheck{"stats.prometheus.bindTo", "is required when Prometheus is enabled", s.Prometheus.Enabled && s.Prometheus.BindTo == "", func(o *Options) { o.Stats.Prometheus.Enabled = false }},
		optionCheck{"stats.prometheus.httpPath", "must be a URL path like /metrics", !validHTTPPath(s.Prometheus.HTTPPath), func(o *Options) { o.Stats.Prometheus.HTTPPath = "" }},
		optionCheck{"stats.prometheus.metricPrefix", "may contain only a-z and 0-9", s.Prometheus.MetricPrefix != "" && !metricPrefixRe.MatchString(s.Prometheus.MetricPrefix), func(o *Options) { o.Stats.Prometheus.MetricPrefix = "" }},
		optionCheck{"stats.statsd.address", "must be an IP:port", s.StatsD.Address != "" && !validHostPort(s.StatsD.Address), func(o *Options) { o.Stats.StatsD = StatsDOptions{} }},
		optionCheck{"stats.statsd.address", "is required when StatsD is enabled", s.StatsD.Enabled && s.StatsD.Address == "", func(o *Options) { o.Stats.StatsD.Enabled = false }},
		optionCheck{"stats.statsd.metricPrefix", "may contain only a-z and 0-9", s.StatsD.MetricPrefix != "" && !metricPrefixRe.MatchString(s.StatsD.MetricPrefix), func(o *Options) { o.Stats.StatsD.MetricPrefix = "" }},
		optionCheck{"stats.statsd.tagFormat", "must be datadog, influxdb or graphite", !slices.Contains([]string{"", "datadog", "influxdb", "graphite"}, s.StatsD.TagFormat), func(o *Options) { o.Stats.StatsD.TagFormat = "" }},

		optionCheck{"web.bindTo", "must be a loopback IP:port like 127.0.0.1:18080 (TLS is terminated by a reverse proxy)", w.BindTo != "" && !validLoopbackHostPort(w.BindTo), func(o *Options) { o.Web = WebOptions{} }},
		optionCheck{"web.host", "must be a domain name like proxy.example.com", w.Host != "" && !hostnameRe.MatchString(w.Host), func(o *Options) { o.Web = WebOptions{} }},
		optionCheck{"web.host", "is required when the WEB listener is set", w.BindTo != "" && w.Host == "", func(o *Options) { o.Web = WebOptions{} }},
		optionCheck{"web.secretMode", "must be dd or plain", !slices.Contains([]string{"", "dd", "plain"}, w.SecretMode), func(o *Options) { o.Web.SecretMode = "" }},
		optionCheck{"web.decoyDir", "must be an absolute directory path", w.DecoyDir != "" && !validAbsPath(w.DecoyDir), func(o *Options) { o.Web.DecoyDir = "" }},
		optionCheck{"web.trustedProxies", fmt.Sprintf("must be at most %d CIDRs like 127.0.0.1/32", maxListEntries), !validList(w.TrustedProxies, validCIDR), func(o *Options) { o.Web.TrustedProxies = nil }},
		optionCheck{"web.maxSessions", fmt.Sprintf("must be between 1 and %d", maxUint16), !validCount(w.MaxSessions), func(o *Options) { o.Web.MaxSessions = 0 }},
		optionCheck{"web.maxPending", fmt.Sprintf("must be between 1 and %d", maxUint16), !validCount(w.MaxPending), func(o *Options) { o.Web.MaxPending = 0 }},
	)
	if err := checkExtraTOML(o.ExtraTOML); err != nil {
		c = append(c, optionCheck{"extraToml", err.Error(), true, func(o *Options) { o.ExtraTOML = "" }})
	}
	return c
}

func ipListChecks(name string, l IPListOptions, field func(*Options) *IPListOptions) []optionCheck {
	p := "defense." + name
	return []optionCheck{
		{p + ".urls", fmt.Sprintf("must be at most %d http(s):// URLs", maxListEntries), !validList(l.URLs, validListURL), func(o *Options) { *field(o) = IPListOptions{} }},
		{p + ".urls", "needs at least one URL when the list is enabled", l.Enabled && len(l.URLs) == 0, func(o *Options) { field(o).Enabled = false }},
		{p + ".updateEach", "must be a positive duration", !validDuration(l.UpdateEach), func(o *Options) { field(o).UpdateEach = "" }},
		{p + ".downloadConcurrency", fmt.Sprintf("must be between 1 and %d", maxUint16), !validCount(l.DownloadConcurrency), func(o *Options) { field(o).DownloadConcurrency = 0 }},
	}
}

// Validate returns the first rule o breaks, phrased for the panel user.
func (o Options) Validate(routeThroughXray bool) error {
	for _, c := range o.checks(routeThroughXray) {
		if c.bad {
			return fmt.Errorf("mtproto %s %s", c.path, c.msg)
		}
	}
	return nil
}

// sanitized drops every value that breaks a rule. Save paths validate, but
// settings can also arrive from raw API payloads or older data, and one bad
// value in a generated config makes mtg refuse to start — taking every client
// of the inbound down with it.
func (o Options) sanitized(routeThroughXray bool) Options {
	for range 4 { // a reset can expose another rule (e.g. cleared URLs vs enabled)
		clean := true
		for _, c := range o.checks(routeThroughXray) {
			if c.bad {
				c.reset(&o)
				clean = false
			}
		}
		if clean {
			break
		}
	}
	return o
}

// fingerprint identifies the options for the structural fingerprint: any
// change needs an mtg restart.
func (o Options) fingerprint() string {
	b, err := json.Marshal(o)
	if err != nil {
		return ""
	}
	return string(b)
}

// ParseOptions reads the options from raw inbound settings. A value of the
// wrong JSON type is skipped (encoding/json keeps decoding the rest).
func ParseOptions(settings string) Options {
	var o Options
	_ = json.Unmarshal([]byte(settings), &o)
	return o
}

// WebEndpoint returns the WEB link domain and key mode of mtproto inbound
// settings when the WEB listener is configured (and valid, so the link is only
// offered when the generated config actually serves it).
func WebEndpoint(settings string) (host, secretMode string, ok bool) {
	var raw struct {
		RouteThroughXray bool `json:"routeThroughXray"`
	}
	_ = json.Unmarshal([]byte(settings), &raw)
	web := ParseOptions(settings).sanitized(raw.RouteThroughXray).Web
	if !web.Enabled() {
		return "", "", false
	}
	if web.SecretMode == "" {
		web.SecretMode = "dd"
	}
	return web.Host, web.SecretMode, true
}

// ValidateSettings checks the mtg options of mtproto inbound settings on save,
// so a value mtg would reject never reaches the generated config.
func ValidateSettings(settings string) error {
	var raw struct {
		RouteThroughXray bool `json:"routeThroughXray"`
		DCPool           *struct {
			Size int `json:"size"`
		} `json:"dcPool"`
	}
	if err := json.Unmarshal([]byte(settings), &raw); err != nil {
		return nil // malformed JSON is rejected by the generic settings checks
	}
	if raw.DCPool != nil && (raw.DCPool.Size < 0 || raw.DCPool.Size > DCPoolMaxSize) {
		return fmt.Errorf("mtproto dcPool.size must be between 1 and %d", DCPoolMaxSize)
	}
	var o Options
	if err := json.Unmarshal([]byte(settings), &o); err != nil {
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			return fmt.Errorf("mtproto %s has the wrong type", typeErr.Field)
		}
		return fmt.Errorf("mtproto settings: %w", err)
	}
	return o.Validate(raw.RouteThroughXray)
}

func validCount(v int) bool { return v >= 0 && v <= maxUint16 }

func validDuration(v string) bool {
	if v == "" {
		return true
	}
	d, err := time.ParseDuration(strings.ToLower(strings.TrimSpace(v)))
	return err == nil && d > 0 && v == strings.TrimSpace(v)
}

func validBytes(v string) bool {
	return v == "" || bytesRe.MatchString(v)
}

func validList(items []string, valid func(string) bool) bool {
	if len(items) > maxListEntries {
		return false
	}
	for _, it := range items {
		if !valid(it) {
			return false
		}
	}
	return true
}

func safeText(v string) bool {
	if v == "" || !utf8.ValidString(v) || v != strings.TrimSpace(v) {
		return false
	}
	return !strings.ContainsFunc(v, unicode.IsControl)
}

func parsedURL(v string, schemes ...string) (*url.URL, bool) {
	if !safeText(v) {
		return nil, false
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || !slices.Contains(schemes, u.Scheme) {
		return nil, false
	}
	return u, true
}

func validProxyURL(v string) bool {
	_, ok := parsedURL(v, "socks5", "socks5h")
	return ok
}

func validHTTPSURL(v string) bool {
	_, ok := parsedURL(v, "https")
	return ok
}

// validListURL accepts remote FireHOL lists only. mtg also takes a local file,
// but it refuses to start when that file is missing, and the panel cannot see
// the filesystem of the node that runs the inbound.
func validListURL(v string) bool {
	_, ok := parsedURL(v, "http", "https")
	return ok
}

func validDNS(v string) bool {
	if v == "" {
		return true
	}
	if !safeText(v) {
		return false
	}
	if net.ParseIP(v) != nil {
		return true
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "tls":
		return u.Path == ""
	case "udp":
		return u.Path == "" && net.ParseIP(u.Hostname()) != nil
	}
	return false
}

func validHostPort(v string) bool {
	host, port, err := net.SplitHostPort(v)
	if err != nil || net.ParseIP(host) == nil {
		return false
	}
	p, err := strconv.Atoi(port)
	return err == nil && p >= 1 && p <= 65535
}

func validLoopbackHostPort(v string) bool {
	if !validHostPort(v) {
		return false
	}
	host, _, _ := net.SplitHostPort(v)
	return net.ParseIP(host).IsLoopback()
}

func validHTTPPath(v string) bool {
	if v == "" {
		return true
	}
	return safeText(v) && strings.HasPrefix(v, "/") && !strings.ContainsAny(v, " ?#")
}

func validAbsPath(v string) bool {
	return safeText(v) && path.IsAbs(v)
}

func validCIDR(v string) bool {
	_, _, err := net.ParseCIDR(v)
	return err == nil
}

func validDCs(dcs []int) bool {
	if len(dcs) > DCPoolMaxDCs {
		return false
	}
	seen := make(map[int]struct{}, len(dcs))
	for _, dc := range dcs {
		if dc == 0 || dc < math.MinInt16 || dc > math.MaxInt16 {
			return false
		}
		if _, dup := seen[dc]; dup {
			return false
		}
		seen[dc] = struct{}{}
	}
	return true
}

// checkExtraTOML accepts an empty block or a TOML document that does not set
// any key the panel owns.
func checkExtraTOML(extra string) error {
	if strings.TrimSpace(extra) == "" {
		return nil
	}
	if len(extra) > MaxExtraTOMLBytes {
		return fmt.Errorf("must be at most %d bytes", MaxExtraTOMLBytes)
	}
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(extra), &doc); err != nil {
		return fmt.Errorf("is not valid TOML: %s", strings.ReplaceAll(err.Error(), "\n", " "))
	}
	for _, k := range forbiddenExtraKeys {
		if _, ok := doc[k]; ok {
			return fmt.Errorf("may not set %q: the panel manages it", k)
		}
	}
	return nil
}

// tomlQuote renders a basic TOML string. Values reaching it are validated to
// be printable UTF-8, for which Go's quoting is valid TOML.
func tomlQuote(v string) string {
	return strconv.Quote(v)
}

func tomlStringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = tomlQuote(it)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func tomlIntArray(items []int) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = strconv.Itoa(it)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// sectionWriter emits a [header] lazily, right before its first key, so a
// section whose keys are all unset is left out entirely.
type sectionWriter struct {
	b      *strings.Builder
	header string
	opened bool
}

func (s *sectionWriter) line(format string, args ...any) {
	if !s.opened {
		fmt.Fprintf(s.b, "\n[%s]\n", s.header)
		s.opened = true
	}
	fmt.Fprintf(s.b, format+"\n", args...)
}

func (s *sectionWriter) str(key, v string) {
	if v != "" {
		s.line("%s = %s", key, tomlQuote(v))
	}
}

func (s *sectionWriter) num(key string, v int) {
	if v != 0 {
		s.line("%s = %d", key, v)
	}
}

func (s *sectionWriter) flag(key string, v bool) {
	if v {
		s.line("%s = true", key)
	}
}

func (s *sectionWriter) strs(key string, v []string) {
	if len(v) > 0 {
		s.line("%s = %s", key, tomlStringArray(v))
	}
}

// writeTopLevel emits the top-level option keys; they must precede every
// [section] header.
func (o Options) writeTopLevel(b *strings.Builder) {
	if o.Concurrency > 0 {
		fmt.Fprintf(b, "concurrency = %d\n", o.Concurrency)
	}
	if o.TolerateTimeSkewness != "" {
		fmt.Fprintf(b, "tolerate-time-skewness = %s\n", tomlQuote(o.TolerateTimeSkewness))
	}
	if o.AllowFallbackOnUnknownDC {
		b.WriteString("allow-fallback-on-unknown-dc = true\n")
	}
	if o.AutoUpdate {
		b.WriteString("auto-update = true\n")
	}
}

// writeNetwork emits [network] and its sub-tables. xrayProxy is the SOCKS
// bridge URL when the inbound routes through Xray; it replaces the user's
// proxies (validation forbids setting both).
func (o Options) writeNetwork(b *strings.Builder, xrayProxy string) {
	n := o.Network
	s := &sectionWriter{b: b, header: "network"}
	if xrayProxy != "" {
		s.strs("proxies", []string{xrayProxy})
	} else {
		s.strs("proxies", n.Proxies)
	}
	s.str("dns", n.DNS)
	s.str("tcp-not-sent-lowat", n.TCPNotSentLowat)

	t := &sectionWriter{b: b, header: "network.timeout"}
	t.str("tcp", n.Timeout.TCP)
	t.str("http", n.Timeout.HTTP)
	t.str("idle", n.Timeout.Idle)
	t.str("handshake", n.Timeout.Handshake)

	k := &sectionWriter{b: b, header: "network.keep-alive"}
	k.flag("disabled", n.KeepAlive.Disabled)
	k.str("idle", n.KeepAlive.Idle)
	k.str("interval", n.KeepAlive.Interval)
	k.num("count", n.KeepAlive.Count)
}

func writeIPList(b *strings.Builder, header string, l IPListOptions) {
	if !l.Enabled {
		return
	}
	s := &sectionWriter{b: b, header: header}
	s.flag("enabled", true)
	s.num("download-concurrency", l.DownloadConcurrency)
	s.strs("urls", l.URLs)
	s.str("update-each", l.UpdateEach)
}

// writeSections emits every option section after [dc-pool].
func (o Options) writeSections(b *strings.Builder) {
	d := o.Defense
	if d.AntiReplay.Enabled {
		a := &sectionWriter{b: b, header: "defense.anti-replay"}
		a.flag("enabled", true)
		a.str("max-size", d.AntiReplay.MaxSize)
		if d.AntiReplay.ErrorRate > 0 {
			a.line("error-rate = %s", strconv.FormatFloat(d.AntiReplay.ErrorRate, 'g', -1, 64))
		}
	}
	writeIPList(b, "defense.blocklist", d.Blocklist)
	writeIPList(b, "defense.allowlist", d.Allowlist)

	g := &sectionWriter{b: b, header: "defense.doppelganger"}
	g.strs("urls", d.Doppelganger.URLs)
	g.num("repeats-per-raid", d.Doppelganger.RepeatsPerRaid)
	g.str("raid-each", d.Doppelganger.RaidEach)
	g.flag("drs", d.Doppelganger.DRS)

	if d.PendingHandshakes.MaxPerIP > 0 {
		p := &sectionWriter{b: b, header: "defense.pending-handshakes"}
		p.num("max-per-ip", d.PendingHandshakes.MaxPerIP)
		p.flag("dry-run", d.PendingHandshakes.DryRun)
	}

	st := o.Stats
	if st.Prometheus.Enabled {
		pr := &sectionWriter{b: b, header: "stats.prometheus"}
		pr.flag("enabled", true)
		pr.str("bind-to", st.Prometheus.BindTo)
		pr.str("http-path", st.Prometheus.HTTPPath)
		pr.str("metric-prefix", st.Prometheus.MetricPrefix)
	}
	if st.StatsD.Enabled {
		sd := &sectionWriter{b: b, header: "stats.statsd"}
		sd.flag("enabled", true)
		sd.str("address", st.StatsD.Address)
		sd.str("metric-prefix", st.StatsD.MetricPrefix)
		sd.str("tag-format", st.StatsD.TagFormat)
	}

	if o.Web.Enabled() {
		w := &sectionWriter{b: b, header: "web"}
		w.str("bind-to", o.Web.BindTo)
		w.str("host", o.Web.Host)
		w.str("secret-mode", o.Web.SecretMode)
		w.str("decoy-dir", o.Web.DecoyDir)
		w.strs("trusted-proxies", o.Web.TrustedProxies)
		w.num("max-sessions", o.Web.MaxSessions)
		w.num("max-pending", o.Web.MaxPending)
		w.flag("diag", o.Web.Diag)
	}
}

// mergeExtraTOML merges the free-form block under the generated head (every
// section before the client ones): keys the panel wrote win, tables merge key
// by key, and forbidden keys are dropped. The result is re-encoded, so it is
// only used when the extra block actually adds something.
func mergeExtraTOML(head, extra string) (string, error) {
	if strings.TrimSpace(extra) == "" {
		return head, nil
	}
	add := map[string]any{}
	if err := toml.Unmarshal([]byte(extra), &add); err != nil {
		return head, err
	}
	for _, k := range forbiddenExtraKeys {
		delete(add, k)
	}
	if len(add) == 0 {
		return head, nil
	}
	base := map[string]any{}
	if err := toml.Unmarshal([]byte(head), &base); err != nil {
		return head, err
	}
	mergeUnder(base, add)
	out, err := toml.Marshal(base)
	if err != nil {
		return head, err
	}
	return string(out), nil
}

// mergeUnder copies keys from add into dst only where dst has none; when
// both hold a table the tables are merged recursively.
func mergeUnder(dst, add map[string]any) {
	for k, v := range add {
		cur, ok := dst[k]
		if !ok {
			dst[k] = v
			continue
		}
		curTable, curIsTable := cur.(map[string]any)
		addTable, addIsTable := v.(map[string]any)
		if curIsTable && addIsTable {
			mergeUnder(curTable, addTable)
		}
	}
}
