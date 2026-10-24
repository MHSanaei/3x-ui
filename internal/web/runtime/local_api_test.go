package runtime

import (
	"context"
	"net"
	"strconv"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xtls/xray-core/app/proxyman/command"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A client per operation made a bulk delete pay a TCP + HTTP/2 handshake per client, so the local
// runtime's client must stay open and be reused between calls.
func TestLocalWithAPIKeepsTheClientOpen(t *testing.T) {
	port := 63211
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})

	var api *xray.XrayAPI
	if err := local.withAPI(func(a *xray.XrayAPI) error { api = a; return nil }); err != nil {
		t.Fatalf("withAPI: %v", err)
	}
	if api.HandlerServiceClient == nil {
		t.Fatal("the client was closed when the call returned; a pooled client must stay open")
	}
}

func TestLocalWithAPIReusesOneClient(t *testing.T) {
	port := 63212
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})

	clients := make([]*xray.XrayAPI, 0, 3)
	for range 3 {
		if err := local.withAPI(func(a *xray.XrayAPI) error {
			clients = append(clients, a)
			return nil
		}); err != nil {
			t.Fatalf("withAPI: %v", err)
		}
	}
	for i, c := range clients {
		if c != clients[0] {
			t.Fatalf("call %d got a different client; the local gRPC connection must be pooled", i)
		}
	}
}

// A core restart can move the API port, so the pool has to follow it and close
// the client it replaces instead of leaking it.
func TestLocalWithAPIEvictsTheClientWhenThePortChanges(t *testing.T) {
	port := 63213
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})

	var first *xray.XrayAPI
	if err := local.withAPI(func(a *xray.XrayAPI) error { first = a; return nil }); err != nil {
		t.Fatalf("withAPI: %v", err)
	}
	port = 63214
	var second *xray.XrayAPI
	if err := local.withAPI(func(a *xray.XrayAPI) error { second = a; return nil }); err != nil {
		t.Fatalf("withAPI after the port change: %v", err)
	}
	if first == second {
		t.Fatal("a changed API port must not reuse the old client")
	}
	if first.HandlerServiceClient != nil {
		t.Fatal("the evicted client must be closed, not leaked")
	}
}

type fakeHandlerServer struct {
	command.UnimplementedHandlerServiceServer
}

func (fakeHandlerServer) RemoveInbound(context.Context, *command.RemoveInboundRequest) (*command.RemoveInboundResponse, error) {
	return &command.RemoveInboundResponse{}, nil
}

// startFakeCore serves the core's handler service on port (0 picks a free one).
func startFakeCore(t *testing.T, port int) (int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("listen on %d: %v", port, err)
	}
	server := grpc.NewServer()
	command.RegisterHandlerServiceServer(server, fakeHandlerServer{})
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().(*net.TCPAddr).Port, server.Stop
}

// A pooled channel kept grpc-go's reconnect backoff after a core restart and failed fast for
// seconds; the first call after the restart must succeed, as it did with a client per call.
func TestLocalWithAPIRecoversAfterTheCoreRestarts(t *testing.T) {
	port, stop := startFakeCore(t, 0)
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})
	delInbound := func(api *xray.XrayAPI) error { return api.DelInbound("tag") }

	if err := local.withAPI(delInbound); err != nil {
		t.Fatalf("first call: %v", err)
	}
	stop()
	if err := local.withAPI(delInbound); status.Code(err) != codes.Unavailable {
		t.Fatalf("a call while the core is down: got %v, want Unavailable", err)
	}

	_, stopRestarted := startFakeCore(t, port)
	t.Cleanup(stopRestarted)
	if err := local.withAPI(delInbound); err != nil {
		t.Fatalf("the first call after the core restarted must succeed: %v", err)
	}
}
