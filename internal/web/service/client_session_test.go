package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const sessionTestEmail = "session@x"

func seedTrafficRow(t *testing.T, db *gorm.DB, row xray.ClientTraffic) {
	t.Helper()
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed client_traffics %q: %v", row.Email, err)
	}
}

// assertSession checks the session columns: an opened session must start inside
// [before, after], a continued one must keep priorStart.
func assertSession(t *testing.T, got xray.ClientTraffic, opens bool, priorStart, before, after, wantUp, wantDown int64) {
	t.Helper()
	if opens {
		if got.SessionStart < before || got.SessionStart > after {
			t.Errorf("session_start = %d, want a new session opened within [%d, %d]", got.SessionStart, before, after)
		}
	} else if got.SessionStart != priorStart {
		t.Errorf("session_start = %d, want the running session's start %d kept", got.SessionStart, priorStart)
	}
	if got.SessionUp != wantUp || got.SessionDown != wantDown {
		t.Errorf("session traffic = %d/%d, want %d/%d", got.SessionUp, got.SessionDown, wantUp, wantDown)
	}
}

func TestAddClientTrafficFoldsSession(t *testing.T) {
	hourAgo := time.Now().Add(-time.Hour).UnixMilli()
	cases := []struct {
		name               string
		lastOnlineAgo      time.Duration
		start              int64
		up, down           int64
		deltaUp, deltaDown int64
		opens              bool
		wantUp, wantDown   int64
	}{
		{
			name: "continues the session the client is still in", lastOnlineAgo: 5 * time.Second,
			start: hourAgo, up: 100, down: 200, deltaUp: 10, deltaDown: 20, wantUp: 110, wantDown: 220,
		},
		{
			name: "opens a new session after a quiet window", lastOnlineAgo: time.Minute,
			start: hourAgo, up: 100, down: 200, deltaUp: 10, deltaDown: 20, opens: true, wantUp: 10, wantDown: 20,
		},
		{
			name: "opens the first session of a row tracked by no session yet", lastOnlineAgo: 5 * time.Second,
			up: 7, deltaUp: 10, deltaDown: 20, opens: true, wantUp: 10, wantDown: 20,
		},
		{
			name: "clamps a long session at the counter cap", lastOnlineAgo: 5 * time.Second,
			start: hourAgo, up: database.TrafficMax - 5, deltaUp: 10, wantUp: database.TrafficMax,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := initTrafficTestDB(t)
			seedTrafficRow(t, db, xray.ClientTraffic{
				Email: sessionTestEmail, Enable: true, LastOnline: time.Now().Add(-tc.lastOnlineAgo).UnixMilli(),
				SessionStart: tc.start, SessionUp: tc.up, SessionDown: tc.down,
			})

			before := time.Now().UnixMilli()
			delta := []*xray.ClientTraffic{{Email: sessionTestEmail, Up: tc.deltaUp, Down: tc.deltaDown}}
			if err := (&InboundService{}).addClientTraffic(db, delta); err != nil {
				t.Fatalf("addClientTraffic: %v", err)
			}
			after := time.Now().UnixMilli()

			assertSession(t, readTraffic(t, db, sessionTestEmail), tc.opens, tc.start, before, after, tc.wantUp, tc.wantDown)
		})
	}
}

// A live connection that moves no bytes keeps a client online, so the bump must
// decide the session too: else a reconnect's first bytes land in the old one.
func TestBumpClientsLastOnlineFoldsSession(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-time.Hour).UnixMilli()
	cases := []struct {
		name             string
		lastOnline       int64
		opens            bool
		wantUp, wantDown int64
	}{
		{name: "an idle connection keeps its session going", lastOnline: now.Add(-5 * time.Second).UnixMilli(), wantUp: 110, wantDown: 220},
		{name: "a reconnect after a quiet window opens an empty session", lastOnline: now.Add(-time.Minute).UnixMilli(), opens: true, wantUp: 10, wantDown: 20},
		{name: "a last_online ahead of this clock is not moved back", lastOnline: now.Add(time.Hour).UnixMilli(), wantUp: 110, wantDown: 220},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := initTrafficTestDB(t)
			seedTrafficRow(t, db, xray.ClientTraffic{
				Email: sessionTestEmail, Enable: true, LastOnline: tc.lastOnline,
				SessionStart: hourAgo, SessionUp: 100, SessionDown: 200,
			})
			svc := &InboundService{}

			before := time.Now().UnixMilli()
			if err := svc.BumpClientsLastOnline([]string{sessionTestEmail}); err != nil {
				t.Fatalf("BumpClientsLastOnline: %v", err)
			}
			after := time.Now().UnixMilli()
			bumped := readTraffic(t, db, sessionTestEmail)
			if want := max(tc.lastOnline, before); bumped.LastOnline < want || bumped.LastOnline > max(tc.lastOnline, after) {
				t.Errorf("last_online after bump = %d, want max(seeded %d, now)", bumped.LastOnline, tc.lastOnline)
			}

			// The same connection then moves bytes: they belong to the session the bump left.
			delta := []*xray.ClientTraffic{{Email: sessionTestEmail, Up: 10, Down: 20}}
			if err := svc.addClientTraffic(db, delta); err != nil {
				t.Fatalf("addClientTraffic: %v", err)
			}
			assertSession(t, readTraffic(t, db, sessionTestEmail), tc.opens, hourAgo, before, after, tc.wantUp, tc.wantDown)
		})
	}
}

// Resets renew the quota counters only: a session that spans a manual reset or
// an auto-renew still reports everything it moved.
func TestTrafficResetsKeepTheRunningSession(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour).UnixMilli()
	start := time.Now().Add(-time.Hour).UnixMilli()
	const id = "55555555-5555-5555-5555-555555555555"
	cases := []struct {
		name   string
		client model.Client
		row    xray.ClientTraffic
		reset  func(t *testing.T, svc *InboundService, db *gorm.DB, inboundID int)
	}{
		{
			name:   "manual reset",
			client: model.Client{Email: sessionTestEmail, ID: id, Enable: true},
			row:    xray.ClientTraffic{Enable: true},
			reset: func(t *testing.T, svc *InboundService, _ *gorm.DB, inboundID int) {
				if _, err := svc.ResetClientTraffic(inboundID, sessionTestEmail); err != nil {
					t.Fatalf("ResetClientTraffic: %v", err)
				}
			},
		},
		{
			name:   "auto-renew",
			client: model.Client{Email: sessionTestEmail, ID: id, Enable: false, Reset: 30, ExpiryTime: past},
			row:    xray.ClientTraffic{Enable: false, Reset: 30, ExpiryTime: past},
			reset: func(t *testing.T, svc *InboundService, db *gorm.DB, _ int) {
				if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil || count != 1 {
					t.Fatalf("autoRenewClients = %d renewed, %v; want 1, nil", count, err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			db := database.GetDB()
			svc := &InboundService{}
			ib := mkInbound(t, 30101, model.VLESS, clientsSettings(t, []model.Client{tc.client}))
			if err := svc.clientService.SyncInbound(nil, ib.Id, []model.Client{tc.client}); err != nil {
				t.Fatalf("SyncInbound: %v", err)
			}
			row := tc.row
			row.InboundId, row.Email, row.Up, row.Down = ib.Id, sessionTestEmail, 100, 200
			row.LastOnline = time.Now().Add(-5 * time.Second).UnixMilli()
			row.SessionStart, row.SessionUp, row.SessionDown = start, 30, 40
			seedTrafficRow(t, db, row)

			tc.reset(t, svc, db, ib.Id)

			got := readTraffic(t, db, sessionTestEmail)
			if got.Up != 0 || got.Down != 0 {
				t.Fatalf("usage after %s = %d/%d, want 0/0: the reset under test did not run", tc.name, got.Up, got.Down)
			}
			if got.SessionStart != start || got.SessionUp != 30 || got.SessionDown != 40 {
				t.Errorf("session after %s = start %d, %d/%d; want the running session (%d, 30/40) kept",
					tc.name, got.SessionStart, got.SessionUp, got.SessionDown, start)
			}
		})
	}
}
