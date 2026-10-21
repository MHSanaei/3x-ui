package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"

	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func isolationRelay(t *testing.T, id int, email string) xray.InboundConfig {
	t.Helper()
	return xray.InboundConfig{
		Tag: fmt.Sprintf("isolation-%d", id), Protocol: "socks", Listen: mustJSON(t, "127.0.0.1"),
		Port: tuic.SOCKSPortForInbound(id), TuicInboundID: id, TuicRelay: true,
		Settings: mustJSON(t, map[string]any{
			"auth": "password", "udp": true,
			"accounts": []relayAccount{{User: email, Pass: tuic.SocksPassword()}},
		}),
	}
}

func isolationConfig(t *testing.T, relays ...xray.InboundConfig) *xray.Config {
	t.Helper()
	cfg := (balancerTestConfig{outbounds: allBalancerTestOutbounds}).build(t, freeLoopbackPort(t))
	cfg.InboundConfigs = append(cfg.InboundConfigs, relays...)
	return cfg
}

func startIsolationCore(t *testing.T, cfg *xray.Config) *xray.XrayAPI {
	t.Helper()
	var config conf.Config
	if err := json.Unmarshal(mustJSON(t, cfg), &config); err != nil {
		t.Fatal(err)
	}
	built, err := config.Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := core.New(built)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = instance.Close() })
	if err := instance.Start(); err != nil {
		t.Fatal(err)
	}
	api := &xray.XrayAPI{}
	if err := api.Init(cfg.InboundConfigs[0].Port); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(api.Close)
	return api
}

func assertIsolationRelay(t *testing.T, relay xray.InboundConfig, email string) {
	t.Helper()
	marker := startTuicRouteMarker(t, "ready")
	_, port, _ := net.SplitHostPort(marker)
	number, _ := strconv.Atoi(port)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	bridge := &tuic.SocksRelay{
		InboundID: relay.TuicInboundID,
		Addr:      fmt.Sprintf("127.0.0.1:%d", relay.Port), Password: tuic.SocksPassword(),
	}
	if relay.TuicRelay {
		bridge.Gate = tuic.ManagedRelayGate(relay.Port)
	}
	conn, err := bridge.DialTCP(ctx, email, &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: uint16(number)})
	if err != nil {
		t.Fatalf("healthy relay %s rejected its client: %v", relay.Tag, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte(tuicRoutingProbe)); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len("ready:"+tuicRoutingProbe))
	if _, err := io.ReadFull(conn, response); err != nil || string(response) != "ready:"+tuicRoutingProbe {
		t.Fatalf("relay response = %q, err=%v", response, err)
	}
}

func TestTuicReadinessFailureDoesNotStrandLaterRelays(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	bad := isolationRelay(t, 960181, strings.Repeat("x", 256))
	good := isolationRelay(t, 960182, "new-after-restart@test")
	cfg := isolationConfig(t, bad, good)
	startIsolationCore(t, cfg)
	for _, warm := range []bool{false, true} {
		t.Run(fmt.Sprintf("warm=%t", warm), func(t *testing.T) {
			gate := tuic.ManagedRelayGate(good.Port)
			update, err := gate.BeginUpdate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			update.Finish(good.TuicInboundID, []string{"old-before-restart@test"}, warm)
			err = publishTuicRelayReadiness(cfg)
			if err == nil {
				t.Fatalf("invalid relay error = %v", err)
			}
			assertIsolationRelay(t, good, "new-after-restart@test")
			if !isNeedXrayReconcile.Load() {
				t.Fatal("failed readiness check did not schedule reconciliation")
			}
			gate.Invalidate(good.TuicInboundID)
			if err := repairUnavailableTuicRelays(cfg, cfg.InboundConfigs[0].Port); err == nil {
				t.Fatal("repair hid the invalid relay")
			}
			assertIsolationRelay(t, good, "new-after-restart@test")
		})
	}
}

func TestTuicMissingRelayDoesNotDelayHealthyReadiness(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	missing, healthy := isolationRelay(t, 960189, "missing@test"), isolationRelay(t, 960190, "ready@test")
	cfg := isolationConfig(t, healthy)
	startIsolationCore(t, cfg)
	wanted := *cfg
	wanted.InboundConfigs = []xray.InboundConfig{cfg.InboundConfigs[0], missing, healthy}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	done := make(chan error, 1)
	go func() { done <- publishTuicRelayBatch(ctx, &wanted) }()
	defer func() { cancel(); <-done }()
	assertIsolationRelay(t, healthy, "ready@test")
}

func TestTuicRestartInvalidatesOldAccountsEvenWhenVerificationCannotStart(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	healthy := isolationRelay(t, 960196, "new@test")
	cfg := isolationConfig(t, healthy)
	startIsolationCore(t, cfg)
	gate := tuic.ManagedRelayGate(healthy.Port)
	update, err := gate.BeginUpdate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	update.Finish(healthy.TuicInboundID, []string{"old@test"}, true)
	wanted := *cfg
	wanted.InboundConfigs = []xray.InboundConfig{cfg.InboundConfigs[0]}
	for id := 960192; id <= 960195; id++ {
		wanted.InboundConfigs = append(wanted.InboundConfigs, isolationRelay(t, id, "missing@test"))
	}
	wanted.InboundConfigs = append(wanted.InboundConfigs, healthy)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := publishTuicRelayBatch(ctx, &wanted); err == nil {
		t.Fatal("missing relays were published as ready")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	bridge := &tuic.SocksRelay{
		Gate: gate, InboundID: healthy.TuicInboundID,
		Addr: fmt.Sprintf("127.0.0.1:%d", healthy.Port), Password: tuic.SocksPassword(),
	}
	conn, err := bridge.DialTCP(ctx, "old@test", &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if conn != nil {
		_ = conn.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stale account crossed the gate after restart: %v", err)
	}
}

func TestTuicFailedSwapDoesNotBlockOtherRelayOrRouting(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	first, second := isolationRelay(t, 960183, "first@test"), isolationRelay(t, 960184, "second@test")
	before := isolationConfig(t, first, second)
	api := startIsolationCore(t, before)
	if err := publishTuicRelayReadiness(before); err != nil {
		t.Fatal(err)
	}
	after := *before
	after.InboundConfigs = append([]xray.InboundConfig(nil), before.InboundConfigs...)
	after.InboundConfigs[1] = isolationRelay(t, first.TuicInboundID, strings.Repeat("x", 256))
	after.InboundConfigs[2] = isolationRelay(t, second.TuicInboundID, "updated@test")
	var router map[string]any
	if err := json.Unmarshal(before.RouterConfig, &router); err != nil {
		t.Fatal(err)
	}
	router["rules"] = append(router["rules"].([]any), map[string]any{"type": "field", "port": "9999", "outboundTag": "blocked"})
	after.RouterConfig = mustJSON(t, router)
	process := xray.NewTestProcess(before, filepath.Join(t.TempDir(), "config.json"))
	applied, err := (&XrayService{}).tryHotApply(process, &after)
	if applied || err == nil || !strings.Contains(err.Error(), "SOCKS5 bounds") {
		t.Fatalf("partial apply did not report its deferred relay: applied=%t err=%v", applied, err)
	}
	route, err := api.TestRoute(xray.RouteTestRequest{IP: "203.0.113.1", Port: 9999, Network: "tcp"})
	if err != nil || route.OutboundTag != "blocked" {
		t.Fatalf("failed TUIC relay prevented unrelated routing update: route=%+v err=%v", route, err)
	}
	assertIsolationRelay(t, first, "first@test")
	assertIsolationRelay(t, second, "updated@test")
	if !process.GetConfig().InboundConfigs[1].Equals(&first) || !process.GetConfig().InboundConfigs[2].Equals(&after.InboundConfigs[2]) || string(process.GetConfig().RouterConfig) != string(after.RouterConfig) {
		t.Fatal("running snapshot lost successful updates or recorded the failed relay as applied")
	}
}

func TestTuicRelayTagSwapKeepsBothOwnersReachable(t *testing.T) {
	first, second := isolationRelay(t, 960185, "first@test"), isolationRelay(t, 960186, "second@test")
	before := isolationConfig(t, first, second)
	api := startIsolationCore(t, before)
	if err := publishTuicRelayReadiness(before); err != nil {
		t.Fatal(err)
	}
	after := *before
	after.InboundConfigs = append([]xray.InboundConfig(nil), before.InboundConfigs...)
	after.InboundConfigs[1].Tag, after.InboundConfigs[2].Tag = second.Tag, first.Tag
	diff, hot := xray.ComputeHotDiff(before, &after)
	if !hot {
		t.Fatal("relay tag swap cannot be hot applied")
	}
	if err := applyTuicRelayReplacements(api, diff.TuicRelayChanges); err != nil {
		t.Fatalf("tag swap collided with a listener in its own transaction: %v", err)
	}
	assertIsolationRelay(t, after.InboundConfigs[1], "first@test")
	assertIsolationRelay(t, after.InboundConfigs[2], "second@test")
}

func TestTuicFailedRenameDefersDependentRoutingButAddsOtherInbound(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	previous := isolationRelay(t, 960187, "original@test")
	second := isolationRelay(t, 960198, "old-second@test")
	before := isolationConfig(t, previous, second)
	api := startIsolationCore(t, before)
	if err := publishTuicRelayReadiness(before); err != nil {
		t.Fatal(err)
	}
	after := *before
	next := isolationRelay(t, previous.TuicInboundID, strings.Repeat("x", 256))
	next.Tag = "renamed-unavailable"
	ordinary := isolationRelay(t, 960188, "other@test")
	ordinary.TuicRelay = false
	ordinary.TuicInboundID = 0
	ordinary.Port = freeLoopbackPort(t)
	ordinary.Settings = mustJSON(t, map[string]any{"auth": "noauth", "udp": true})
	dependent := ordinary
	dependent.Tag = "dependent-ordinary"
	dependent.Port = freeLoopbackPort(t)
	updatedSecond := isolationRelay(t, second.TuicInboundID, "new-second@test")
	after.InboundConfigs = []xray.InboundConfig{before.InboundConfigs[0], next, updatedSecond, ordinary, dependent}
	var policy map[string]any
	if err := json.Unmarshal(before.RouterConfig, &policy); err != nil {
		t.Fatal(err)
	}
	policy["rules"] = append(policy["rules"].([]any),
		map[string]any{"type": "field", "inboundTag": []string{next.Tag}, "outboundTag": "blocked"},
		map[string]any{"type": "field", "inboundTag": []string{dependent.Tag}, "outboundTag": "blocked"},
		map[string]any{"type": "field", "inboundTag": []string{second.Tag}, "outboundTag": "blocked"},
	)
	after.RouterConfig = mustJSON(t, policy)
	process := xray.NewTestProcess(before, filepath.Join(t.TempDir(), "config.json"))
	if applied, err := (&XrayService{}).tryHotApply(process, &after); applied || err == nil || !strings.Contains(err.Error(), "SOCKS5 bounds") {
		t.Fatalf("failed rename result = %t, %v", applied, err)
	}
	assertIsolationRelay(t, previous, "original@test")
	assertIsolationRelay(t, second, "old-second@test")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	secondBridge := &tuic.SocksRelay{
		Gate: tuic.ManagedRelayGate(second.Port), InboundID: second.TuicInboundID,
		Addr: fmt.Sprintf("127.0.0.1:%d", second.Port), Password: tuic.SocksPassword(),
	}
	pending, err := secondBridge.DialTCP(ctx, "new-second@test", &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if pending != nil {
		_ = pending.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("new TUIC account escaped its deferred routing policy: %v", err)
	}
	assertIsolationRelay(t, ordinary, "other@test")
	reservation, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", dependent.Port))
	if err != nil {
		t.Fatalf("new inbound started before its dependent routing was applied: %v", err)
	}
	_ = reservation.Close()
	route, err := api.TestRoute(xray.RouteTestRequest{InboundTag: next.Tag, IP: "203.0.113.1", Port: 6697})
	if err != nil || route.Matched {
		t.Fatalf("routing depending on an uninstalled relay was published: route=%+v err=%v", route, err)
	}
	if string(process.GetConfig().RouterConfig) != string(before.RouterConfig) {
		t.Fatal("snapshot recorded deferred routing as applied")
	}
	after.InboundConfigs[1] = previous
	after.InboundConfigs[1].Tag = next.Tag
	if applied, err := (&XrayService{}).tryHotApply(process, &after); !applied || err != nil {
		t.Fatalf("corrected rename did not converge: applied=%t err=%v", applied, err)
	}
	route, err = api.TestRoute(xray.RouteTestRequest{InboundTag: next.Tag, IP: "203.0.113.1", Port: 6697})
	if err != nil || route.OutboundTag != "blocked" || !process.GetConfig().Equals(&after) {
		t.Fatalf("deferred routing did not converge after repair: route=%+v err=%v", route, err)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", dependent.Port), time.Second)
	if err != nil {
		t.Fatalf("dependent inbound did not converge after routing recovery: %v", err)
	}
	_ = conn.Close()
}

func TestTuicFailedRenameStillAppliesUnrelatedRouting(t *testing.T) {
	wasPending := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() { isNeedXrayReconcile.Store(wasPending) })
	previous := isolationRelay(t, 960191, "original@test")
	before := isolationConfig(t, previous)
	api := startIsolationCore(t, before)
	if err := publishTuicRelayReadiness(before); err != nil {
		t.Fatal(err)
	}
	after := *before
	next := isolationRelay(t, previous.TuicInboundID, strings.Repeat("x", 256))
	next.Tag = "renamed-failed"
	after.InboundConfigs = []xray.InboundConfig{before.InboundConfigs[0], next}
	var router map[string]any
	if err := json.Unmarshal(before.RouterConfig, &router); err != nil {
		t.Fatal(err)
	}
	router["rules"] = append(router["rules"].([]any), map[string]any{"type": "field", "port": "9999", "outboundTag": "blocked"})
	after.RouterConfig = mustJSON(t, router)
	process := xray.NewTestProcess(before, filepath.Join(t.TempDir(), "config.json"))
	if _, err := (&XrayService{}).tryHotApply(process, &after); err == nil || !strings.Contains(err.Error(), "SOCKS5 bounds") {
		t.Fatalf("failed rename was not reported: %v", err)
	}
	route, err := api.TestRoute(xray.RouteTestRequest{IP: "203.0.113.1", Port: 9999})
	if err != nil || route.OutboundTag != "blocked" {
		t.Fatalf("failed rename blocked an unrelated rule: route=%+v err=%v", route, err)
	}
}
