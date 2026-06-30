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

	got, found := clientWithFlow(clients, "b", "")
	if !found {
		t.Fatal("expected to find client b")
	}
	if got.Email != "b" || got.Flow != "" {
		t.Fatalf("override failed: got email=%q flow=%q, want b/empty", got.Email, got.Flow)
	}
	if clients[1].Flow != "xtls-rprx-vision" {
		t.Fatalf("source slice mutated: %q", clients[1].Flow)
	}

	if got2, found2 := clientWithFlow(clients, "a", "xtls-rprx-vision"); !found2 || got2.Flow != "xtls-rprx-vision" {
		t.Fatalf("set vision failed: found=%v flow=%q", found2, got2.Flow)
	}

	if _, found3 := clientWithFlow(clients, "missing", ""); found3 {
		t.Fatal("expected missing client to be not found")
	}
}
