package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The SQLite suite cannot catch fold SQL PostgreSQL rejects, such as SQLite's
// scalar MIN, or a CASE that misreads last_online's pre-update value there.
func TestClientSessionFold_Postgres(t *testing.T) {
	db := durablePostgresDB(t)
	svc := &InboundService{}
	const localEmail, nodeEmail = "pg-session-local", "pg-session-node"
	const nodeID = 990731
	const tag = "pg-session-node-in"
	cleanupDurableInboundFixtures(t, db, tag)
	cleanup := func() {
		_ = db.Where("email IN ?", []string{localEmail, nodeEmail}).Delete(&xray.ClientTraffic{}).Error
		_ = db.Where("node_id = ?", nodeID).Delete(&model.NodeClientTraffic{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	hourAgo := time.Now().Add(-time.Hour).UnixMilli()
	seedTrafficRow(t, db, xray.ClientTraffic{
		Email: localEmail, Enable: true, LastOnline: time.Now().Add(-time.Minute).UnixMilli(),
		SessionStart: hourAgo, SessionUp: 100, SessionDown: 200,
	})
	before := time.Now().UnixMilli()
	if err := svc.addClientTraffic(db, []*xray.ClientTraffic{{Email: localEmail, Up: 10, Down: 20}}); err != nil {
		t.Fatalf("addClientTraffic opening a session: %v", err)
	}
	after := time.Now().UnixMilli()
	opened := readTraffic(t, db, localEmail)
	assertSession(t, opened, true, hourAgo, before, after, 10, 20)

	if err := svc.addClientTraffic(db, []*xray.ClientTraffic{{Email: localEmail, Up: 1, Down: 2}}); err != nil {
		t.Fatalf("addClientTraffic continuing the session: %v", err)
	}
	if err := svc.BumpClientsLastOnline([]string{localEmail}); err != nil {
		t.Fatalf("BumpClientsLastOnline: %v", err)
	}
	assertSession(t, readTraffic(t, db, localEmail), false, opened.SessionStart, 0, 0, 11, 22)

	ib := durableTestInbound(new(int), tag, 25731)
	*ib.NodeID = nodeID
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}
	s1, s2 := nodeSessionAt, nodeSessionAt+600_000
	merge := func(ct xray.ClientTraffic) {
		t.Helper()
		ct.Email, ct.Enable = nodeEmail, true
		mergeNodeSnapshot(t, svc, nodeID, false, &model.Inbound{Tag: tag, ClientStats: []xray.ClientTraffic{ct}})
	}
	merge(xray.ClientTraffic{Up: 1000, Down: 2000, LastOnline: s1 + 5_000, SessionStart: s1, SessionUp: 40, SessionDown: 60})
	merge(xray.ClientTraffic{Up: 1100, Down: 2300, LastOnline: s1 + 45_000, SessionStart: s1, SessionUp: 140, SessionDown: 360})
	assertNodeSession(t, readTraffic(t, db, nodeEmail), s1, 140, 360, "node session continued")
	merge(xray.ClientTraffic{Up: 2300, Down: 3500, LastOnline: s2 + 5_000, SessionStart: s2, SessionUp: 30, SessionDown: 50})
	assertNodeSession(t, readTraffic(t, db, nodeEmail), s2, 30, 50, "node session reopened")
}
