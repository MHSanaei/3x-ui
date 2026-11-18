package mtproto

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// eeSecret builds a FakeTLS client secret the way the panel stores it.
func eeSecret(key, domain string) string {
	return "ee" + key + hex.EncodeToString([]byte(domain))
}

const (
	keyAlice = "0123456789abcdef0123456789abcdef"
	keyBob   = "fedcba9876543210fedcba9876543210"
	adTag    = "aaaabbbbccccddddeeeeffff00001111"
)

func telemtInst(secrets ...SecretEntry) Instance {
	return Instance{Id: 7, Tag: "inbound-7", Port: 8443, Secrets: secrets, Backend: BackendTelemt, FakeTLSDomain: "example.com"}
}

func TestSplitFakeTLSSecret(t *testing.T) {
	cases := []struct {
		secret, key, domain string
		ok                  bool
	}{
		{eeSecret(keyAlice, "example.com"), keyAlice, "example.com", true},
		{"dd" + keyBob, keyBob, "", true},
		{strings.ToUpper(eeSecret(keyAlice, "a.io")), keyAlice, "a.io", true},
		{"ee0123", "", "", false},
		{"eezz23456789abcdef0123456789abcdef", "", "", false},
	}
	for _, c := range cases {
		key, domain, ok := splitFakeTLSSecret(c.secret)
		if key != c.key || domain != c.domain || ok != c.ok {
			t.Errorf("splitFakeTLSSecret(%q) = %q, %q, %v; want %q, %q, %v", c.secret, key, domain, ok, c.key, c.domain, c.ok)
		}
	}
}

// TestTomlStringRoundTrip: client emails become TOML keys, and one bad escape
// makes telemt refuse the whole users.toml.
func TestTomlStringRoundTrip(t *testing.T) {
	for _, name := range []string{`a"b`, `back\slash`, "ctl\x01\x7f", "tab\tname", "юзер@пример.рф"} {
		var doc map[string]string
		if err := toml.Unmarshal([]byte("k = "+tomlString(name)), &doc); err != nil {
			t.Fatalf("%q: rendered %s is not TOML: %v", name, tomlString(name), err)
		}
		if doc["k"] != name {
			t.Fatalf("%q round-tripped as %q", name, doc["k"])
		}
	}
}

func TestRenderTelemtUsers(t *testing.T) {
	inst := telemtInst(
		SecretEntry{Name: "zed@example.com", Secret: eeSecret(keyBob, "example.com"), AdTag: adTag},
		SecretEntry{Name: `al"ice`, Secret: eeSecret(keyAlice, "example.com"), ExpiresUnix: 1893456000, QuotaBytes: 1 << 30},
		SecretEntry{Name: "broken", Secret: "ee12"},
	)
	inst.Telemt.MaxTCPConnsPerUser = 8
	want := `[access.users]
"al\"ice" = "0123456789abcdef0123456789abcdef"
"zed@example.com" = "fedcba9876543210fedcba9876543210"

[access.user_expirations]
"al\"ice" = "2030-01-01T00:00:00Z"

[access.user_max_tcp_conns]
"al\"ice" = 8
"zed@example.com" = 8

[access.user_ad_tags]
"zed@example.com" = "aaaabbbbccccddddeeeeffff00001111"
`
	got := renderTelemtUsers(inst)
	if got != want {
		t.Fatalf("users.toml mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if strings.Contains(got, "quota") {
		t.Fatal("the panel holds the quota: telemt must not get user_data_quota it cannot reset under read_only")
	}
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("users.toml must be valid TOML: %v", err)
	}
}

func TestRenderTelemtMain(t *testing.T) {
	inst := telemtInst(
		SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com")},
		SecretEntry{Name: "bob", Secret: eeSecret(keyBob, "other.org")},
	)
	inst.Listen = "10.0.0.5"
	inst.ProxyProtocolListener = true
	inst.FrontingIP = "127.0.0.1"
	inst.FrontingPort = 8444
	want := `[general]
use_middle_proxy = false
prefer_ipv6 = false
disable_colors = true

[general.modes]
classic = false
secure = false
tls = true

[server]
port = 8443
proxy_protocol = true
metrics_listen = "127.0.0.1:9001"
metrics_whitelist = ["127.0.0.1/32"]

[server.api]
enabled = true
listen = "127.0.0.1:9000"
whitelist = ["127.0.0.1/32"]
auth_header = "Bearer tok"
read_only = true

[[server.listeners]]
ip = "10.0.0.5"

[censorship]
tls_domain = "example.com"
tls_domains = ["other.org"]
mask_host = "127.0.0.1"
mask_port = 8444

include = "users.toml"
`
	got := renderTelemtMain(inst, telemtRuntime{apiPort: 9000, metricsPort: 9001, apiToken: "tok"})
	if got != want {
		t.Fatalf("telemt.toml mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderTelemtMainRoutedAndTagged(t *testing.T) {
	inst := telemtInst(SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com"), AdTag: adTag})
	inst.RouteThroughXray = true
	inst.XrayRoutePort = 50001
	inst.PreferIP = "only-ipv6"
	got := renderTelemtMain(inst, telemtRuntime{})
	for _, want := range []string{
		"use_middle_proxy = true\n",
		"prefer_ipv6 = true\n",
		"ip = \"0.0.0.0\"\n",
		"[[upstreams]]\ntype = \"socks5\"\naddress = \"127.0.0.1:50001\"\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("telemt.toml lacks %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "\ninclude = \"users.toml\"\n") {
		t.Fatalf("the users include must close the config:\n%s", got)
	}
}

// TestTelemtFingerprintSplit pins what restarts telemt: only telemt.toml
// changes do; a client edit must stay a users.toml hot reload.
func TestTelemtFingerprintSplit(t *testing.T) {
	base := telemtInst(SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com")})
	mutate := func(f func(*Instance)) Instance {
		inst := base
		inst.Secrets = append([]SecretEntry(nil), base.Secrets...)
		f(&inst)
		return inst
	}
	cases := []struct {
		name          string
		inst          Instance
		restart, hups bool
	}{
		{"client added", mutate(func(i *Instance) {
			i.Secrets = append(i.Secrets, SecretEntry{Name: "bob", Secret: eeSecret(keyBob, "example.com")})
		}), false, true},
		{"expiry changed", mutate(func(i *Instance) { i.Secrets[0].ExpiresUnix = 1893456000 }), false, true},
		{"conn cap changed", mutate(func(i *Instance) { i.Telemt.MaxTCPConnsPerUser = 4 }), false, true},
		{"first ad-tag", mutate(func(i *Instance) { i.Secrets[0].AdTag = adTag }), true, true},
		{"port changed", mutate(func(i *Instance) { i.Port = 9443 }), true, false},
		{"extra toml", mutate(func(i *Instance) { i.Telemt.ExtraTOML = "[server]\nclient_mss = \"tspu\"\n" }), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			restart := telemtMainFingerprint(c.inst) != telemtMainFingerprint(base)
			hups := renderTelemtUsers(c.inst) != renderTelemtUsers(base)
			if restart != c.restart || hups != c.hups {
				t.Fatalf("restart=%v users-changed=%v, want %v %v", restart, hups, c.restart, c.hups)
			}
		})
	}
}

func TestTelemtExtraTOMLMerge(t *testing.T) {
	inst := telemtInst(SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com")})
	inst.RouteThroughXray = true
	inst.XrayRoutePort = 50001
	inst.Telemt.ExtraTOML = `
[general.modes]
tls = false
[server]
client_mss = "tspu"
client_mss_bulk = "1400"
port = 1
[server.api]
read_only = false
[censorship]
tls_domain = "evil.com"
mask = true
[access]
replay_check_len = 65536
[access.users]
mallory = "00000000000000000000000000000000"
[access.user_rate_limits]
alice = { up_bps = 1, down_bps = 1 }
[[upstreams]]
type = "direct"
[[web.vhosts]]
host = "w.example.com"
[[web.vhosts.profiles]]
user = "alice"
[[web.vhosts.profiles]]
user = "gone"
`
	got := renderTelemtMain(inst, telemtRuntime{apiPort: 9000, metricsPort: 9001, apiToken: "tok"})
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(strings.TrimSuffix(got, "include = \"users.toml\"\n")), &doc); err != nil {
		t.Fatalf("merged telemt.toml must be valid TOML: %v\n%s", err, got)
	}
	server := doc["server"].(map[string]any)
	api := server["api"].(map[string]any)
	modes := doc["general"].(map[string]any)["modes"].(map[string]any)
	censorship := doc["censorship"].(map[string]any)
	access := doc["access"].(map[string]any)
	checks := []struct {
		name string
		ok   bool
	}{
		{"extra client_mss kept", server["client_mss"] == "tspu" && server["client_mss_bulk"] == "1400"},
		{"extra replay_check_len kept", access["replay_check_len"] == int64(65536)},
		{"extra censorship.mask kept", censorship["mask"] == true},
		{"panel port wins", server["port"] == int64(8443)},
		{"panel api stays read-only", api["read_only"] == true && api["listen"] == "127.0.0.1:9000"},
		{"panel tls mode wins", modes["tls"] == true},
		{"panel tls_domain wins", censorship["tls_domain"] == "example.com"},
		{"access.users dropped", access["users"] == nil},
		{"per-user maps dropped", access["user_rate_limits"] == nil},
		{"routed upstream wins", len(doc["upstreams"].([]any)) == 1 && doc["upstreams"].([]any)[0].(map[string]any)["type"] == "socks5"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("%s:\n%s", c.name, got)
		}
	}
	if strings.Contains(got, "evil") {
		t.Errorf("a forbidden include or domain leaked into the config:\n%s", got)
	}
	if users := webProfileUsers(doc); len(users) != 1 || users[0] != "alice" {
		t.Errorf("only the profile of a served user may stay, got %v", users)
	}
	if strings.Count(got, "include = ") != 1 {
		t.Errorf("exactly one include, the panel's:\n%s", got)
	}
}

func TestValidateTelemtSettings(t *testing.T) {
	clients := `"clients":[{"email":"alice","secret":"` + eeSecret(keyAlice, "example.com") + `","enable":true}]`
	cases := []struct {
		name     string
		settings string
		wantErr  string
	}{
		{"plain", `{"backend":"telemt",` + clients + `}`, ""},
		{"extra ok", `{"backend":"telemt","telemt":{"maxTcpConnsPerUser":8,"extraToml":"[server]\nclient_mss = \"tspu\"\n[[web.vhosts]]\nhost = \"w\"\n[[web.vhosts.profiles]]\nuser = \"alice\"\n"},` + clients + `}`, ""},
		{"conn cap", `{"backend":"telemt","telemt":{"maxTcpConnsPerUser":70000},` + clients + `}`, "maxTcpConnsPerUser must be between 0 and 65535"},
		{"bad toml", `{"backend":"telemt","telemt":{"extraToml":"[server"},` + clients + `}`, "is not valid TOML"},
		{"api", `{"backend":"telemt","telemt":{"extraToml":"[server.api]\nread_only = false\n"},` + clients + `}`, `may not set "server.api"`},
		{"users", `{"backend":"telemt","telemt":{"extraToml":"[access.users]\nx = \"00\"\n"},` + clients + `}`, `may not set "access.users"`},
		{"per-user map", `{"backend":"telemt","telemt":{"extraToml":"[access.user_data_quota]\nalice = 1\n"},` + clients + `}`, `may not set "access.user_data_quota"`},
		{"routed upstreams", `{"backend":"telemt","routeThroughXray":true,"telemt":{"extraToml":"[[upstreams]]\ntype = \"direct\"\n"},` + clients + `}`, `may not set "upstreams"`},
		{"orphan web profile", `{"backend":"telemt","telemt":{"extraToml":"[[web.vhosts]]\nhost = \"w\"\n[[web.vhosts.profiles]]\nuser = \"gone\"\n"},` + clients + `}`, `WEB profile user "gone" is not a client`},
		{"mtg ignores telemt keys", `{"telemt":{"maxTcpConnsPerUser":70000},` + clients + `}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateSettings(c.settings)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, c.wantErr)
			}
		})
	}
}

func TestInstanceFromInboundTelemt(t *testing.T) {
	ib := &model.Inbound{
		Id: 4, Tag: "inbound-4", Port: 443, Protocol: model.MTProto,
		Settings: `{"backend":"TeLeMt","fakeTlsDomain":" example.com ",` +
			`"telemt":{"maxTcpConnsPerUser":99999,"extraToml":"[server]\n"},` +
			`"clients":[{"email":"alice","secret":"` + eeSecret(keyAlice, "example.com") + `","enable":true}]}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Backend != BackendTelemt || inst.FakeTLSDomain != "example.com" {
		t.Fatalf("backend/domain not parsed: %q %q", inst.Backend, inst.FakeTLSDomain)
	}
	if inst.Telemt.MaxTCPConnsPerUser != 0 || inst.Telemt.ExtraTOML != "[server]\n" {
		t.Fatalf("an out-of-range cap must be dropped, extra kept: %+v", inst.Telemt)
	}
	ib.Settings = strings.Replace(ib.Settings, `"backend":"TeLeMt",`, "", 1)
	if inst, _ := InstanceFromInbound(ib); inst.Backend != BackendMtg {
		t.Fatalf("settings without a backend must stay on mtg, got %q", inst.Backend)
	}
}

// TestTelemtNestedIncludeRefused: telemt splices in any line starting with
// `include =`, inside a table or a multi-line string too, before it parses TOML.
func TestTelemtNestedIncludeRefused(t *testing.T) {
	clients := `"clients":[{"email":"alice","secret":"` + eeSecret(keyAlice, "example.com") + `","enable":true}]`
	for _, extra := range []string{
		`[censorship]\ninclude = \"/etc/shadow\"\n`,
		`[general]\nnote = \"\"\"\n  include = \"/etc/shadow\"\n\"\"\"\n`,
	} {
		err := ValidateSettings(`{"backend":"telemt","telemt":{"extraToml":"` + extra + `"},` + clients + `}`)
		if err == nil || !strings.Contains(err.Error(), `may not set "include"`) {
			t.Errorf("%q: error = %v, want the include refused", extra, err)
		}
	}
	inst := telemtInst(SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com")})
	inst.Telemt.ExtraTOML = "[censorship]\ninclude = \"/etc/shadow\"\nmask = true\n"
	if got := renderTelemtMain(inst, telemtRuntime{}); strings.Contains(got, "shadow") {
		t.Fatalf("a nested include must not reach telemt.toml:\n%s", got)
	}
}

// TestTelemtFingerprintIsQuiet: the reconcile tick fingerprints every inbound every
// 10s, so only an actual render may warn about dropped extra TOML.
func TestTelemtFingerprintIsQuiet(t *testing.T) {
	ghost := fmt.Sprintf("ghost-%d", time.Now().UnixNano())
	inst := telemtInst(SecretEntry{Name: "alice", Secret: eeSecret(keyAlice, "example.com")})
	inst.Telemt.ExtraTOML = "[[web.vhosts]]\nhost = \"w\"\n[[web.vhosts.profiles]]\nuser = \"" + ghost + "\"\n"
	warnings := func() int {
		n := 0
		for _, line := range logger.GetLogs(5000, "warning") {
			if strings.Contains(line, ghost) {
				n++
			}
		}
		return n
	}
	for range 3 {
		telemtMainFingerprint(inst)
	}
	if n := warnings(); n != 0 {
		t.Fatalf("fingerprinting logged %d warning(s)", n)
	}
	renderTelemtMain(inst, telemtRuntime{apiPort: 1, metricsPort: 2})
	if n := warnings(); n != 1 {
		t.Fatalf("the render must warn once about the dropped profile, got %d", n)
	}
}
