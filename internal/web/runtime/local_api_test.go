package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The local core is reached over one gRPC connection. Opening one per
// operation made a bulk client delete pay a TCP + HTTP/2 handshake per client,
// so the client has to be pooled and kept open between calls.
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

func TestLocalWithAPIWithoutARunningCoreFails(t *testing.T) {
	local := NewLocal(LocalDeps{APIPort: func() int { return 0 }})
	if err := local.withAPI(func(*xray.XrayAPI) error { return nil }); err == nil {
		t.Fatal("want an error when the local core is not running")
	}
}
