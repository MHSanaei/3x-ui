package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientWithFlow(t *testing.T) {
	clients := []model.Client{
		{Email: "a", Flow: "xtls-rprx-vision"},
		{Email: "b", Flow: "xtls-rprx-vision"},
	}

	// The override use case: clear the flow on b only, on a flow-capable inbound,
	// without the capability clamp re-adding it.
	got, found := clientWithFlow(clients, "b", "")
	if !found {
		t.Fatal("expected to find client b")
	}
	if got.Email != "b" || got.Flow != "" {
		t.Fatalf("override failed: got email=%q flow=%q, want b/empty", got.Email, got.Flow)
	}
	// Returned value is a copy — the source slice is untouched.
	if clients[1].Flow != "xtls-rprx-vision" {
		t.Fatalf("source slice mutated: %q", clients[1].Flow)
	}

	// Setting Vision explicitly is preserved verbatim.
	if got2, found2 := clientWithFlow(clients, "a", "xtls-rprx-vision"); !found2 || got2.Flow != "xtls-rprx-vision" {
		t.Fatalf("set vision failed: found=%v flow=%q", found2, got2.Flow)
	}

	// A missing email is reported as not found.
	if _, found3 := clientWithFlow(clients, "missing", ""); found3 {
		t.Fatal("expected missing client to be not found")
	}
}
