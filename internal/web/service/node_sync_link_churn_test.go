package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The node traffic poll runs every 5s and re-syncs every node inbound from its
// snapshot. A steady-state poll must not rewrite the inbound's whole membership
// set — that churn was the bulk of the write load in #6252.
func TestNodeSyncDoesNotChurnInboundLinks(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	emails := []string{"n1@x", "n2@x", "n3@x"}
	entries := make([]string, 0, len(emails))
	stats := make([]xray.ClientTraffic, 0, len(emails))
	for i, e := range emails {
		entries = append(entries, fmt.Sprintf(`{"email": %q, "enable": true}`, e))
		stats = append(stats, xray.ClientTraffic{Email: e, Up: int64(100 * (i + 1)), Down: 100, Enable: true})
	}
	settings := `{"clients": [` + strings.Join(entries, ",") + `]}`

	createNodeInbound(t, db, 1, "n1-in", 41101)
	syncNodeWithSettings(t, svc, 1, "n1-in", settings, stats...)

	var ib model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&ib).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	before := linksOf(t, ib.Id)
	if len(before) != len(emails) {
		t.Fatalf("link count after first sync = %d, want %d", len(before), len(emails))
	}
	stampLinkCreatedAt(t, ib.Id)

	// Second poll: identical client set, counters have grown.
	for i := range stats {
		stats[i].Up += 500
		stats[i].Down += 500
	}
	syncNodeWithSettings(t, svc, 1, "n1-in", settings, stats...)

	after := linksOf(t, ib.Id)
	if len(after) != len(before) {
		t.Fatalf("link count after second sync = %d, want %d", len(after), len(before))
	}
	for id, link := range after {
		if link.CreatedAt != 1 {
			t.Errorf("client %d: link created_at = %d, want the 1 sentinel: a steady-state node poll rebuilt the membership set",
				id, link.CreatedAt)
		}
	}

	// A client removed on the node must still lose its link, or the soft-orphan
	// sweep that reads this table would stop seeing remote deletions.
	shrunk := `{"clients": [` + strings.Join(entries[:2], ",") + `]}`
	syncNodeWithSettings(t, svc, 1, "n1-in", shrunk, stats[:2]...)
	pruned := linksOf(t, ib.Id)
	if len(pruned) != 2 {
		t.Fatalf("link count after shrink = %d, want 2", len(pruned))
	}
	if _, still := pruned[recordID(t, "n3@x")]; still {
		t.Error("n3@x link survived a snapshot that dropped it")
	}
}
