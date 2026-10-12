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
	return startBalancerTestXrayOnPort(t, spec, freeLoopbackPort(t))
}

// startBalancerTestXrayOnPort keeps the API port, as a panel restart does,
// so clients still holding the old core's port reach the new one.
func startBalancerTestXrayOnPort(t *testing.T, spec balancerTestConfig, apiPort int) (*xray.Process, int) {
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

// routeFor is the outbound the running core picks for a TCP connection to port.
func routeFor(process *xray.Process, port int) (string, error) {
	api := xray.XrayAPI{}
	if err := api.Init(process.GetAPIPort()); err != nil {
		return "", err
	}
	defer api.Close()
	result, err := api.TestRoute(xray.RouteTestRequest{IP: "203.0.113.1", Port: port})
	if err != nil {
		return "", err
	}
	return result.OutboundTag, nil
}

func liveRoute(t *testing.T, process *xray.Process, port int) string {
	t.Helper()
	route, err := routeFor(process, port)
	if err != nil {
		t.Fatalf("TestRoute(port %d): %v", port, err)
	}
	return route
}

// saveDuringTest hot-applies cfg the way RestartXray does, holding the package lock.
func saveDuringTest(t *testing.T, process *xray.Process, cfg *xray.Config) {
	t.Helper()
	lock.Lock()
	defer lock.Unlock()
	if !(&XrayService{}).tryHotApply(process, cfg) {
		t.Fatal("tryHotApply = false, want the change applied live")
	}
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

// A node probe can run for seconds; an override the operator sets meanwhile,
// onto an outbound saved meanwhile, must outlive the bridge's teardown.
func TestBalancerOverrideSetDuringNodeBridgeSurvivesTeardown(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	withProxyC := balancerTestConfig{outbounds: []string{"direct", "blocked", "proxy-a", "proxy-b", "proxy-c"}}.build(t, apiPort)

	(&NodeService{}).withOutboundBridge(1, "direct", func(proxyURL string) {
		if proxyURL == "" {
			t.Fatal("bridge was not built, so the teardown under test never runs")
		}
		saveDuringTest(t, process, withProxyC)
		if err := (&XrayService{}).OverrideBalancer("b1", "proxy-c"); err != nil {
			t.Fatalf("OverrideBalancer: %v", err)
		}
	})

	if got := liveBalancerOverride(t, process, "b1"); got != "proxy-c" {
		t.Fatalf("override after the node bridge was torn down = %q, want proxy-c", got)
	}
}

// The teardown used to restore the routing captured before the probe, silently
// reverting a rule change saved while the probe ran.
func TestNodeBridgeTeardownKeepsRoutingSavedDuringProbe(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	withPort8888Rule := balancerTestConfig{outbounds: allBalancerTestOutbounds, extraRule: true}.build(t, apiPort)

	(&NodeService{}).withOutboundBridge(1, "direct", func(proxyURL string) {
		if proxyURL == "" {
			t.Fatal("bridge was not built, so the teardown under test never runs")
		}
		saveDuringTest(t, process, withPort8888Rule)
	})

	if got := liveRoute(t, process, 8888); got != "blocked" {
		t.Fatalf("route for port 8888 after the node bridge was torn down = %q, want blocked", got)
	}
}

// Mid-apply, the core already runs the saved rules but its config snapshot is
// stale; a bridge opened then ran every connection on the stale rules.
func TestNodeBridgeOpenedDuringConfigApplyRoutesByTheAppliedConfig(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	withPort8888Rule := balancerTestConfig{outbounds: allBalancerTestOutbounds, extraRule: true}.build(t, apiPort)
	api := xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		t.Fatal(err)
	}
	defer api.Close()

	type probeView struct {
		proxyURL, route string
		err             error
	}
	seen, torndown := make(chan probeView, 1), make(chan struct{})
	lock.Lock()
	if err := api.ApplyRoutingConfig(withPort8888Rule.RouterConfig); err != nil {
		lock.Unlock()
		t.Fatalf("ApplyRoutingConfig: %v", err)
	}
	go func() {
		defer close(torndown)
		(&NodeService{}).withOutboundBridge(1, "direct", func(proxyURL string) {
			route, err := routeFor(process, 8888)
			seen <- probeView{proxyURL, route, err}
		})
	}()
	time.Sleep(300 * time.Millisecond) // room for an unserialized bridge to open mid-apply
	process.SetConfig(withPort8888Rule)
	lock.Unlock()
	view := <-seen
	<-torndown

	if view.proxyURL == "" {
		t.Fatal("bridge was not built, so the routing under test never ran")
	}
	if view.err != nil {
		t.Fatalf("TestRoute while the bridge was up: %v", view.err)
	}
	if view.route != "blocked" {
		t.Fatalf("route for port 8888 while the node bridge was up = %q, want blocked", view.route)
	}
}

// Mid-apply, the core already runs the saved rules but its config snapshot is
// stale; a teardown landing then restored the stale rules over the save.
func TestNodeBridgeTeardownDuringConfigApplyRestoresTheAppliedRouting(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	withPort8888Rule := balancerTestConfig{outbounds: allBalancerTestOutbounds, extraRule: true}.build(t, apiPort)
	api := xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		t.Fatal(err)
	}
	defer api.Close()

	probing, finishProbe, torndown := make(chan string), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(torndown)
		(&NodeService{}).withOutboundBridge(1, "direct", func(proxyURL string) {
			probing <- proxyURL
			<-finishProbe
		})
	}()
	if proxyURL := <-probing; proxyURL == "" {
		close(finishProbe)
		t.Fatal("bridge was not built, so the teardown under test never runs")
	}

	lock.Lock()
	if err := api.ApplyRoutingConfig(withPort8888Rule.RouterConfig); err != nil {
		lock.Unlock()
		close(finishProbe)
		t.Fatalf("ApplyRoutingConfig: %v", err)
	}
	close(finishProbe)
	time.Sleep(300 * time.Millisecond) // room for an unserialized teardown to land mid-apply
	process.SetConfig(withPort8888Rule)
	lock.Unlock()
	<-torndown

	if got := liveRoute(t, process, 8888); got != "blocked" {
		t.Fatalf("route for port 8888 after the node bridge was torn down = %q, want blocked", got)
	}
}

// A core restarted during the probe never had the bridge; the teardown must not
// push the old core's routing or overrides into it through the shared API port.
func TestNodeBridgeTeardownLeavesRestartedCoreAlone(t *testing.T) {
	first, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	if err := (&XrayService{}).OverrideBalancer("b1", "proxy-b"); err != nil {
		t.Fatalf("OverrideBalancer: %v", err)
	}

	var restarted *xray.Process
	(&NodeService{}).withOutboundBridge(1, "direct", func(proxyURL string) {
		if proxyURL == "" {
			t.Fatal("bridge was not built, so the teardown under test never runs")
		}
		lock.Lock()
		defer lock.Unlock()
		_ = first.Stop()
		restarted, _ = startBalancerTestXrayOnPort(t, balancerTestConfig{outbounds: allBalancerTestOutbounds, extraRule: true}, apiPort)
	})

	if got := liveRoute(t, restarted, 8888); got != "blocked" {
		t.Errorf("route for port 8888 on the restarted core = %q, want blocked from its own config", got)
	}
	if got := liveBalancerOverride(t, restarted, "b1"); got != "" {
		t.Errorf("override on the restarted core = %q, want none", got)
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

// Mid-apply, the core has already dropped an outbound its config snapshot still
// lists; an override landing then pinned the balancer to that missing tag.
func TestOverrideBalancerDuringConfigApplyChecksTheAppliedConfig(t *testing.T) {
	process, apiPort := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	withoutProxyB := balancerTestConfig{outbounds: []string{"direct", "blocked", "proxy-a"}}.build(t, apiPort)
	api := xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		t.Fatal(err)
	}
	defer api.Close()

	lock.Lock()
	if err := api.DelOutbound("proxy-b"); err != nil {
		lock.Unlock()
		t.Fatalf("DelOutbound: %v", err)
	}
	result := make(chan error, 1)
	go func() { result <- (&XrayService{}).OverrideBalancer("b1", "proxy-b") }()
	time.Sleep(300 * time.Millisecond) // room for an unserialized override to land mid-apply
	process.SetConfig(withoutProxyB)
	lock.Unlock()

	if err := <-result; !errors.Is(err, errOverrideTargetUnknown) {
		t.Errorf("OverrideBalancer(removed outbound) error = %v, want errOverrideTargetUnknown", err)
	}
	if got := liveBalancerOverride(t, process, "b1"); got != "" {
		t.Errorf("core override = %q, want none: proxy-b is gone from the core", got)
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
