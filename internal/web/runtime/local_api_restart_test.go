package runtime

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/xtls/xray-core/app/proxyman/command"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// fakeHandlerServer stands in for the core's gRPC handler service.
type fakeHandlerServer struct {
	command.UnimplementedHandlerServiceServer
}

func (fakeHandlerServer) RemoveInbound(context.Context, *command.RemoveInboundRequest) (*command.RemoveInboundResponse, error) {
	return &command.RemoveInboundResponse{}, nil
}

// startFakeCore starts a handler service on the port (0 picks a free one) and
// returns the port it listens on plus a stop function.
func startFakeCore(t *testing.T, port int) (int, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("listen on %d: %v", port, err)
	}
	server := grpc.NewServer()
	command.RegisterHandlerServiceServer(server, fakeHandlerServer{})
	go func() { _ = server.Serve(listener) }()
	return listener.Addr().(*net.TCPAddr).Port, func() {
		server.Stop()
		_ = listener.Close()
	}
}

// A pooled channel keeps grpc-go's reconnect backoff after the core went away,
// so it failed fast with Unavailable even once the core was listening again.
// The first call after a core restart has to succeed, the way it did when every
// call opened its own connection.
func TestLocalWithAPIRecoversAfterTheCoreRestarts(t *testing.T) {
	port, stop := startFakeCore(t, 0)
	local := NewLocal(LocalDeps{APIPort: func() int { return port }})

	if err := local.withAPI(func(api *xray.XrayAPI) error { return api.DelInbound("tag") }); err != nil {
		t.Fatalf("first call: %v", err)
	}

	stop()
	// The core stays down while calls keep arriving: grpc-go's reconnect
	// backoff grows (1s base, x1.6) and every call fails fast with
	// Unavailable.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := local.withAPI(func(api *xray.XrayAPI) error { return api.DelInbound("tag") }); err == nil {
			t.Fatal("the call must fail while the core is down")
		}
		time.Sleep(50 * time.Millisecond)
	}

	_, stopRestarted := startFakeCore(t, port)
	t.Cleanup(stopRestarted)

	if err := local.withAPI(func(api *xray.XrayAPI) error { return api.DelInbound("tag") }); err != nil {
		t.Fatalf("the first call after the core restarted must succeed: %v", err)
	}
}
