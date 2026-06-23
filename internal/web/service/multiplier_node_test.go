package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestNodeSync_FoldsBilled verifies the node->master fold carries Billed
// alongside Real: the node has already applied its inbound's multiplier, so the
// master folds the reported Billed delta as-is (never re-multiplying) into the
// per-client aggregate that quota enforcement reads.
func TestNodeSync_FoldsBilled(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41010)
	svc := &InboundService{}
	const email = "nbilled"

	// First sync seeds the (node,email) baseline; historical totals are not
	// imported. The node reports a 2x inbound: Billed = 2 x Real.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 500, Down: 600, BilledUp: 1000, BilledDown: 1200, Enable: true})
	ct := readTraffic(t, db, email)
	if ct.Up != 0 || ct.Down != 0 || ct.BilledUp != 0 || ct.BilledDown != 0 {
		t.Fatalf("first sync should import nothing, got real %d/%d billed %d/%d", ct.Up, ct.Down, ct.BilledUp, ct.BilledDown)
	}

	// Second sync: Real +200/+200, Billed +400/+400 (the node's 2x). The master
	// folds both deltas.
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Up: 700, Down: 800, BilledUp: 1400, BilledDown: 1600, Enable: true})
	ct = readTraffic(t, db, email)
	if ct.Up != 200 || ct.Down != 200 {
		t.Errorf("real delta: up=%d down=%d, want 200/200", ct.Up, ct.Down)
	}
	if ct.BilledUp != 400 || ct.BilledDown != 400 {
		t.Errorf("billed delta: billedUp=%d billedDown=%d, want 400/400", ct.BilledUp, ct.BilledDown)
	}
}
