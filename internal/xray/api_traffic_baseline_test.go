package xray

import (
	"testing"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

// GetTraffic advances its baseline as it reads. When the caller cannot persist
// the deltas (a failed database write), restoring the snapshot has to make the
// next poll report the same interval instead of losing it.
func TestRestoreStatsBaselineReReportsDeltas(t *testing.T) {
	api := startFakeStats(t, [][]*statsService.Stat{
		{stat("user>>>alice>>>traffic>>>uplink", 100)},
		{stat("user>>>alice>>>traffic>>>uplink", 180)},
		{stat("user>>>alice>>>traffic>>>uplink", 180)},
	})

	if _, _, err := api.GetTraffic(); err != nil {
		t.Fatalf("baseline poll: %v", err)
	}
	snapshot := api.SnapshotStatsBaseline()

	_, clients, err := api.GetTraffic()
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if got := clientTrafficByEmail(t, clients)["alice"].Up; got != 80 {
		t.Fatalf("first delta = %d, want 80", got)
	}

	api.RestoreStatsBaseline(snapshot)
	_, replayed, err := api.GetTraffic()
	if err != nil {
		t.Fatalf("replay poll: %v", err)
	}
	if got := clientTrafficByEmail(t, replayed)["alice"].Up; got != 80 {
		t.Fatalf("replayed delta = %d, want the lost 80 to be reported again", got)
	}
}
