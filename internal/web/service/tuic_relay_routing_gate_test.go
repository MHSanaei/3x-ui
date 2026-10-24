package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtls/xray-core/app/proxyman/command"
	routercommand "github.com/xtls/xray-core/app/router/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type delayedTuicRouting struct {
	routercommand.UnimplementedRoutingServiceServer
	client          routercommand.RoutingServiceClient
	entered, resume chan struct{}
	reject          atomic.Bool
}

func (s *delayedTuicRouting) AddRule(ctx context.Context, request *routercommand.AddRuleRequest) (*routercommand.AddRuleResponse, error) {
	if s.reject.Load() {
		return nil, status.Error(codes.Unavailable, "controlled routing failure")
	}
	close(s.entered)
	select {
	case <-s.resume:
		return s.client.AddRule(ctx, request)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestTuicNewAccountWaitsForItsRoutingPolicy(t *testing.T) {
	previous := isolationRelay(t, 960197, "old@test")
	before := isolationConfig(t, previous)
	api := startIsolationCore(t, before)
	if err := publishTuicRelayReadiness(before); err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", before.InboundConfigs[0].Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	routing := &delayedTuicRouting{client: routercommand.NewRoutingServiceClient(conn), entered: make(chan struct{}), resume: make(chan struct{})}
	ready := make(chan struct{})
	close(ready)
	command.RegisterHandlerServiceServer(server, &delayedTuicHandler{client: command.NewHandlerServiceClient(conn), entered: make(chan struct{}), resume: ready})
	routercommand.RegisterRoutingServiceServer(server, routing)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	// The proxy exposes the real core API while holding only the routing reload.
	snapshot := *before
	snapshot.InboundConfigs = append([]xray.InboundConfig(nil), before.InboundConfigs...)
	snapshot.InboundConfigs[0].Port = listener.Addr().(*net.TCPAddr).Port
	after := snapshot
	after.InboundConfigs = append([]xray.InboundConfig(nil), snapshot.InboundConfigs...)
	after.InboundConfigs[1] = isolationRelay(t, previous.TuicInboundID, "new@test")
	var router map[string]any
	if err := json.Unmarshal(before.RouterConfig, &router); err != nil {
		t.Fatal(err)
	}
	router["rules"] = append(router["rules"].([]any), map[string]any{"type": "field", "user": []string{"new@test"}, "outboundTag": "blocked"})
	after.RouterConfig = mustJSON(t, router)
	process := xray.NewTestProcess(&snapshot, filepath.Join(t.TempDir(), "config.json"))
	done := make(chan error, 1)
	completed := make(chan struct{})
	go func() { defer close(completed); _, err := (&XrayService{}).tryHotApply(process, &after); done <- err }()
	var release sync.Once
	resume := func() { release.Do(func() { close(routing.resume) }) }
	defer func() { resume(); <-completed }()
	select {
	case <-routing.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("routing reload was not reached")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	bridge := &tuic.SocksRelay{Gate: tuic.ManagedRelayGate(previous.Port), InboundID: previous.TuicInboundID, Addr: fmt.Sprintf("127.0.0.1:%d", previous.Port), Password: tuic.SocksPassword()}
	tunnel, err := bridge.DialTCP(ctx, "new@test", &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if tunnel != nil {
		_ = tunnel.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("new account was admitted before its routing policy: %v", err)
	}
	resume()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	result, err := api.TestRoute(xray.RouteTestRequest{Email: "new@test", IP: "203.0.113.1", Port: 6697})
	if err != nil || result.OutboundTag != "blocked" {
		t.Fatalf("new routing policy not installed: result=%+v err=%v", result, err)
	}
	routing.reject.Store(true)
	failed := after
	failed.InboundConfigs = append([]xray.InboundConfig(nil), after.InboundConfigs...)
	failed.InboundConfigs[1] = isolationRelay(t, previous.TuicInboundID, "failed@test")
	router["rules"] = append(router["rules"].([]any), map[string]any{"type": "field", "user": []string{"failed@test"}, "outboundTag": "blocked"})
	failed.RouterConfig = mustJSON(t, router)
	if applied, err := (&XrayService{}).tryHotApply(process, &failed); applied || err != nil {
		t.Fatalf("ordinary routing failure should request a restart fallback: applied=%t err=%v", applied, err)
	}
	if !process.GetConfig().Equals(&after) {
		t.Fatal("failed routing reload advanced the running snapshot")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	tunnel, err = bridge.DialTCP(ctx, "failed@test", &tuic.Address{Type: tuic.AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if tunnel != nil {
		_ = tunnel.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("routing failure left the new relay account admitted: %v", err)
	}
}
