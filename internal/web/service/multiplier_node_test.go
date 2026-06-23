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

// TestNodeSync_OldNodeBilledFallback verifies the version-skew degrade: an old
// node reports Real but no Billed (the field decodes to 0), and the master folds
// the Real delta as Billed (a neutral 1x) instead of leaving Billed at 0 forever
// — so Billed-based quota enforcement still works for node-only clients.
func TestNodeSync_OldNodeBilledFallback(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-old", 41011)
	svc := &InboundService{}
	const email = "olde"

	// Old node: Up/Down set, BilledUp/BilledDown absent (0). First sync seeds the baseline.
	syncNode(t, svc, 1, "n1-old", xray.ClientTraffic{Email: email, Up: 100, Down: 100, Enable: true})
	if ct := readTraffic(t, db, email); ct.Up+ct.Down != 0 || ct.BilledUp+ct.BilledDown != 0 {
		t.Fatalf("first sync should import nothing")
	}

	// Second sync grows Real by 50/50 with Billed still 0; the master bills it at 1x.
	syncNode(t, svc, 1, "n1-old", xray.ClientTraffic{Email: email, Up: 150, Down: 150, Enable: true})
	ct := readTraffic(t, db, email)
	if ct.Up != 50 || ct.Down != 50 {
		t.Errorf("real delta up=%d down=%d, want 50/50", ct.Up, ct.Down)
	}
	if ct.BilledUp != 50 || ct.BilledDown != 50 {
		t.Errorf("billed (1x fallback) up=%d down=%d, want 50/50", ct.BilledUp, ct.BilledDown)
	}
}
