package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// nodeSessionAt is a fixed node-clock instant; node merges compare only
// snapshot timestamps, so these tests never read the wall clock.
const nodeSessionAt = int64(1_900_000_000_000)

func mergeNodeSnapshot(t *testing.T, svc *InboundService, nodeID int, justPushed bool, inbounds ...*model.Inbound) {
	t.Helper()
	if _, err := svc.setRemoteTrafficLocked(nodeID, &runtime.TrafficSnapshot{Inbounds: inbounds}, false, justPushed); err != nil {
		t.Fatalf("setRemoteTrafficLocked node %d: %v", nodeID, err)
	}
}

func assertNodeSession(t *testing.T, ct xray.ClientTraffic, wantStart, wantUp, wantDown int64, when string) {
	t.Helper()
	if ct.SessionStart != wantStart || ct.SessionUp != wantUp || ct.SessionDown != wantDown {
		t.Errorf("%s: session = start %d, %d/%d; want start %d, %d/%d",
			when, ct.SessionStart, ct.SessionUp, ct.SessionDown, wantStart, wantUp, wantDown)
	}
}

// A node that tracks sessions decides their boundaries: the master merges it on
// its own schedule, often more than the 20s online window apart.
func TestNodeMergeTrustsNodeSessionStart(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	const email = "node-session"
	s1 := nodeSessionAt

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Enable: true, Up: 1000, Down: 2000,
		LastOnline: s1 + 5_000, SessionStart: s1, SessionUp: 40, SessionDown: 60,
	})
	assertNodeSession(t, readTraffic(t, db, email), s1, 40, 60, "first sight mirrors the session the node runs")

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Enable: true, Up: 1100, Down: 2300,
		LastOnline: s1 + 45_000, SessionStart: s1, SessionUp: 140, SessionDown: 360,
	})
	assertNodeSession(t, readTraffic(t, db, email), s1, 140, 360, "a 40s merge gap inside one node session")

	s2 := s1 + 600_000
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Enable: true, Up: 2300, Down: 3500,
		LastOnline: s2 + 5_000, SessionStart: s2, SessionUp: 30, SessionDown: 50,
	})
	assertNodeSession(t, readTraffic(t, db, email), s2, 30, 50, "a session the node opened while unmerged adopts its exact counters")
}

// Without the node's own session data the master can only read gaps in its
// lastOnline, but an idle client's repeated snapshots are no activity at all.
func TestNodeMergeOldNodeFallsBackToGapRule(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	const email = "old-node"
	l := nodeSessionAt
	sync := func(up, down, lastOnline int64) {
		t.Helper()
		syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{Email: email, Enable: true, Up: up, Down: down, LastOnline: lastOnline})
	}

	sync(1000, 2000, l)
	sync(1000, 2000, l)
	assertNodeSession(t, readTraffic(t, db, email), 0, 0, 0, "repeated snapshots of an idle client")

	sync(1010, 2020, l+5_000)
	assertNodeSession(t, readTraffic(t, db, email), l+5_000, 10, 20, "the first new activity")

	sync(1030, 2060, l+10_000)
	assertNodeSession(t, readTraffic(t, db, email), l+5_000, 30, 60, "activity within the online window")

	sync(1100, 2100, l+70_000)
	assertNodeSession(t, readTraffic(t, db, email), l+70_000, 70, 40, "activity after a quiet minute")
}

// A node's snapshot can carry divergent copies of one email across inbounds
// (#5274); the freshest copy must decide, whichever inbound comes first.
func TestNodeMergeFoldsMultiAttachedEmailOnce(t *testing.T) {
	const email = "multi-attached"
	l := nodeSessionAt
	stale := xray.ClientTraffic{Email: email, Enable: true, Up: 1010, Down: 2020, LastOnline: l + 5_000}
	fresh := xray.ClientTraffic{Email: email, Enable: true, Up: 1110, Down: 2070, LastOnline: l + 65_000}
	cases := []struct {
		name         string
		first, later xray.ClientTraffic
	}{
		{name: "stale copy first", first: stale, later: fresh},
		{name: "fresh copy first", first: fresh, later: stale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := initTrafficTestDB(t)
			createNodeInbound(t, db, 1, "n1-a", 41001)
			createNodeInbound(t, db, 1, "n1-b", 41002)
			svc := &InboundService{}
			both := func(a, b xray.ClientTraffic) {
				t.Helper()
				mergeNodeSnapshot(t, svc, 1, false,
					&model.Inbound{Tag: "n1-a", ClientStats: []xray.ClientTraffic{a}},
					&model.Inbound{Tag: "n1-b", ClientStats: []xray.ClientTraffic{b}})
			}
			seed := xray.ClientTraffic{Email: email, Enable: true, Up: 1000, Down: 2000, LastOnline: l}
			both(seed, seed)
			both(stale, stale)
			assertNodeSession(t, readTraffic(t, db, email), l+5_000, 10, 20, "session before the divergent snapshot")

			both(tc.first, tc.later)
			assertNodeSession(t, readTraffic(t, db, email), l+65_000, 100, 50, "after a quiet minute, folded from the freshest copy")
		})
	}
}

// A node-side renewal zeroes the node's counters mid-session: what it reports
// after belongs to the running session, though it is no delta over the baseline.
func TestNodeMergeRenewalCountsTheRenewedBytes(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-in", 41001)
	svc := &InboundService{}
	const email = "renew-session"
	s1 := nodeSessionAt

	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Enable: true, Up: 900, Down: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays,
		LastOnline: s1 + 5_000, SessionStart: s1, SessionUp: 40, SessionDown: 60,
	})
	syncNode(t, svc, 1, "n1-in", xray.ClientTraffic{
		Email: email, Enable: true, Up: 5, Down: 2, ExpiryTime: renewSecondExpiry, Reset: renewPeriodDays,
		LastOnline: s1 + 10_000, SessionStart: s1, SessionUp: 45, SessionDown: 62,
	})
	got := readTraffic(t, db, email)
	if got.ExpiryTime != renewSecondExpiry {
		t.Fatalf("expiry = %d, want %d: the renewal branch did not run", got.ExpiryTime, renewSecondExpiry)
	}
	assertNodeSession(t, got, s1, 45, 62, "a renewal inside the running session")
}

// The merge writes a client through one of three UPDATE statements; each must
// carry the session fold with its arguments in place.
func TestNodeMergeEveryUpdateBranchFoldsSession(t *testing.T) {
	const email = "node-branches"
	s1 := nodeSessionAt
	s2 := s1 + 600_000
	running := xray.ClientTraffic{
		Email: email, Enable: true, Up: 900, Down: 100, ExpiryTime: renewFirstExpiry, Reset: renewPeriodDays,
		LastOnline: s1 + 5_000, SessionStart: s1, SessionUp: 40, SessionDown: 60,
	}
	reopened := func(up, down, expiry int64) xray.ClientTraffic {
		return xray.ClientTraffic{
			Email: email, Enable: true, Up: up, Down: down, ExpiryTime: expiry, Reset: renewPeriodDays,
			LastOnline: s2 + 5_000, SessionStart: s2, SessionUp: 30, SessionDown: 50,
		}
	}
	cases := []struct {
		name       string
		next       xray.ClientTraffic
		justPushed bool
	}{
		{name: "normal merge", next: reopened(950, 150, renewFirstExpiry)},
		{name: "lifecycle frozen by a push", next: reopened(950, 150, renewFirstExpiry), justPushed: true},
		{name: "node-side renewal", next: reopened(5, 2, renewSecondExpiry)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := initTrafficTestDB(t)
			createNodeInbound(t, db, 1, "n1-in", 41001)
			svc := &InboundService{}
			mergeNodeSnapshot(t, svc, 1, false, &model.Inbound{Tag: "n1-in", ClientStats: []xray.ClientTraffic{running}})

			mergeNodeSnapshot(t, svc, 1, tc.justPushed, &model.Inbound{Tag: "n1-in", ClientStats: []xray.ClientTraffic{tc.next}})
			got := readTraffic(t, db, email)
			if tc.next.ExpiryTime == renewSecondExpiry && got.ExpiryTime != renewSecondExpiry {
				t.Fatalf("expiry = %d, want %d: the renewal branch did not run", got.ExpiryTime, renewSecondExpiry)
			}
			assertNodeSession(t, got, s2, 30, 50, tc.name)
		})
	}
}
