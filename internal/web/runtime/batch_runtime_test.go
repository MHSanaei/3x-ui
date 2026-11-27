package runtime

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xtls/xray-core/app/proxyman/command"
	"google.golang.org/grpc"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type batchCaptureHandlerServer struct {
	command.UnimplementedHandlerServiceServer
	mu         sync.Mutex
	calls      []string
	delay      time.Duration
	inFlight   atomic.Int32
	peakFlight atomic.Int32
}

func (s *batchCaptureHandlerServer) AlterInbound(_ context.Context, req *command.AlterInboundRequest) (*command.AlterInboundResponse, error) {
	cur := s.inFlight.Add(1)
	for {
		prev := s.peakFlight.Load()
		if cur <= prev || s.peakFlight.CompareAndSwap(prev, cur) {
			break
		}
	}
	defer s.inFlight.Add(-1)

	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	s.mu.Lock()
	s.calls = append(s.calls, req.Tag+":"+req.Operation.Type)
	s.mu.Unlock()
	return &command.AlterInboundResponse{}, nil
}

func startBatchCaptureCore(t *testing.T, delay time.Duration) (int, *batchCaptureHandlerServer) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	handler := &batchCaptureHandlerServer{delay: delay}
	command.RegisterHandlerServiceServer(srv, handler)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)
	return listener.Addr().(*net.TCPAddr).Port, handler
}

func TestLocalUserBatchRuntimeConcurrentAddRemoveAndReplaceFlag(t *testing.T) {
	port, handler := startBatchCaptureCore(t, 15*time.Millisecond)
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})

	var _ UserBatchRuntime = local

	// Concurrent VLESS adds must multiplex over HTTP/2 (>1 in flight).
	vlessIb := &model.Inbound{Tag: "vless-test", Protocol: model.VLESS}
	ops := []UserAddOp{
		{User: map[string]any{"email": "u1@x", "id": "11111111-1111-1111-1111-111111111111"}, Replace: false},
		{User: map[string]any{"email": "u2@x", "id": "22222222-2222-2222-2222-222222222222"}, Replace: false},
		{User: map[string]any{"email": "u3@x", "id": "33333333-3333-3333-3333-333333333333"}, Replace: false},
		{User: map[string]any{"email": "u4@x", "id": "44444444-4444-4444-4444-444444444444"}, Replace: true},
	}
	addErrs := local.AddUsers(context.Background(), vlessIb, ops)
	for i, e := range addErrs {
		if e != nil {
			t.Fatalf("AddUsers[%d]: %v", i, e)
		}
	}
	if peak := handler.peakFlight.Load(); peak < 2 {
		t.Fatalf("peak concurrent AlterInbound RPCs = %d, want >= 2", peak)
	}

	rmErrs := local.RemoveUsers(context.Background(), vlessIb, []string{"u1@x", "u2@x"})
	for i, e := range rmErrs {
		if e != nil {
			t.Fatalf("RemoveUsers[%d]: %v", i, e)
		}
	}

	// Legacy Shadowsocks Replace=false skips pre-emptive RemoveUser; Replace=true keeps it.
	handler.mu.Lock()
	handler.calls = nil
	handler.mu.Unlock()

	ssIb := &model.Inbound{Tag: "ss-test", Protocol: model.Shadowsocks}
	ssOps := []UserAddOp{
		{User: map[string]any{"email": "ss-new@x", "password": "pw1", "cipher": "chacha20-ietf-poly1305"}, Replace: false},
		{User: map[string]any{"email": "ss-upd@x", "password": "pw2", "cipher": "chacha20-ietf-poly1305"}, Replace: true},
	}
	ssErrs := local.AddUsers(context.Background(), ssIb, ssOps)
	for i, e := range ssErrs {
		if e != nil {
			t.Fatalf("ss AddUsers[%d]: %v", i, e)
		}
	}
	handler.mu.Lock()
	ssCalls := append([]string(nil), handler.calls...)
	handler.mu.Unlock()
	adds, removes := 0, 0
	for _, c := range ssCalls {
		if strings.Contains(c, "AddUserOperation") {
			adds++
		}
		if strings.Contains(c, "RemoveUserOperation") {
			removes++
		}
	}
	if adds != 2 || removes != 1 {
		t.Fatalf("legacy SS batch RPCs = (adds=%d, removes=%d), want (2, 1); calls=%v", adds, removes, ssCalls)
	}
}
