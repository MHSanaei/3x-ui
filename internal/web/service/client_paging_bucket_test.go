package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The clients-page status buckets must classify on Billed (the figure quota
// enforcement and buildClientsSummary use), not Real — otherwise the filtered
// list contradicts the summary badge and the client's enforced state on any
// non-1x inbound.
func TestClientMatchesBucket_ClassifiesOnBilled(t *testing.T) {
	const gb = int64(1) << 30
	const nowMs, expireDiff, trafficDiff = int64(0), int64(0), int64(0)

	depleted := ClientWithAttachments{
		ClientRecord: model.ClientRecord{Email: "u@x", Enable: true, TotalGB: 100 * gb},
		Traffic: &xray.ClientTraffic{
			Up: 40 * gb, Down: 20 * gb, // Real 60GB
			BilledUp: 80 * gb, BilledDown: 40 * gb, // Billed 120GB >= 100GB
		},
	}
	if !clientMatchesBucket(depleted, "depleted", nil, nowMs, expireDiff, trafficDiff) {
		t.Error("Billed-exhausted client must land in the 'depleted' bucket")
	}
	if clientMatchesBucket(depleted, "active", nil, nowMs, expireDiff, trafficDiff) {
		t.Error("Billed-exhausted client must NOT be classified 'active'")
	}

	active := ClientWithAttachments{
		ClientRecord: model.ClientRecord{Email: "v@x", Enable: true, TotalGB: 100 * gb},
		Traffic: &xray.ClientTraffic{
			Up: 40 * gb, Down: 20 * gb,
			BilledUp: 40 * gb, BilledDown: 20 * gb, // Billed 60GB < 100GB
		},
	}
	if clientMatchesBucket(active, "depleted", nil, nowMs, expireDiff, trafficDiff) {
		t.Error("a client below its Billed quota must not be 'depleted'")
	}
	if !clientMatchesBucket(active, "active", nil, nowMs, expireDiff, trafficDiff) {
		t.Error("a client below its Billed quota must be 'active'")
	}
}

// The "remaining" sort key is quota-remaining (Total - Billed), so a client
// closer to its Billed limit must sort ahead of one with lower Real usage but
// more billed-remaining.
func TestSortClients_RemainingUsesBilled(t *testing.T) {
	const gb = int64(1) << 30
	a := ClientWithAttachments{ // 2x: 40GB Real = 80GB Billed -> 20GB remaining
		ClientRecord: model.ClientRecord{Email: "a", Enable: true, TotalGB: 100 * gb},
		Traffic:      &xray.ClientTraffic{Up: 30 * gb, Down: 10 * gb, BilledUp: 60 * gb, BilledDown: 20 * gb},
	}
	b := ClientWithAttachments{ // 1x: 60GB Real = 60GB Billed -> 40GB remaining
		ClientRecord: model.ClientRecord{Email: "b", Enable: true, TotalGB: 100 * gb},
		Traffic:      &xray.ClientTraffic{Up: 40 * gb, Down: 20 * gb, BilledUp: 40 * gb, BilledDown: 20 * gb},
	}
	rows := []ClientWithAttachments{b, a}
	sortClients(rows, "remaining", "ascend")
	if rows[0].Email != "a" {
		t.Errorf("remaining-asc order = [%s,%s], want 'a' first (least Billed remaining, despite lower Real usage)", rows[0].Email, rows[1].Email)
	}
}
