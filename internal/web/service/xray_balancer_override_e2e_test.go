package service

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Real-core tests: the bugs they pin live in how RoutingService rebuilds
// balancers, which no fake reproduces faithfully.

type balancerTestConfig struct {
	outbounds []string
	extraRule bool
}

func (c balancerTestConfig) build(t *testing.T, apiPort int) *xray.Config {
	t.Helper()
	outbounds := []any{}
	for _, tag := range c.outbounds {
		protocol := "freedom"
		if tag == "blocked" {
			protocol = "blackhole"
		}
		outbounds = append(outbounds, map[string]any{"tag": tag, "protocol": protocol, "settings": map[string]any{}})
	}
	rules := []any{
		map[string]any{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
		map[string]any{"type": "field", "port": "7777", "balancerTag": "b1"},
	}
	if c.extraRule {
		rules = append(rules, map[string]any{"type": "field", "port": "8888", "outboundTag": "blocked"})
	}
	routing := map[string]any{
		"domainStrategy": "AsIs",
		"rules":          rules,
		"balancers":      []any{map[string]any{"tag": "b1", "selector": []string{"proxy-"}}},
	}
	return &xray.Config{
		LogConfig:       mustJSON(t, map[string]any{"loglevel": "warning"}),
		RouterConfig:    mustJSON(t, routing),
		OutboundConfigs: mustJSON(t, outbounds),
		API:             mustJSON(t, map[string]any{"tag": "api", "services": []string{"HandlerService", "RoutingService"}}),
		InboundConfigs: []xray.InboundConfig{{
			Listen:   mustJSON(t, "127.0.0.1"),
			Port:     apiPort,
			Protocol: "tunnel",
			Settings: mustJSON(t, map[string]any{"rewriteAddress": "127.0.0.1"}),
			Tag:      "api",
		}},
	}
}

var allBalancerTestOutbounds = []string{"direct", "blocked", "proxy-a", "proxy-b"}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startBalancerTestXray runs a real core and installs it as the panel's
// current process, the way RestartXray does.
func startBalancerTestXray(t *testing.T, spec balancerTestConfig) (*xray.Process, int) {
	t.Helper()
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("set XRAY_E2E_BINARY to an xray built from go.mod's xray-core (go build github.com/xtls/xray-core/main)")
	}
	binDir := t.TempDir()
	if err := os.Symlink(bin, filepath.Join(binDir, xray.GetBinaryName())); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_BIN_FOLDER", binDir)

	apiPort := freeLoopbackPort(t)
	process := xray.NewTestProcess(spec.build(t, apiPort), filepath.Join(binDir, "config.json"))
	if err := process.Start(); err != nil {
		t.Fatalf("start xray: %v", err)
	}
	t.Cleanup(func() { _ = process.Stop() })

	previousProcess, previousResult := xrayState.snapshot()
	xrayState.replace(process)
	t.Cleanup(func() {
		xrayState.mu.Lock()
		xrayState.process = previousProcess
		xrayState.result = previousResult
		xrayState.mu.Unlock()
	})

	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := liveBalancerInfo(process, "b1"); err == nil {
			return process, apiPort
		} else if time.Now().After(deadline) {
			t.Fatalf("xray api never became ready: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func liveBalancerInfo(process *xray.Process, tag string) (*xray.BalancerInfo, error) {
	api := xray.XrayAPI{}
	if err := api.Init(process.GetAPIPort()); err != nil {
		return nil, err
	}
	defer api.Close()
	return api.GetBalancerInfo(tag)
}

func liveBalancerOverride(t *testing.T, process *xray.Process, tag string) string {
	t.Helper()
	info, err := liveBalancerInfo(process, tag)
	if err != nil {
		t.Fatalf("GetBalancerInfo(%s): %v", tag, err)
	}
	return info.Override
}

// Every routing save is hot-applied with AddRule(shouldAppend=false), which
// rebuilds all balancers and used to drop the operator's override silently.
func TestBalancerOverrideSurvivesRoutingHotApply(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	svc := &XrayService{}
	if err := svc.OverrideBalancer("b1", "proxy-b"); err != nil {
		t.Fatalf("OverrideBalancer: %v", err)
	}

	newCfg := balancerTestConfig{outbounds: allBalancerTestOutbounds, extraRule: true}.build(t, apiPort)
	if !svc.tryHotApply(process, newCfg) {
		t.Fatal("tryHotApply = false, want the routing change applied live")
	}

	if got := liveBalancerOverride(t, process, "b1"); got != "proxy-b" {
		t.Fatalf("override after routing hot apply = %q, want proxy-b", got)
	}
}

// An override naming a removed outbound makes the core close every connection
// the balancer routes ("non existing outTag"), so removing the target clears it.
func TestBalancerOverrideClearedWhenTargetOutboundRemoved(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	svc := &XrayService{}
	if err := svc.OverrideBalancer("b1", "proxy-b"); err != nil {
		t.Fatalf("OverrideBalancer: %v", err)
	}

	newCfg := balancerTestConfig{outbounds: []string{"direct", "blocked", "proxy-a"}}.build(t, apiPort)
	if !svc.tryHotApply(process, newCfg) {
		t.Fatal("tryHotApply = false, want the outbound removal applied live")
	}

	if got := liveBalancerOverride(t, process, "b1"); got != "" {
		t.Fatalf("override after its target was removed = %q, want cleared", got)
	}
}

// Testing a node through an outbound swaps the live routing in and back out;
// the override must hold while the probe runs and after the restore.
func TestBalancerOverrideSurvivesNodeOutboundBridge(t *testing.T) {
	process, _ := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	if err := (&XrayService{}).OverrideBalancer("b1", "proxy-b"); err != nil {
		t.Fatalf("OverrideBalancer: %v", err)
	}

	var duringProbe string
	(&NodeService{}).withOutboundBridge(1, "direct", func(proxyURL string) {
		if proxyURL == "" {
			t.Fatal("bridge was not built, so the routing swap under test never ran")
		}
		duringProbe = liveBalancerOverride(t, process, "b1")
	})

	if duringProbe != "proxy-b" {
		t.Errorf("override while the node bridge was up = %q, want proxy-b", duringProbe)
	}
	if got := liveBalancerOverride(t, process, "b1"); got != "proxy-b" {
		t.Errorf("override after the node bridge was torn down = %q, want proxy-b", got)
	}
}

// The core accepts any string as a target and then closes every connection the
// balancer routes, so the panel must refuse a target the running config lacks.
func TestOverrideBalancerRejectsUnknownTarget(t *testing.T) {
	process, _ := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})

	err := (&XrayService{}).OverrideBalancer("b1", "no-such-outbound")
	if !errors.Is(err, errOverrideTargetUnknown) {
		t.Fatalf("OverrideBalancer(unknown target) error = %v, want errOverrideTargetUnknown", err)
	}
	if got := liveBalancerOverride(t, process, "b1"); got != "" {
		t.Fatalf("core override after a refused target = %q, want none", got)
	}
}

// A restart is documented to clear overrides; a routing save on the new core
// must not resurrect one set on the previous process.
func TestBalancerOverrideNotCarriedAcrossRestart(t *testing.T) {
	_, _ = startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	if err := (&XrayService{}).OverrideBalancer("b1", "proxy-b"); err != nil {
		t.Fatalf("OverrideBalancer: %v", err)
	}

	restarted, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	newCfg := balancerTestConfig{outbounds: allBalancerTestOutbounds, extraRule: true}.build(t, apiPort)
	if !(&XrayService{}).tryHotApply(restarted, newCfg) {
		t.Fatal("tryHotApply = false, want the routing change applied live")
	}

	if got := liveBalancerOverride(t, restarted, "b1"); got != "" {
		t.Fatalf("override on the restarted core = %q, want none", got)
	}
}
