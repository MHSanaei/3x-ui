package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestAggregateClientTraffics verifies the per-attachment slice is collapsed back
// to one row per logical email (summing Real + Billed) for the external-inform
// webhook and the WS payload, with the inbound id cleared on the aggregate.
func TestAggregateClientTraffics(t *testing.T) {
	in := []*xray.ClientTraffic{
		{Email: "alice", InboundId: 1, Up: 10, Down: 20, BilledUp: 20, BilledDown: 40},
		{Email: "alice", InboundId: 2, Up: 5, Down: 5, BilledUp: 2, BilledDown: 2},
		{Email: "bob", InboundId: 1, Up: 100, Down: 0},
		nil,
	}
	out := aggregateClientTraffics(in)
	if len(out) != 2 {
		t.Fatalf("want 2 aggregated rows, got %d", len(out))
	}
	byEmail := map[string]*xray.ClientTraffic{}
	for _, ct := range out {
		byEmail[ct.Email] = ct
	}
	a := byEmail["alice"]
	if a == nil {
		t.Fatal("alice missing from aggregate")
	}
	if a.Up != 15 || a.Down != 25 || a.BilledUp != 22 || a.BilledDown != 42 {
		t.Errorf("alice = up%d down%d billedUp%d billedDown%d, want 15/25/22/42", a.Up, a.Down, a.BilledUp, a.BilledDown)
	}
	if a.InboundId != 0 {
		t.Errorf("aggregated row should clear InboundId, got %d", a.InboundId)
	}
	if b := byEmail["bob"]; b == nil || b.Up != 100 || b.Down != 0 {
		t.Errorf("bob = %+v, want up100 down0", b)
	}
}
