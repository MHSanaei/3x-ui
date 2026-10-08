package mtproto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const optTestSecret = "ee0123456789abcdef0123456789abcdef6578616d706c652e636f6d"

func optInstance(opts Options) Instance {
	return Instance{
		Id: 1, Listen: "0.0.0.0", Port: 443,
		Secrets: []SecretEntry{{Name: "alice", Secret: optTestSecret, QuotaBytes: 1 << 30, AdTag: "fedcba9876543210fedcba9876543210"}},
		Options: opts,
	}
}

// fullOptions sets every option to a valid non-default value.
func fullOptions() Options {
	return Options{
		Concurrency:              2048,
		TolerateTimeSkewness:     "5s",
		AllowFallbackOnUnknownDC: true,
		AutoUpdate:               true,
		ThrottleCheckInterval:    "10s",
		Network: NetworkOptions{
			DNS:             "https://1.1.1.1/dns-query",
			Proxies:         []string{"socks5://user:pass@10.0.0.1:1080"},
			TCPNotSentLowat: "1mib",
			ClientMSS:       92,
			ClientMSSBulk:   new(1200),
			Timeout:         TimeoutOptions{TCP: "5s", HTTP: "10s", Idle: "5m", Handshake: "10s"},
			KeepAlive:       KeepAliveOptions{Disabled: true, Idle: "15s", Interval: "15s", Count: 9},
		},
		Defense: DefenseOptions{
			AntiReplay: AntiReplayOptions{Enabled: true, MaxSize: "1mib", ErrorRate: 0.001},
			Blocklist: IPListOptions{
				Enabled: true, URLs: []string{"https://iplists.firehol.org/files/firehol_level1.netset"},
				UpdateEach: "24h", DownloadConcurrency: 2,
			},
			Allowlist:         IPListOptions{Enabled: true, URLs: []string{"https://example.com/ru.netset"}, UpdateEach: "12h"},
			Doppelganger:      DoppelgangerOptions{URLs: []string{"https://cdn.example.com/a.js"}, RepeatsPerRaid: 10, RaidEach: "6h", DRS: true},
			PendingHandshakes: PendingHandshakesOptions{MaxPerIP: 32, DryRun: true},
		},
		DCPool: DCPoolOptions{DCs: []int{1, 2, 3, 4, 5, -2, -4, 203}},
		Stats: StatsOptions{
			Prometheus: PrometheusOptions{Enabled: true, BindTo: "127.0.0.1:3129", HTTPPath: "/metrics", MetricPrefix: "mtg"},
			StatsD:     StatsDOptions{Enabled: true, Address: "10.0.0.5:8125", MetricPrefix: "mtg", TagFormat: "influxdb"},
		},
		Web: WebOptions{
			BindTo: "127.0.0.1:18080", Host: "proxy.example.com", SecretMode: "plain", DecoyDir: "/var/www/decoy",
			TrustedProxies: []string{"127.0.0.1/32", "::1/128"}, MaxSessions: 1024, MaxPending: 4096, Diag: true,
		},
	}
}

func TestRenderOptionGroups(t *testing.T) {
	inst := optInstance(fullOptions())
	inst.ThrottleMaxConnections = 100
	inst.DCPoolEnabled, inst.DCPoolSize = true, 4
	cfg := renderConfig(inst, 5000, "tok")

	for _, want := range []string{
		"concurrency = 2048\ntolerate-time-skewness = \"5s\"\nallow-fallback-on-unknown-dc = true\nauto-update = true\n",
		"\n[network]\nproxies = [\"socks5://user:pass@10.0.0.1:1080\"]\ndns = \"https://1.1.1.1/dns-query\"\ntcp-not-sent-lowat = \"1mib\"\nclient-mss = 92\nclient-mss-bulk = 1200\n",
		"\n[network.timeout]\ntcp = \"5s\"\nhttp = \"10s\"\nidle = \"5m\"\nhandshake = \"10s\"\n",
		"\n[network.keep-alive]\ndisabled = true\nidle = \"15s\"\ninterval = \"15s\"\ncount = 9\n",
		"\n[throttle]\nmax-connections = 100\ncheck-interval = \"10s\"\n",
		"\n[dc-pool]\nenabled = true\nsize = 4\ndcs = [1, 2, 3, 4, 5, -2, -4, 203]\n",
		"\n[defense.anti-replay]\nenabled = true\nmax-size = \"1mib\"\nerror-rate = 0.001\n",
		"\n[defense.blocklist]\nenabled = true\ndownload-concurrency = 2\nurls = [\"https://iplists.firehol.org/files/firehol_level1.netset\"]\nupdate-each = \"24h\"\n",
		"\n[defense.allowlist]\nenabled = true\nurls = [\"https://example.com/ru.netset\"]\nupdate-each = \"12h\"\n",
		"\n[defense.doppelganger]\nurls = [\"https://cdn.example.com/a.js\"]\nrepeats-per-raid = 10\nraid-each = \"6h\"\ndrs = true\n",
		"\n[defense.pending-handshakes]\nmax-per-ip = 32\ndry-run = true\n",
		"\n[stats.prometheus]\nenabled = true\nbind-to = \"127.0.0.1:3129\"\nhttp-path = \"/metrics\"\nmetric-prefix = \"mtg\"\n",
		"\n[stats.statsd]\nenabled = true\naddress = \"10.0.0.5:8125\"\nmetric-prefix = \"mtg\"\ntag-format = \"influxdb\"\n",
		"\n[web]\nbind-to = \"127.0.0.1:18080\"\nhost = \"proxy.example.com\"\nsecret-mode = \"plain\"\ndecoy-dir = \"/var/www/decoy\"\ntrusted-proxies = [\"127.0.0.1/32\", \"::1/128\"]\nmax-sessions = 1024\nmax-pending = 4096\ndiag = true\n",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config is missing %q:\n%s", want, cfg)
		}
	}
	assertValidMtgConfig(t, cfg)
}

// assertValidMtgConfig parses cfg as TOML and checks the panel-owned keys are
// intact and [secrets] is the last section.
func assertValidMtgConfig(t *testing.T, cfg string) map[string]any {
	t.Helper()
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(cfg), &doc); err != nil {
		t.Fatalf("config is not valid TOML: %v\n%s", err, cfg)
	}
	secrets, ok := doc["secrets"].(map[string]any)
	if !ok || secrets["alice"] != optTestSecret {
		t.Fatalf("[secrets] lost the client secret: %v", doc["secrets"])
	}
	if doc["bind-to"] != "0.0.0.0:443" || doc["api-bind-to"] != "127.0.0.1:5000" {
		t.Fatalf("panel-owned keys changed: bind-to=%v api-bind-to=%v", doc["bind-to"], doc["api-bind-to"])
	}
	last := strings.LastIndex(cfg, "\n[")
	if !strings.HasPrefix(cfg[last:], "\n[secrets]\n") {
		t.Fatalf("[secrets] must be the last section:\n%s", cfg)
	}
	return doc
}

// Disabled sections render nothing, even with their sub-keys filled in.
func TestRenderSkipsDisabledSections(t *testing.T) {
	o := Options{
		Defense: DefenseOptions{
			AntiReplay:        AntiReplayOptions{MaxSize: "1mib"},
			Blocklist:         IPListOptions{URLs: []string{"https://example.com/l.netset"}},
			PendingHandshakes: PendingHandshakesOptions{DryRun: true},
		},
		Stats: StatsOptions{
			Prometheus: PrometheusOptions{BindTo: "127.0.0.1:3129"},
			StatsD:     StatsDOptions{Address: "127.0.0.1:8125"},
		},
		Web: WebOptions{Host: "proxy.example.com"},
	}
	cfg := renderConfig(optInstance(o), 5000, "")
	for _, unwanted := range []string{"[defense", "[stats", "[web]"} {
		if strings.Contains(cfg, unwanted) {
			t.Fatalf("a disabled section must not render %q:\n%s", unwanted, cfg)
		}
	}
}

// Routing through Xray owns mtg's proxies: the bridge stays the only upstream,
// and an inbound with nothing else set renders exactly what it did before.
func TestRenderNetworkWithXrayRoute(t *testing.T) {
	inst := optInstance(Options{})
	inst.Secrets = []SecretEntry{{Name: "alice", Secret: optTestSecret}}
	inst.RouteThroughXray, inst.XrayRoutePort = true, 50000
	want := "bind-to = \"0.0.0.0:443\"\napi-bind-to = \"127.0.0.1:5000\"\n" +
		"\n[network]\nproxies = [\"socks5://127.0.0.1:50000\"]\n" +
		"\n[secrets]\n\"alice\" = \"" + optTestSecret + "\"\n"
	if got := renderConfig(inst, 5000, ""); got != want {
		t.Fatalf("route-only config changed:\n got: %q\nwant: %q", got, want)
	}

	inst.Options.Network = NetworkOptions{DNS: "1.1.1.1", Proxies: []string{"socks5://10.0.0.1:1080"}}
	cfg := renderConfig(inst, 5000, "")
	if !strings.Contains(cfg, "\n[network]\nproxies = [\"socks5://127.0.0.1:50000\"]\ndns = \"1.1.1.1\"\n") {
		t.Fatalf("the Xray bridge must replace user proxies and dns must join [network]:\n%s", cfg)
	}
	if strings.Count(cfg, "[network]") != 1 {
		t.Fatalf("[network] must be emitted once:\n%s", cfg)
	}

	// A relay splits the ServerHello towards its clients and still dials
	// Telegram through the bridge: both land in the one [network] table.
	inst.Options.Network = NetworkOptions{ClientMSS: 92}
	if cfg := renderConfig(inst, 5000, ""); !strings.Contains(cfg, "\n[network]\nproxies = [\"socks5://127.0.0.1:50000\"]\nclient-mss = 92\n\n") {
		t.Fatalf("client-mss must join the bridged [network]:\n%s", cfg)
	}
}

// client-mss-bulk is written only next to client-mss (mtg ignores it alone),
// and an explicit 0 must survive: it keeps the whole session at client-mss.
func TestRenderClientMSS(t *testing.T) {
	for _, tc := range []struct {
		name     string
		network  NetworkOptions
		want     string
		wantBulk any
	}{
		{"bulk left to mtg", NetworkOptions{ClientMSS: 92}, "client-mss = 92\n", nil},
		{"own bulk", NetworkOptions{ClientMSS: 92, ClientMSSBulk: new(1400)}, "client-mss = 92\nclient-mss-bulk = 1400\n", int64(1400)},
		{"whole session small", NetworkOptions{ClientMSS: 120, ClientMSSBulk: new(0)}, "client-mss = 120\nclient-mss-bulk = 0\n", int64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := renderConfig(optInstance(Options{Network: tc.network}), 5000, "")
			if !strings.Contains(cfg, "\n[network]\n"+tc.want+"\n") {
				t.Fatalf("config is missing %q:\n%s", tc.want, cfg)
			}
			network := assertValidMtgConfig(t, cfg)["network"].(map[string]any)
			if network["client-mss-bulk"] != tc.wantBulk {
				t.Fatalf("client-mss-bulk = %v, want %v", network["client-mss-bulk"], tc.wantBulk)
			}
		})
	}

	cfg := renderConfig(optInstance(Options{Network: NetworkOptions{ClientMSSBulk: new(1400)}}), 5000, "")
	if strings.Contains(cfg, "client-mss") || strings.Contains(cfg, "[network]") {
		t.Fatalf("client-mss-bulk without client-mss must render nothing:\n%s", cfg)
	}
}

func TestValidateSettings(t *testing.T) {
	const clients = `"clients":[{"email":"a","secret":"` + optTestSecret + `","enable":true}]`
	full, err := json.Marshal(fullOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSettings(string(full)); err != nil {
		t.Fatalf("every valid option must pass: %v", err)
	}
	for _, ok := range []string{
		`{` + clients + `}`,
		`{"network":{"dns":"tls://dns.example.com"},` + clients + `}`,
		`{"network":{"dns":"udp://8.8.8.8"},` + clients + `}`,
		`{"network":{"tcpNotSentLowat":"4 MiB"},` + clients + `}`,
		`{"network":{"clientMss":48},` + clients + `}`,
		`{"network":{"clientMss":1460,"clientMssBulk":65495},` + clients + `}`,
		`{"network":{"clientMss":92,"clientMssBulk":536},` + clients + `}`,
		`{"network":{"clientMss":88,"clientMssBulk":0},` + clients + `}`,
		`{"network":{"clientMss":0,"clientMssBulk":0},` + clients + `}`,
		`{"routeThroughXray":true,"network":{"clientMss":92},` + clients + `}`,
		`{"defense":{"pendingHandshakes":{"maxPerIp":0}},` + clients + `}`,
		`{"web":{"bindTo":"[::1]:18080","host":"proxy.example.com"},` + clients + `}`,
		`{"dcPool":{"enabled":true,"dcs":[-32768,32767]},` + clients + `}`,
		`{"extraToml":"# only a comment\n",` + clients + `}`,
		`{"extraToml":"usage-state-file = \"/var/lib/mtg/usage.json\"\n[web]\nmax-sessions-per-user = 4\n",` + clients + `}`,
	} {
		if err := ValidateSettings(ok); err != nil {
			t.Fatalf("%s: unexpected error %v", ok, err)
		}
	}

	for _, tc := range []struct{ settings, wantPath string }{
		{`{"concurrency":70000}`, "concurrency"},
		{`{"concurrency":-1}`, "concurrency"},
		{`{"concurrency":"many"}`, "concurrency"},
		{`{"tolerateTimeSkewness":"5"}`, "tolerateTimeSkewness"},
		{`{"tolerateTimeSkewness":"-5s"}`, "tolerateTimeSkewness"},
		{`{"throttleCheckInterval":"soon"}`, "throttleCheckInterval"},
		{`{"network":{"dns":"ftp://1.1.1.1"}}`, "network.dns"},
		{`{"network":{"dns":"udp://dns.google"}}`, "network.dns"},
		{`{"network":{"proxies":["http://10.0.0.1:3128"]}}`, "network.proxies"},
		{`{"routeThroughXray":true,"network":{"proxies":["socks5://10.0.0.1:1080"]}}`, "network.proxies"},
		{`{"network":{"tcpNotSentLowat":"lots"}}`, "network.tcpNotSentLowat"},
		{`{"network":{"clientMss":47}}`, "network.clientMss must be 0 or between 48 and 1460"},
		{`{"network":{"clientMss":1461}}`, "network.clientMss must be 0 or between 48 and 1460"},
		{`{"network":{"clientMss":-1}}`, "network.clientMss must be 0 or between 48 and 1460"},
		{`{"network":{"clientMss":92,"clientMssBulk":535}}`, "network.clientMssBulk must be 0 or between 536 and 65495"},
		{`{"network":{"clientMss":92,"clientMssBulk":65496}}`, "network.clientMssBulk must be 0 or between 536 and 65495"},
		{`{"network":{"clientMss":1400,"clientMssBulk":1400}}`, "network.clientMssBulk must be greater than network.clientMss"},
		{`{"network":{"clientMss":87,"clientMssBulk":0}}`, "network.clientMss must be at least 88 when network.clientMssBulk is 0"},
		{`{"network":{"timeout":{"tcp":"0s"}}}`, "network.timeout.tcp"},
		{`{"network":{"timeout":{"handshake":"10"}}}`, "network.timeout.handshake"},
		{`{"network":{"keepAlive":{"count":70000}}}`, "network.keepAlive.count"},
		{`{"defense":{"antiReplay":{"maxSize":"1 lightyear"}}}`, "defense.antiReplay.maxSize"},
		{`{"defense":{"antiReplay":{"errorRate":100}}}`, "defense.antiReplay.errorRate"},
		{`{"defense":{"blocklist":{"enabled":true}}}`, "defense.blocklist.urls"},
		{`{"defense":{"blocklist":{"urls":["/etc/list.netset"]}}}`, "defense.blocklist.urls"},
		{`{"defense":{"allowlist":{"enabled":true,"urls":[]}}}`, "defense.allowlist.urls"},
		{`{"defense":{"allowlist":{"updateEach":"daily"}}}`, "defense.allowlist.updateEach"},
		{`{"defense":{"blocklist":{"downloadConcurrency":70000}}}`, "defense.blocklist.downloadConcurrency"},
		{`{"defense":{"doppelganger":{"urls":["http://example.com/a.js"]}}}`, "defense.doppelganger.urls"},
		{`{"defense":{"doppelganger":{"repeatsPerRaid":-1}}}`, "defense.doppelganger.repeatsPerRaid"},
		{`{"defense":{"pendingHandshakes":{"maxPerIp":70000}}}`, "defense.pendingHandshakes.maxPerIp"},
		{`{"dcPool":{"enabled":true,"dcs":[2,0]}}`, "dcPool.dcs"},
		{`{"dcPool":{"enabled":true,"dcs":[2,-2,2]}}`, "dcPool.dcs"},
		{`{"dcPool":{"enabled":true,"dcs":[40000]}}`, "dcPool.dcs"},
		{`{"dcPool":{"enabled":true,"dcs":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33]}}`, "dcPool.dcs"},
		{`{"dcPool":{"size":65}}`, "dcPool.size"},
		{`{"stats":{"prometheus":{"enabled":true}}}`, "stats.prometheus.bindTo"},
		{`{"stats":{"prometheus":{"enabled":true,"bindTo":"0.0.0.0:3129"}}}`, "stats.prometheus.bindTo"},
		{`{"stats":{"prometheus":{"metricPrefix":"mtg_proxy"}}}`, "stats.prometheus.metricPrefix"},
		{`{"stats":{"prometheus":{"httpPath":"metrics"}}}`, "stats.prometheus.httpPath"},
		{`{"stats":{"statsd":{"enabled":true,"address":"statsd.local:8125"}}}`, "stats.statsd.address"},
		{`{"stats":{"statsd":{"tagFormat":"json"}}}`, "stats.statsd.tagFormat"},
		{`{"web":{"bindTo":"0.0.0.0:18080","host":"proxy.example.com"}}`, "web.bindTo"},
		{`{"web":{"bindTo":"127.0.0.1:18080"}}`, "web.host"},
		{`{"web":{"bindTo":"127.0.0.1:18080","host":"not a host"}}`, "web.host"},
		{`{"web":{"secretMode":"ee"}}`, "web.secretMode"},
		{`{"web":{"decoyDir":"www/decoy"}}`, "web.decoyDir"},
		{`{"web":{"trustedProxies":["10.0.0.1"]}}`, "web.trustedProxies"},
		{`{"web":{"maxSessions":70000}}`, "web.maxSessions"},
		{`{"extraToml":"concurrency = "}`, "extraToml"},
		{`{"extraToml":"[secrets]\nmallory = \"ee00\"\n"}`, "extraToml"},
		{`{"extraToml":"api-token = \"x\"\n"}`, "extraToml"},
		{`{"extraToml":"bind-to = \"0.0.0.0:1\"\n"}`, "extraToml"},
		{`{"extraToml":"[secret-limits.alice]\nquota = \"1B\"\n"}`, "extraToml"},
	} {
		err := ValidateSettings(tc.settings)
		if err == nil || !strings.Contains(err.Error(), tc.wantPath) {
			t.Fatalf("%s: err = %v, want one naming %s", tc.settings, err, tc.wantPath)
		}
	}
}

// Values a raw API payload smuggles past validation are dropped on read, so
// mtg never sees them.
func TestInstanceFromInboundSanitizesOptions(t *testing.T) {
	settings := `{"concurrency":70000,"tolerateTimeSkewness":"5s",` +
		`"network":{"dns":"ftp://x","clientMss":2000,"clientMssBulk":100,"timeout":{"idle":"1m"}},` +
		`"defense":{"blocklist":{"enabled":true,"urls":["file:///etc/passwd"]}},` +
		`"stats":{"prometheus":{"enabled":true,"bindTo":"0.0.0.0:3129"}},` +
		`"web":{"bindTo":"0.0.0.0:80","host":"proxy.example.com"},` +
		`"dcPool":{"enabled":false,"dcs":[2]},` +
		`"extraToml":"[secrets]\nmallory = \"ee00\"\n",` +
		`"clients":[{"email":"alice","secret":"` + optTestSecret + `","enable":true}]}`
	inst, ok := InstanceFromInbound(&model.Inbound{Id: 1, Port: 443, Protocol: model.MTProto, Settings: settings})
	if !ok {
		t.Fatal("expected a usable instance")
	}
	o := inst.Options
	if o.Concurrency != 0 || o.Network.DNS != "" || o.Network.ClientMSS != 0 || o.Network.ClientMSSBulk != nil || o.Defense.Blocklist.Enabled || o.Stats.Prometheus.Enabled ||
		o.Web.BindTo != "" || o.ExtraTOML != "" || o.DCPool.DCs != nil {
		t.Fatalf("invalid options must be dropped: %+v", o)
	}
	if o.TolerateTimeSkewness != "5s" || o.Network.Timeout.Idle != "1m" {
		t.Fatalf("valid options must survive next to invalid ones: %+v", o)
	}
	cfg := renderConfig(inst, 5000, "")
	if strings.Contains(cfg, "mallory") || strings.Contains(cfg, "0.0.0.0:3129") {
		t.Fatalf("sanitized values leaked into the config:\n%s", cfg)
	}
}

func TestInstanceFromInboundReadsOptions(t *testing.T) {
	want := fullOptions()
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	settings := strings.TrimSuffix(string(raw), "}") +
		`,"dcPool":{"enabled":true,"dcs":[1,2,3,4,5,-2,-4,203]},` +
		`"clients":[{"email":"alice","secret":"` + optTestSecret + `","enable":true}]}`
	inst, ok := InstanceFromInbound(&model.Inbound{Id: 1, Port: 443, Protocol: model.MTProto, Settings: settings})
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Options.fingerprint() != want.fingerprint() {
		t.Fatalf("options were not read back intact:\n got: %s\nwant: %s", inst.Options.fingerprint(), want.fingerprint())
	}
}

// Every option is structural: changing any of them must restart mtg, while
// the reloadable secrets fingerprint stays put.
func TestOptionsFingerprint(t *testing.T) {
	base := optInstance(Options{})
	for name, mutate := range map[string]func(*Options){
		"concurrency":       func(o *Options) { o.Concurrency = 10 },
		"skew":              func(o *Options) { o.TolerateTimeSkewness = "5s" },
		"fallback":          func(o *Options) { o.AllowFallbackOnUnknownDC = true },
		"autoUpdate":        func(o *Options) { o.AutoUpdate = true },
		"throttleInterval":  func(o *Options) { o.ThrottleCheckInterval = "5s" },
		"dns":               func(o *Options) { o.Network.DNS = "1.1.1.1" },
		"proxies":           func(o *Options) { o.Network.Proxies = []string{"socks5://10.0.0.1:1080"} },
		"lowat":             func(o *Options) { o.Network.TCPNotSentLowat = "1mib" },
		"clientMss":         func(o *Options) { o.Network.ClientMSS = 92 },
		"clientMssBulk":     func(o *Options) { o.Network.ClientMSSBulk = new(0) },
		"timeoutTCP":        func(o *Options) { o.Network.Timeout.TCP = "1s" },
		"timeoutHTTP":       func(o *Options) { o.Network.Timeout.HTTP = "1s" },
		"timeoutIdle":       func(o *Options) { o.Network.Timeout.Idle = "1s" },
		"timeoutHandshake":  func(o *Options) { o.Network.Timeout.Handshake = "1s" },
		"keepAliveOff":      func(o *Options) { o.Network.KeepAlive.Disabled = true },
		"keepAliveIdle":     func(o *Options) { o.Network.KeepAlive.Idle = "1s" },
		"keepAliveInterval": func(o *Options) { o.Network.KeepAlive.Interval = "1s" },
		"keepAliveCount":    func(o *Options) { o.Network.KeepAlive.Count = 3 },
		"antiReplay":        func(o *Options) { o.Defense.AntiReplay.Enabled = true },
		"antiReplaySize":    func(o *Options) { o.Defense.AntiReplay.MaxSize = "2mib" },
		"antiReplayRate":    func(o *Options) { o.Defense.AntiReplay.ErrorRate = 0.01 },
		"blocklist":         func(o *Options) { o.Defense.Blocklist.Enabled = true },
		"blocklistURLs":     func(o *Options) { o.Defense.Blocklist.URLs = []string{"https://a.b/c"} },
		"blocklistEach":     func(o *Options) { o.Defense.Blocklist.UpdateEach = "1h" },
		"blocklistConc":     func(o *Options) { o.Defense.Blocklist.DownloadConcurrency = 3 },
		"allowlist":         func(o *Options) { o.Defense.Allowlist.Enabled = true },
		"doppelURLs":        func(o *Options) { o.Defense.Doppelganger.URLs = []string{"https://a.b/c"} },
		"doppelRepeats":     func(o *Options) { o.Defense.Doppelganger.RepeatsPerRaid = 3 },
		"doppelEach":        func(o *Options) { o.Defense.Doppelganger.RaidEach = "1h" },
		"doppelDRS":         func(o *Options) { o.Defense.Doppelganger.DRS = true },
		"pendingMax":        func(o *Options) { o.Defense.PendingHandshakes.MaxPerIP = 8 },
		"pendingDry":        func(o *Options) { o.Defense.PendingHandshakes.DryRun = true },
		"dcs":               func(o *Options) { o.DCPool.DCs = []int{2} },
		"prometheus":        func(o *Options) { o.Stats.Prometheus.Enabled = true },
		"prometheusBind":    func(o *Options) { o.Stats.Prometheus.BindTo = "127.0.0.1:1" },
		"prometheusPath":    func(o *Options) { o.Stats.Prometheus.HTTPPath = "/m" },
		"prometheusPrefix":  func(o *Options) { o.Stats.Prometheus.MetricPrefix = "x" },
		"statsd":            func(o *Options) { o.Stats.StatsD.Enabled = true },
		"statsdAddress":     func(o *Options) { o.Stats.StatsD.Address = "127.0.0.1:1" },
		"statsdPrefix":      func(o *Options) { o.Stats.StatsD.MetricPrefix = "x" },
		"statsdTags":        func(o *Options) { o.Stats.StatsD.TagFormat = "graphite" },
		"webBind":           func(o *Options) { o.Web.BindTo = "127.0.0.1:1" },
		"webHost":           func(o *Options) { o.Web.Host = "a.example.com" },
		"webMode":           func(o *Options) { o.Web.SecretMode = "plain" },
		"webDecoy":          func(o *Options) { o.Web.DecoyDir = "/srv" },
		"webTrusted":        func(o *Options) { o.Web.TrustedProxies = []string{"10.0.0.0/8"} },
		"webSessions":       func(o *Options) { o.Web.MaxSessions = 1 },
		"webPending":        func(o *Options) { o.Web.MaxPending = 1 },
		"webDiag":           func(o *Options) { o.Web.Diag = true },
		"extra":             func(o *Options) { o.ExtraTOML = "usage-state-file = \"/x\"" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed.Options)
			if base.structuralFingerprint() == changed.structuralFingerprint() {
				t.Fatalf("structural fingerprint must change when %s changes", name)
			}
			if base.secretsFingerprint() != changed.secretsFingerprint() {
				t.Fatalf("secrets fingerprint must stay put when %s changes", name)
			}
		})
	}
}

func TestExtraTOMLMerge(t *testing.T) {
	inst := optInstance(Options{Concurrency: 100, Network: NetworkOptions{DNS: "1.1.1.1"}})
	inst.Options.ExtraTOML = "concurrency = 5\n" + // the panel's value wins
		"usage-state-file = \"/var/lib/mtg/usage.json\"\n" + // a key the panel does not write is added
		"api-token = \"stolen\"\n" + // forbidden keys are dropped
		"bind-to = \"0.0.0.0:1\"\n" +
		"[network]\ndns = \"9.9.9.9\"\ntcp-not-sent-lowat = \"4mib\"\n" + // tables merge key by key
		"[web]\nmax-sessions-per-user = 4\n" +
		"[secrets]\nmallory = \"ee00\"\n" +
		"[secret-limits.alice]\nquota = \"1B\"\n"
	cfg := renderConfig(inst, 5000, "tok")
	doc := assertValidMtgConfig(t, cfg)

	if doc["concurrency"] != int64(100) {
		t.Fatalf("panel keys must win over the extra TOML, concurrency = %v", doc["concurrency"])
	}
	if doc["usage-state-file"] != "/var/lib/mtg/usage.json" {
		t.Fatalf("extra keys must be added, got %v", doc["usage-state-file"])
	}
	if doc["api-token"] != "tok" {
		t.Fatalf("the extra TOML must not replace the API token, got %v", doc["api-token"])
	}
	network := doc["network"].(map[string]any)
	if network["dns"] != "1.1.1.1" || network["tcp-not-sent-lowat"] != "4mib" {
		t.Fatalf("tables must merge with the panel winning: %v", network)
	}
	if web := doc["web"].(map[string]any); web["max-sessions-per-user"] != int64(4) {
		t.Fatalf("extra-only tables must be kept: %v", web)
	}
	if len(doc["secrets"].(map[string]any)) != 1 || strings.Contains(cfg, "mallory") {
		t.Fatalf("the extra TOML must not add secrets:\n%s", cfg)
	}
	limits := doc["secret-limits"].(map[string]any)["alice"].(map[string]any)
	if limits["quota"] != "1073741824B" {
		t.Fatalf("the extra TOML must not touch client limits: %v", limits)
	}
	if strings.Count(cfg, "[secrets]") != 1 {
		t.Fatalf("[secrets] must appear once:\n%s", cfg)
	}
}

// A block that adds nothing (empty, comments, or only forbidden keys) keeps
// the generated config byte for byte.
func TestExtraTOMLNoopKeepsConfig(t *testing.T) {
	plain := renderConfig(optInstance(Options{Concurrency: 100}), 5000, "tok")
	for _, extra := range []string{"", "  \n", "# just a note\n", "secrets = {}\napi-bind-to = \"127.0.0.1:1\"\n"} {
		got := renderConfig(optInstance(Options{Concurrency: 100, ExtraTOML: extra}), 5000, "tok")
		if got != plain {
			t.Fatalf("extra %q changed the config:\n got: %q\nwant: %q", extra, got, plain)
		}
	}
}
