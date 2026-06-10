package xray

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestXrayAPI_E2E exercises the gRPC hot-apply surface (outbounds, inbounds,
// routing) against a real xray-core process. It validates the exact error
// texts IsMissingHandlerErr/IsExistingTagErr rely on, and that replacing the
// routing config keeps the api rule working.
//
// Skipped unless XRAY_E2E_BINARY points at an xray executable built from the
// same xray-core version as go.mod, e.g.:
//
//	go install github.com/xtls/xray-core/main@<version from go.mod>
//	XRAY_E2E_BINARY=$GOBIN/main go test ./internal/xray -run TestXrayAPI_E2E -v
func TestXrayAPI_E2E(t *testing.T) {
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("set XRAY_E2E_BINARY to an xray binary to run this test")
	}

	apiPort := freePort(t)
	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"api": map[string]any{
			"services": []string{"HandlerService", "StatsService", "RoutingService"},
			"tag":      "api",
		},
		"inbounds": []any{
			map[string]any{
				"listen":   "127.0.0.1",
				"port":     apiPort,
				"protocol": "tunnel",
				"settings": map[string]any{"rewriteAddress": "127.0.0.1"},
				"tag":      "api",
			},
		},
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "settings": map[string]any{}, "tag": "direct"},
			map[string]any{"protocol": "blackhole", "settings": map[string]any{}, "tag": "blocked"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
			},
		},
		"policy": map[string]any{},
		"stats":  map[string]any{},
	}
	cfgBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "-c", cfgPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start xray: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	waitForPort(t, apiPort)

	api := XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		t.Fatalf("api init: %v", err)
	}
	defer api.Close()

	// --- outbounds ---
	socksOutbound := []byte(`{"protocol":"socks","settings":{"servers":[{"address":"127.0.0.1","port":10808}]},"tag":"test-out"}`)
	if err := api.AddOutbound(socksOutbound); err != nil {
		t.Fatalf("AddOutbound: %v", err)
	}
	err = api.AddOutbound(socksOutbound)
	if err == nil {
		t.Fatal("duplicate AddOutbound must fail")
	}
	if !IsExistingTagErr(err) {
		t.Fatalf("duplicate AddOutbound error not matched by IsExistingTagErr: %q", err)
	}
	if err := api.DelOutbound("test-out"); err != nil {
		t.Fatalf("DelOutbound: %v", err)
	}
	// xray's outbound manager treats removal of an unknown tag as a no-op.
	if err := api.DelOutbound("test-out"); err != nil && !IsMissingHandlerErr(err) {
		t.Fatalf("removing a missing outbound: unexpected error %q", err)
	}

	// --- inbounds ---
	vlessPort := freePort(t)
	vlessInbound := fmt.Appendf(nil,
		`{"listen":"127.0.0.1","port":%d,"protocol":"vless","settings":{"clients":[{"id":"a17e367c-2074-4d3e-aaeb-fbef5dfde7e7","email":"e2e"}],"decryption":"none"},"tag":"test-in"}`,
		vlessPort)
	if err := api.AddInbound(vlessInbound); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	err = api.AddInbound(vlessInbound)
	if err == nil {
		t.Fatal("duplicate AddInbound must fail")
	}
	if !IsExistingTagErr(err) {
		t.Fatalf("duplicate AddInbound error not matched by IsExistingTagErr: %q", err)
	}
	if err := api.DelInbound("test-in"); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	err = api.DelInbound("test-in")
	if err == nil {
		t.Fatal("removing a missing inbound must fail")
	}
	if !IsMissingHandlerErr(err) {
		t.Fatalf("missing inbound error not matched by IsMissingHandlerErr: %q", err)
	}

	// --- routing (rules + balancers replace) ---
	newRouting := []byte(`{
		"domainStrategy": "AsIs",
		"balancers": [{"tag":"b1","selector":["direct"]}],
		"rules": [
			{"type":"field","inboundTag":["api"],"outboundTag":"api"},
			{"type":"field","port":"6666","outboundTag":"blocked","ruleTag":"e2e-rule"},
			{"type":"field","port":"7777","balancerTag":"b1","ruleTag":"e2e-balancer-rule"}
		]
	}`)
	if err := api.ApplyRoutingConfig(newRouting); err != nil {
		t.Fatalf("ApplyRoutingConfig: %v", err)
	}
	// The replaced rule set still contains the api rule — the gRPC channel
	// must keep working after the swap.
	if err := api.AddOutbound([]byte(`{"protocol":"blackhole","settings":{},"tag":"post-routing"}`)); err != nil {
		t.Fatalf("api unusable after routing replace (api rule lost?): %v", err)
	}
	if err := api.DelOutbound("post-routing"); err != nil {
		t.Fatalf("DelOutbound after routing replace: %v", err)
	}

	// --- route testing ---
	res, err := api.TestRoute(RouteTestRequest{IP: "1.2.3.4", Port: 6666, Network: "tcp"})
	if err != nil {
		t.Fatalf("TestRoute(port rule): %v", err)
	}
	if !res.Matched || res.OutboundTag != "blocked" {
		t.Fatalf("TestRoute(port rule) = %+v, want matched blocked", res)
	}
	res, err = api.TestRoute(RouteTestRequest{Domain: "example.com", Port: 7777, Network: "tcp"})
	if err != nil {
		t.Fatalf("TestRoute(balancer rule): %v", err)
	}
	if !res.Matched || res.OutboundTag != "direct" {
		t.Fatalf("TestRoute(balancer rule) = %+v, want matched direct", res)
	}
	// Note: current xray-core never populates OutboundGroupTags in PickRoute,
	// so GroupTags stays empty even for balancer rules — don't assert on it.
	res, err = api.TestRoute(RouteTestRequest{Domain: "example.com", Port: 9999, Network: "tcp"})
	if err != nil {
		t.Fatalf("TestRoute(no match): %v", err)
	}
	if res.Matched {
		t.Fatalf("TestRoute(no match) = %+v, want unmatched (default outbound)", res)
	}

	// --- balancer info + override ---
	info, err := api.GetBalancerInfo("b1")
	if err != nil {
		t.Fatalf("GetBalancerInfo: %v", err)
	}
	if info.Override != "" {
		t.Fatalf("fresh balancer must have no override, got %q", info.Override)
	}
	if err := api.SetBalancerTarget("b1", "blocked"); err != nil {
		t.Fatalf("SetBalancerTarget: %v", err)
	}
	info, err = api.GetBalancerInfo("b1")
	if err != nil {
		t.Fatalf("GetBalancerInfo after override: %v", err)
	}
	if info.Override != "blocked" {
		t.Fatalf("override = %q, want blocked", info.Override)
	}
	res, err = api.TestRoute(RouteTestRequest{Domain: "example.com", Port: 7777, Network: "tcp"})
	if err != nil {
		t.Fatalf("TestRoute(overridden balancer): %v", err)
	}
	if res.OutboundTag != "blocked" {
		t.Fatalf("overridden balancer must route to blocked, got %+v", res)
	}
	if err := api.SetBalancerTarget("b1", ""); err != nil {
		t.Fatalf("SetBalancerTarget(clear): %v", err)
	}
	info, err = api.GetBalancerInfo("b1")
	if err != nil {
		t.Fatalf("GetBalancerInfo after clear: %v", err)
	}
	if info.Override != "" {
		t.Fatalf("override after clear = %q, want empty", info.Override)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForPort(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("xray api port %d did not open in time", port)
}
