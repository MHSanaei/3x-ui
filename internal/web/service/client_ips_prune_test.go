package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Rows for clients absent from the online scan are never rewritten, so the
// sweep is the only thing standing between them and indefinite retention.
func TestPruneStaleClientIpsExpiresUnobservedRows(t *testing.T) {
	setupClientIpTestDB(t)
	db := database.GetDB()

	now := time.Now().Unix()
	stale := now - clientIpStaleAfterSeconds - 300
	fresh := now - 60

	mkNode := func(entries ...model.ClientIpEntry) string {
		t.Helper()
		b, err := json.Marshal(entries)
		if err != nil {
			t.Fatalf("marshal node ips: %v", err)
		}
		return string(b)
	}

	seed := []any{
		&model.InboundClientIps{ClientEmail: "offline", Ips: marshalIps(t, clientIpEntry{IP: "198.51.100.7", Timestamp: stale})},
		&model.InboundClientIps{ClientEmail: "mixed", Ips: marshalIps(t,
			clientIpEntry{IP: "198.51.100.8", Timestamp: stale},
			clientIpEntry{IP: "203.0.113.9", Timestamp: fresh})},
		&model.NodeClientIp{NodeGuid: "g1", Email: "node-offline", Ips: mkNode(model.ClientIpEntry{IP: "198.51.100.9", Timestamp: stale})},
		&model.NodeClientIp{NodeGuid: "g1", Email: "node-fresh", Ips: mkNode(model.ClientIpEntry{IP: "203.0.113.10", Timestamp: fresh})},
	}
	for _, row := range seed {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed %T: %v", row, err)
		}
	}

	if err := (&InboundService{}).PruneStaleClientIps(); err != nil {
		t.Fatalf("PruneStaleClientIps: %v", err)
	}

	if _, exists := readClientIps(t, "offline"); exists {
		t.Fatal("fully stale inbound_client_ips row must be deleted")
	}
	got, exists := readClientIps(t, "mixed")
	if !exists {
		t.Fatal("row with a fresh entry must survive")
	}
	if len(got) != 1 || got["203.0.113.9"] != fresh {
		t.Fatalf("mixed row = %v, want only 203.0.113.9@%d", got, fresh)
	}

	var nodeRows []model.NodeClientIp
	if err := db.Where("node_guid = ?", "g1").Find(&nodeRows).Error; err != nil {
		t.Fatalf("read node rows: %v", err)
	}
	if len(nodeRows) != 1 || nodeRows[0].Email != "node-fresh" {
		t.Fatalf("node rows after prune = %+v, want only node-fresh", nodeRows)
	}
	var kept []model.ClientIpEntry
	if err := json.Unmarshal([]byte(nodeRows[0].Ips), &kept); err != nil || len(kept) != 1 || kept[0].IP != "203.0.113.10" {
		t.Fatalf("node-fresh ips = %q (err %v), want 203.0.113.10 kept", nodeRows[0].Ips, err)
	}
}
