package mtproto

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Backend names the sidecar that serves an mtproto inbound.
type Backend string

const (
	BackendMtg    Backend = "mtg"
	BackendTelemt Backend = "telemt"
)

// ParseBackend maps the stored settings value to a Backend; anything but
// "telemt" is mtg, so inbounds saved before the field existed keep mtg.
func ParseBackend(v string) Backend {
	if strings.EqualFold(strings.TrimSpace(v), string(BackendTelemt)) {
		return BackendTelemt
	}
	return BackendMtg
}

// TelemtOptions are the telemt-only knobs of an mtproto inbound, stored under
// settings.telemt so they never collide with the mtg ones.
type TelemtOptions struct {
	MaxTCPConnsPerUser int    `json:"maxTcpConnsPerUser,omitempty"`
	ExtraTOML          string `json:"extraToml,omitempty"`
}

// telemtRuntime is what one telemt process is managed through: loopback API
// and metrics ports and the API bearer token. Zero values render placeholders.
type telemtRuntime struct {
	apiPort     int
	metricsPort int
	apiToken    string
}

const telemtUsersFile = "users.toml"

const (
	maxUint16 = math.MaxUint16
	// MaxExtraTOMLBytes caps the free-form telemt config block.
	MaxExtraTOMLBytes = 64 << 10
)

// usableTelemtOptions drops an out-of-range connection cap from raw API
// payloads, which telemt would otherwise refuse with the whole config.
func usableTelemtOptions(o TelemtOptions) TelemtOptions {
	if o.MaxTCPConnsPerUser < 0 || o.MaxTCPConnsPerUser > maxUint16 {
		o.MaxTCPConnsPerUser = 0
	}
	return o
}

// telemtUser is one panel client as telemt sees it: the 16-byte key of the
// client's ee secret plus the FakeTLS domain encoded after it.
type telemtUser struct {
	name        string
	key         string
	domain      string
	adTag       string
	expiresUnix int64
}

// splitFakeTLSSecret splits an ee/dd FakeTLS secret into its 32-hex key and the
// domain hex-encoded after it; telemt takes the key alone and the domain globally.
func splitFakeTLSSecret(secret string) (key, domain string, ok bool) {
	s := strings.ToLower(strings.TrimSpace(secret))
	if strings.HasPrefix(s, "ee") || strings.HasPrefix(s, "dd") {
		s = s[2:]
	}
	if len(s) < 32 {
		return "", "", false
	}
	key = s[:32]
	if _, err := hex.DecodeString(key); err != nil {
		return "", "", false
	}
	if raw, err := hex.DecodeString(s[32:]); err == nil {
		domain = string(raw)
	}
	return key, domain, true
}

// telemtUsers converts the served secrets to telemt users sorted by name, so
// the rendered users.toml is byte-stable for an unchanged client set.
func telemtUsers(inst Instance) []telemtUser {
	users := make([]telemtUser, 0, len(inst.Secrets))
	for _, e := range inst.Secrets {
		key, domain, ok := splitFakeTLSSecret(e.Secret)
		if !ok || !utf8.ValidString(e.Name) {
			continue
		}
		users = append(users, telemtUser{name: e.Name, key: key, domain: domain, adTag: e.AdTag, expiresUnix: e.ExpiresUnix})
	}
	slices.SortFunc(users, func(a, b telemtUser) int { return strings.Compare(a.name, b.name) })
	return users
}

// telemtDomains returns the FakeTLS domain telemt answers as (the inbound's,
// else the first client's) and every other domain a client secret carries.
func telemtDomains(inst Instance, users []telemtUser) (string, []string) {
	primary := strings.TrimSpace(inst.FakeTLSDomain)
	for _, u := range users {
		if primary == "" && u.domain != "" {
			primary = u.domain
		}
	}
	var extra []string
	for _, u := range users {
		if u.domain != "" && u.domain != primary && !slices.Contains(extra, u.domain) {
			extra = append(extra, u.domain)
		}
	}
	slices.Sort(extra)
	return primary, extra
}

// renderTelemtUsers renders users.toml, the only file a client edit rewrites:
// telemt hot-reloads every [access.*] map in it without dropping connections.
func renderTelemtUsers(inst Instance) string {
	users := telemtUsers(inst)
	var b strings.Builder
	b.WriteString("[access.users]\n")
	for _, u := range users {
		fmt.Fprintf(&b, "%s = %s\n", tomlString(u.name), tomlString(u.key))
	}
	section := func(header string, value func(telemtUser) string) {
		wrote := false
		for _, u := range users {
			v := value(u)
			if v == "" {
				continue
			}
			if !wrote {
				fmt.Fprintf(&b, "\n[%s]\n", header)
				wrote = true
			}
			fmt.Fprintf(&b, "%s = %s\n", tomlString(u.name), v)
		}
	}
	section("access.user_expirations", func(u telemtUser) string {
		if u.expiresUnix <= 0 {
			return ""
		}
		return tomlString(time.Unix(u.expiresUnix, 0).UTC().Format(time.RFC3339))
	})
	section("access.user_max_tcp_conns", func(telemtUser) string {
		if inst.Telemt.MaxTCPConnsPerUser <= 0 {
			return ""
		}
		return strconv.Itoa(inst.Telemt.MaxTCPConnsPerUser)
	})
	section("access.user_ad_tags", func(u telemtUser) string {
		if u.adTag == "" {
			return ""
		}
		return tomlString(u.adTag)
	})
	return b.String()
}

// telemtUsesMiddleProxy: ad-tags only work through Telegram's middle proxy, and
// use_middle_proxy is not hot-reloadable, so the first tagged client restarts telemt.
func telemtUsesMiddleProxy(inst Instance) bool {
	for _, e := range inst.Secrets {
		if e.AdTag != "" {
			return true
		}
	}
	return false
}

// renderTelemtMain renders telemt.toml: everything except the clients, which
// come last through the include so a users-only change never touches this file.
func renderTelemtMain(inst Instance, rt telemtRuntime) string {
	main, warnings := composeTelemtMain(inst, rt)
	for _, w := range warnings {
		logger.Warningf("mtproto: inbound %d %s", inst.Id, w)
	}
	return main
}

// composeTelemtMain renders telemt.toml and returns what it had to leave out,
// so the fingerprint taken every reconcile tick does not log it each time.
func composeTelemtMain(inst Instance, rt telemtRuntime) (string, []string) {
	users := telemtUsers(inst)
	primary, extraDomains := telemtDomains(inst, users)
	preferIPv6 := inst.PreferIP == "prefer-ipv6" || inst.PreferIP == "only-ipv6"
	listen := strings.TrimSpace(inst.Listen)
	if listen == "" {
		listen = "0.0.0.0"
	}

	var b strings.Builder
	b.WriteString("[general]\n")
	fmt.Fprintf(&b, "use_middle_proxy = %t\n", telemtUsesMiddleProxy(inst))
	fmt.Fprintf(&b, "prefer_ipv6 = %t\n", preferIPv6)
	b.WriteString("disable_colors = true\n")
	b.WriteString("\n[general.modes]\nclassic = false\nsecure = false\ntls = true\n")

	b.WriteString("\n[server]\n")
	fmt.Fprintf(&b, "port = %d\n", inst.Port)
	if inst.ProxyProtocolListener {
		b.WriteString("proxy_protocol = true\n")
	}
	fmt.Fprintf(&b, "metrics_listen = \"127.0.0.1:%d\"\n", rt.metricsPort)
	b.WriteString("metrics_whitelist = [\"127.0.0.1/32\"]\n")

	b.WriteString("\n[server.api]\nenabled = true\n")
	fmt.Fprintf(&b, "listen = \"127.0.0.1:%d\"\n", rt.apiPort)
	b.WriteString("whitelist = [\"127.0.0.1/32\"]\n")
	fmt.Fprintf(&b, "auth_header = %s\n", tomlString("Bearer "+rt.apiToken))
	b.WriteString("read_only = true\n")

	b.WriteString("\n[[server.listeners]]\n")
	fmt.Fprintf(&b, "ip = %s\n", tomlString(listen))

	b.WriteString("\n[censorship]\n")
	if primary != "" {
		fmt.Fprintf(&b, "tls_domain = %s\n", tomlString(primary))
	}
	if len(extraDomains) > 0 {
		quoted := make([]string, len(extraDomains))
		for i, d := range extraDomains {
			quoted[i] = tomlString(d)
		}
		fmt.Fprintf(&b, "tls_domains = [%s]\n", strings.Join(quoted, ", "))
	}
	if inst.FrontingIP != "" {
		fmt.Fprintf(&b, "mask_host = %s\n", tomlString(inst.FrontingIP))
	}
	if inst.FrontingPort > 0 {
		fmt.Fprintf(&b, "mask_port = %d\n", inst.FrontingPort)
	}

	if inst.RouteThroughXray && inst.XrayRoutePort > 0 {
		b.WriteString("\n[[upstreams]]\ntype = \"socks5\"\n")
		fmt.Fprintf(&b, "address = \"127.0.0.1:%d\"\n", inst.XrayRoutePort)
	}

	head := b.String()
	merged, warnings, err := mergeTelemtExtraTOML(head, inst, users)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("ignores its telemt extra TOML: %v", err))
	} else {
		head = merged
	}
	return head + "\ninclude = " + tomlString(telemtUsersFile) + "\n", warnings
}

// telemtMainFingerprint is the rendered telemt.toml with placeholder runtime
// ports, so it moves exactly when a change needs a telemt restart.
func telemtMainFingerprint(inst Instance) string {
	main, _ := composeTelemtMain(inst, telemtRuntime{})
	return main
}

// hasIncludeLine mirrors telemt's include preprocessor, which splices in any line
// that trims to `include =`, inside a table or a multi-line string alike.
func hasIncludeLine(text string) bool {
	for line := range strings.Lines(text) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "include")
		if ok && strings.HasPrefix(strings.TrimSpace(rest), "=") {
			return true
		}
	}
	return false
}

// tomlString renders a TOML basic string; unlike strconv.Quote it never emits
// \x escapes, which TOML does not have.
func tomlString(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// telemtForbiddenPaths are what the panel owns in telemt.toml: the listener, the
// API and metrics it manages telemt through, and the FakeTLS domains of the links.
var telemtForbiddenPaths = [][]string{
	{"include"},
	{"server", "port"},
	{"server", "listeners"},
	{"server", "api"},
	{"server", "admin_api"},
	{"server", "metrics_port"},
	{"server", "metrics_listen"},
	{"server", "metrics_whitelist"},
	{"general", "data_path"},
	{"censorship", "tls_domain"},
	{"censorship", "tls_domains"},
}

// telemtForbiddenExtraKey names the first panel-owned key the extra TOML sets,
// "" if none. Per-user [access] maps belong to users.toml, so all are refused.
func telemtForbiddenExtraKey(doc map[string]any, routed bool) string {
	for _, p := range telemtForbiddenPaths {
		if hasPath(doc, p) {
			return strings.Join(p, ".")
		}
	}
	if access, ok := doc["access"].(map[string]any); ok {
		for k, v := range access {
			if _, table := v.(map[string]any); k == "users" || (table && strings.HasPrefix(k, "user_")) {
				return "access." + k
			}
		}
	}
	if _, ok := doc["upstreams"]; ok && routed {
		return "upstreams"
	}
	return ""
}

func hasPath(doc map[string]any, path []string) bool {
	cur := doc
	for i, k := range path {
		v, ok := cur[k]
		if !ok {
			return false
		}
		if i == len(path)-1 {
			return true
		}
		if cur, ok = v.(map[string]any); !ok {
			return false
		}
	}
	return false
}

func deletePath(doc map[string]any, path []string) {
	cur := doc
	for _, k := range path[:len(path)-1] {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return
		}
		cur = next
	}
	delete(cur, path[len(path)-1])
}

// webProfileUsers lists the user of every [[web.vhosts.profiles]] entry.
func webProfileUsers(doc map[string]any) []string {
	var users []string
	forEachWebProfile(doc, func(p map[string]any) bool {
		u, _ := p["user"].(string)
		users = append(users, u)
		return true
	})
	return users
}

// forEachWebProfile walks the WEB profiles; keep=false drops the profile.
func forEachWebProfile(doc map[string]any, keep func(map[string]any) bool) {
	web, _ := doc["web"].(map[string]any)
	vhosts, _ := web["vhosts"].([]any)
	for _, vh := range vhosts {
		vhost, ok := vh.(map[string]any)
		if !ok {
			continue
		}
		profiles, ok := vhost["profiles"].([]any)
		if !ok {
			continue
		}
		kept := profiles[:0]
		for _, p := range profiles {
			pm, ok := p.(map[string]any)
			if !ok || keep(pm) {
				kept = append(kept, p)
			}
		}
		vhost["profiles"] = kept
	}
}

// mergeTelemtExtraTOML merges the extra block under the generated config, panel keys winning.
// It drops WEB profiles of unserved users: telemt rejects the config ("unknown access user").
func mergeTelemtExtraTOML(head string, inst Instance, users []telemtUser) (string, []string, error) {
	extra := inst.Telemt.ExtraTOML
	if strings.TrimSpace(extra) == "" {
		return head, nil, nil
	}
	if hasIncludeLine(extra) {
		return head, nil, errors.New("it has an include line")
	}
	add := map[string]any{}
	if err := toml.Unmarshal([]byte(extra), &add); err != nil {
		return head, nil, err
	}
	for _, p := range telemtForbiddenPaths {
		deletePath(add, p)
	}
	if access, ok := add["access"].(map[string]any); ok {
		for k, v := range access {
			if _, table := v.(map[string]any); k == "users" || (table && strings.HasPrefix(k, "user_")) {
				delete(access, k)
			}
		}
	}
	if inst.RouteThroughXray {
		delete(add, "upstreams")
	}
	served := make(map[string]struct{}, len(users))
	for _, u := range users {
		served[u.name] = struct{}{}
	}
	var warnings []string
	forEachWebProfile(add, func(p map[string]any) bool {
		u, _ := p["user"].(string)
		if _, ok := served[u]; ok {
			return true
		}
		warnings = append(warnings, fmt.Sprintf("drops the telemt WEB profile of %q: the client is not served", u))
		return false
	})
	if len(add) == 0 {
		return head, warnings, nil
	}
	base := map[string]any{}
	if err := toml.Unmarshal([]byte(head), &base); err != nil {
		return head, warnings, err
	}
	mergeUnder(base, add)
	out, err := toml.Marshal(base)
	if err != nil {
		return head, warnings, err
	}
	return string(out), warnings, nil
}

// ValidateSettings checks the telemt side of mtproto settings on save, what telemt
// would refuse or the panel owns; mtg inbounds have nothing to check here.
func ValidateSettings(settings string) error {
	var raw struct {
		Backend          string        `json:"backend"`
		RouteThroughXray bool          `json:"routeThroughXray"`
		Telemt           TelemtOptions `json:"telemt"`
		Clients          []struct {
			Email string `json:"email"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &raw); err != nil {
		return nil // malformed JSON is rejected by the generic settings checks
	}
	if ParseBackend(raw.Backend) != BackendTelemt {
		return nil
	}
	routed := raw.RouteThroughXray
	if n := raw.Telemt.MaxTCPConnsPerUser; n < 0 || n > maxUint16 {
		return fmt.Errorf("mtproto telemt.maxTcpConnsPerUser must be between 0 and %d", maxUint16)
	}
	extra := raw.Telemt.ExtraTOML
	if strings.TrimSpace(extra) == "" {
		return nil
	}
	if len(extra) > MaxExtraTOMLBytes {
		return fmt.Errorf("mtproto telemt.extraToml must be at most %d bytes", MaxExtraTOMLBytes)
	}
	if hasIncludeLine(extra) {
		return fmt.Errorf("mtproto telemt.extraToml may not set %q: telemt reads any include line, nested ones too", "include")
	}
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(extra), &doc); err != nil {
		return fmt.Errorf("mtproto telemt.extraToml is not valid TOML: %s", strings.ReplaceAll(err.Error(), "\n", " "))
	}
	if k := telemtForbiddenExtraKey(doc, routed); k != "" {
		return fmt.Errorf("mtproto telemt.extraToml may not set %q: the panel manages it", k)
	}
	known := make(map[string]struct{}, len(raw.Clients))
	for _, c := range raw.Clients {
		known[c.Email] = struct{}{}
	}
	for _, u := range webProfileUsers(doc) {
		if _, ok := known[u]; !ok {
			return fmt.Errorf("mtproto telemt.extraToml WEB profile user %q is not a client of this inbound", u)
		}
	}
	return nil
}

// SettingsBackend reads the backend of raw mtproto inbound settings.
func SettingsBackend(settings string) Backend {
	var raw struct {
		Backend string `json:"backend"`
	}
	_ = json.Unmarshal([]byte(settings), &raw)
	return ParseBackend(raw.Backend)
}

// TelemtAvailable reports whether this host has the telemt binary to run.
func TelemtAvailable() bool {
	info, err := os.Stat(GetTelemtBinaryPath())
	return err == nil && !info.IsDir()
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

// ValidTelemtListen reports whether listen is one telemt can bind: empty (all
// interfaces) or a literal IP, since its listeners take no hostnames.
func ValidTelemtListen(listen string) bool {
	listen = strings.TrimSpace(listen)
	return listen == "" || net.ParseIP(listen) != nil
}
