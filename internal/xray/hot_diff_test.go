package xray

import (
	"os"
	"strings"
	"testing"

	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"

	"github.com/op/go-logging"
)

func TestMain(m *testing.M) {
	// ComputeHotDiff logs the section that blocks a hot apply; the package
	// logger must exist before any test exercises a blocked path.
	xuilogger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

func makeHotConfig() *Config {
	return &Config{
		LogConfig:       json_util.RawMessage(`{"loglevel":"warning"}`),
		RouterConfig:    json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"}]}`),
		OutboundConfigs: json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"blackhole","tag":"blocked"}]`),
		Policy:          json_util.RawMessage(`{}`),
		API:             json_util.RawMessage(`{"services":["HandlerService","StatsService","RoutingService"],"tag":"api"}`),
		Stats:           json_util.RawMessage(`{}`),
		Metrics:         json_util.RawMessage(`{}`),
		InboundConfigs: []InboundConfig{
			{
				Port:     62789,
				Protocol: "tunnel",
				Tag:      "api",
				Listen:   json_util.RawMessage(`"127.0.0.1"`),
				Settings: json_util.RawMessage(`{}`),
			},
			{
				Port:     1080,
				Protocol: "vless",
				Tag:      "inbound-1080",
				Listen:   json_util.RawMessage(`"0.0.0.0"`),
				Settings: json_util.RawMessage(`{"clients":[]}`),
			},
		},
	}
}

func TestComputeHotDiff_NoChanges(t *testing.T) {
	diff, ok := ComputeHotDiff(makeHotConfig(), makeHotConfig())
	if !ok {
		t.Fatal("identical configs must be hot-appliable")
	}
	if !diff.Empty() {
		t.Fatalf("identical configs must produce an empty diff, got %+v", diff)
	}
}

func TestComputeHotDiff_FormattingOnlyChangeIsEmptyDiff(t *testing.T) {
	oldCfg := makeHotConfig()
	newCfg := makeHotConfig()
	// Reformat every section the way a frontend textarea save would.
	newCfg.LogConfig = json_util.RawMessage("{\n  \"loglevel\": \"warning\"\n}")
	newCfg.Policy = json_util.RawMessage("{ }")
	newCfg.API = json_util.RawMessage("{\n  \"services\": [\"HandlerService\", \"StatsService\", \"RoutingService\"],\n  \"tag\": \"api\"\n}")
	newCfg.OutboundConfigs = json_util.RawMessage("[\n  {\"protocol\": \"freedom\", \"tag\": \"direct\"},\n  {\"protocol\": \"blackhole\", \"tag\": \"blocked\"}\n]")
	newCfg.InboundConfigs[1].Settings = json_util.RawMessage("{\n  \"clients\": []\n}")

	diff, ok := ComputeHotDiff(oldCfg, newCfg)
	if !ok {
		t.Fatal("formatting-only change must be hot-appliable")
	}
	if len(diff.RemovedInboundTags) != 0 || len(diff.AddedInbounds) != 0 ||
		len(diff.RemovedOutboundTags) != 0 || len(diff.AddedOutbounds) != 0 {
		t.Fatalf("formatting-only change must produce no handler ops, got %+v", diff)
	}
}

func TestComputeHotDiff_CanonicalEquality(t *testing.T) {
	// Key reorder in a static section (the DNS editor rebuilds the object on
	// save) must not read as a change.
	oldCfg := makeHotConfig()
	oldCfg.DNSConfig = json_util.RawMessage(`{"servers":["1.1.1.1"],"queryStrategy":"UseIP","tag":"dns-in"}`)
	newCfg := makeHotConfig()
	newCfg.DNSConfig = json_util.RawMessage(`{"tag":"dns-in","queryStrategy":"UseIP","servers":["1.1.1.1"]}`)
	diff, ok := ComputeHotDiff(oldCfg, newCfg)
	if !ok || !diff.Empty() {
		t.Fatalf("dns key reorder must be an empty hot diff, ok=%v diff=%+v", ok, diff)
	}

	// Explicit null and an absent section are the same thing.
	newCfg = makeHotConfig()
	newCfg.FakeDNS = json_util.RawMessage(`null`)
	diff, ok = ComputeHotDiff(makeHotConfig(), newCfg)
	if !ok || !diff.Empty() {
		t.Fatalf("fakedns null vs absent must be an empty hot diff, ok=%v diff=%+v", ok, diff)
	}

	// A real DNS change still forces a restart — there is no reload API.
	newCfg = makeHotConfig()
	newCfg.DNSConfig = json_util.RawMessage(`{"servers":["8.8.8.8"]}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("real dns change must force a restart")
	}

	// Large integers keep full precision during normalization: two values
	// that only differ past float64 precision must still read as a change.
	oldCfg = makeHotConfig()
	oldCfg.Policy = json_util.RawMessage(`{"big":9007199254740993}`)
	newCfg = makeHotConfig()
	newCfg.Policy = json_util.RawMessage(`{"big":9007199254740992}`)
	if _, ok := ComputeHotDiff(oldCfg, newCfg); ok {
		t.Fatal("values differing past float64 precision must not compare equal")
	}

	// Reordered keys inside the first (default) outbound must not force a
	// restart — the form editor rebuilds the object on save.
	oldCfg = makeHotConfig()
	oldCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","settings":{"domainStrategy":"AsIs"},"tag":"direct"},{"protocol":"blackhole","tag":"blocked"}]`)
	newCfg = makeHotConfig()
	newCfg.OutboundConfigs = json_util.RawMessage(`[{"tag":"direct","settings":{"domainStrategy":"AsIs"},"protocol":"freedom"},{"protocol":"blackhole","tag":"blocked"}]`)
	diff, ok = ComputeHotDiff(oldCfg, newCfg)
	if !ok || !diff.Empty() {
		t.Fatalf("first outbound key reorder must be an empty hot diff, ok=%v diff=%+v", ok, diff)
	}
}

func TestComputeHotDiff_StaticSectionChangeNeedsRestart(t *testing.T) {
	newCfg := makeHotConfig()
	newCfg.LogConfig = json_util.RawMessage(`{"loglevel":"debug"}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("log change must force a restart")
	}

	newCfg = makeHotConfig()
	newCfg.DNSConfig = json_util.RawMessage(`{"servers":["1.1.1.1"]}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("dns change must force a restart")
	}

	newCfg = makeHotConfig()
	newCfg.Observatory = json_util.RawMessage(`{"subjectSelector":["wg"]}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("observatory change must force a restart")
	}

	newCfg = makeHotConfig()
	newCfg.Env = json_util.RawMessage(`{"XRAY_DNS_PATH":"/tmp/dns"}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("env change must force a restart: env vars are read only at process start")
	}
}

func TestComputeHotDiff_InboundAddRemoveChange(t *testing.T) {
	oldCfg := makeHotConfig()
	newCfg := makeHotConfig()
	// change existing beyond the clients list, so no user-level shortcut applies
	newCfg.InboundConfigs[1].Settings = json_util.RawMessage(`{"clients":[],"decryption":"none"}`)
	// add new
	newCfg.InboundConfigs = append(newCfg.InboundConfigs, InboundConfig{
		Port: 2080, Protocol: "vmess", Tag: "inbound-2080",
		Settings: json_util.RawMessage(`{}`),
	})

	diff, ok := ComputeHotDiff(oldCfg, newCfg)
	if !ok {
		t.Fatal("inbound-only change must be hot-appliable")
	}
	if len(diff.RemovedInboundTags) != 1 || diff.RemovedInboundTags[0] != "inbound-1080" {
		t.Fatalf("expected changed inbound to be removed, got %v", diff.RemovedInboundTags)
	}
	if len(diff.AddedInbounds) != 2 {
		t.Fatalf("expected re-add + new add, got %d", len(diff.AddedInbounds))
	}
	if diff.RoutingConfig != nil || len(diff.AddedOutbounds) != 0 || len(diff.RemovedOutboundTags) != 0 {
		t.Fatalf("unexpected non-inbound operations: %+v", diff)
	}
}

func TestComputeHotDiff_ClientOnlyChangeUsesUserOps(t *testing.T) {
	oldCfg := makeHotConfig()
	oldCfg.InboundConfigs[1].Settings = json_util.RawMessage(`{"clients":[{"email":"a","id":"uuid-a"},{"email":"b","id":"uuid-b"}],"decryption":"none"}`)
	newCfg := makeHotConfig()
	// b expired and is stripped from the generated config (#5712); a's id rotated.
	newCfg.InboundConfigs[1].Settings = json_util.RawMessage(`{"clients":[{"email":"a","id":"uuid-a2"},{"email":"c","id":"uuid-c"}],"decryption":"none"}`)

	diff, ok := ComputeHotDiff(oldCfg, newCfg)
	if !ok {
		t.Fatal("client-only change must be hot-appliable")
	}
	if len(diff.RemovedInboundTags) != 0 || len(diff.AddedInbounds) != 0 {
		t.Fatalf("client-only change must not replace the handler, got %+v", diff)
	}
	removed := map[string]bool{}
	for _, u := range diff.RemovedUsers {
		if u.Tag != "inbound-1080" || u.Protocol != "vless" {
			t.Fatalf("removed user op has wrong target: %+v", u)
		}
		removed[u.Email] = true
	}
	if len(removed) != 2 || !removed["a"] || !removed["b"] {
		t.Fatalf("expected users a (changed) and b (gone) removed, got %v", removed)
	}
	added := map[string]string{}
	for _, u := range diff.AddedUsers {
		id, _ := u.User["id"].(string)
		added[u.Email] = id
	}
	if len(added) != 2 || added["a"] != "uuid-a2" || added["c"] != "uuid-c" {
		t.Fatalf("expected users a (new id) and c added, got %v", added)
	}
}

func TestComputeHotDiff_ClientChangeFallsBackToReplace(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(cfg *Config)
	}{
		{
			name: "unsupported protocol",
			mutate: func(cfg *Config) {
				cfg.InboundConfigs[1].Protocol = "shadowsocks"
			},
		},
		{
			name: "client without email",
			mutate: func(cfg *Config) {
				cfg.InboundConfigs[1].Settings = json_util.RawMessage(`{"clients":[{"id":"uuid-a"}]}`)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldCfg := makeHotConfig()
			newCfg := makeHotConfig()
			tc.mutate(oldCfg)
			tc.mutate(newCfg)
			newCfg.InboundConfigs[1].Settings = json_util.RawMessage(`{"clients":[{"email":"x","id":"uuid-x","password":"pw"}]}`)

			diff, ok := ComputeHotDiff(oldCfg, newCfg)
			if !ok {
				t.Fatal("change must still be hot-appliable via handler replacement")
			}
			if len(diff.RemovedUsers) != 0 || len(diff.AddedUsers) != 0 {
				t.Fatalf("expected no user ops, got %+v", diff)
			}
			if len(diff.RemovedInboundTags) != 1 || len(diff.AddedInbounds) != 1 {
				t.Fatalf("expected handler replacement, got %+v", diff)
			}
		})
	}
}

func TestComputeHotDiff_ApiInboundChangeNeedsRestart(t *testing.T) {
	newCfg := makeHotConfig()
	newCfg.InboundConfigs[0].Port = 62790
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("api inbound change must force a restart")
	}
}

func TestComputeHotDiff_OutboundChangeAndReorder(t *testing.T) {
	oldCfg := makeHotConfig()
	newCfg := makeHotConfig()
	// change a non-first outbound + add one
	newCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"blackhole","settings":{},"tag":"blocked"},{"protocol":"socks","tag":"warp"}]`)

	diff, ok := ComputeHotDiff(oldCfg, newCfg)
	if !ok {
		t.Fatal("outbound-only change must be hot-appliable")
	}
	if len(diff.RemovedOutboundTags) != 1 || diff.RemovedOutboundTags[0] != "blocked" {
		t.Fatalf("expected changed outbound to be removed, got %v", diff.RemovedOutboundTags)
	}
	if len(diff.AddedOutbounds) != 2 {
		t.Fatalf("expected re-add + new add, got %d", len(diff.AddedOutbounds))
	}
	for _, raw := range diff.AddedOutbounds {
		if !strings.Contains(string(raw), `"tag"`) {
			t.Fatalf("added outbound JSON must be the raw element, got %s", raw)
		}
	}

	// pure reorder of non-first outbounds must be a no-op
	reordered := makeHotConfig()
	reordered.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"socks","tag":"warp"},{"protocol":"blackhole","tag":"blocked"}]`)
	base := makeHotConfig()
	base.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"blackhole","tag":"blocked"},{"protocol":"socks","tag":"warp"}]`)
	diff, ok = ComputeHotDiff(base, reordered)
	if !ok || !diff.Empty() {
		t.Fatalf("reorder of non-first outbounds must be an empty hot diff, ok=%v diff=%+v", ok, diff)
	}
}

func TestComputeHotDiff_FirstOutboundChangeNeedsRestart(t *testing.T) {
	newCfg := makeHotConfig()
	// change the default (first) outbound content
	newCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","settings":{"domainStrategy":"UseIP"},"tag":"direct"},{"protocol":"blackhole","tag":"blocked"}]`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("changing the default outbound must force a restart")
	}

	// swap which outbound comes first
	newCfg = makeHotConfig()
	newCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"blackhole","tag":"blocked"},{"protocol":"freedom","tag":"direct"}]`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("changing the first outbound must force a restart")
	}
}

func TestComputeHotDiff_TaglessOutboundNeedsRestart(t *testing.T) {
	newCfg := makeHotConfig()
	newCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"blackhole"}]`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("tagless outbound must force a restart")
	}
}

func TestComputeHotDiff_RoutingRulesChange(t *testing.T) {
	newCfg := makeHotConfig()
	newCfg.RouterConfig = json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"},{"type":"field","ip":["geoip:private"],"outboundTag":"blocked"}]}`)

	diff, ok := ComputeHotDiff(makeHotConfig(), newCfg)
	if !ok {
		t.Fatal("rules-only routing change must be hot-appliable")
	}
	if diff.RoutingConfig == nil {
		t.Fatal("routing diff must carry the new routing section")
	}

	// balancers are reloadable too
	newCfg = makeHotConfig()
	newCfg.RouterConfig = json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[],"balancers":[{"tag":"b1","selector":["wg"]}]}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); !ok {
		t.Fatal("balancer-only routing change must be hot-appliable")
	}
}

func TestComputeHotDiff_RoutingStrategyChangeNeedsRestart(t *testing.T) {
	newCfg := makeHotConfig()
	newCfg.RouterConfig = json_util.RawMessage(`{"domainStrategy":"IPIfNonMatch","rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"}]}`)
	if _, ok := ComputeHotDiff(makeHotConfig(), newCfg); ok {
		t.Fatal("domainStrategy change must force a restart")
	}
}
