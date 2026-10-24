package service

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Legacy JSON of either serialization style must remain discoverable after
// the supported ClientsTable startup migration (#5805, #6886).
func TestGetClientTrafficTgBot_SettingsSerializationStyles(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))

	db := database.GetDB()

	const tgId int64 = 123456789
	cases := []struct {
		name     string
		settings string
		email    string
		port     int
	}{
		{"compact", `{"clients":[{"id":"u1","email":"compact-user","tgId":123456789}]}`, "compact-user", 41001},
		{"spaced", `{"clients": [{"id": "u2", "email": "spaced-user", "tgId": 123456789}]}`, "spaced-user", 41002},
		{"indented", `{
			"clients": [
				{"id": "u3", "email": "indented-user", "tgId": 123456789}
			]
		}`, "indented-user", 41003},
	}
	for _, c := range cases {
		inbound := &model.Inbound{UserId: 1, Tag: "tg-" + c.name, Enable: true, Port: c.port, Protocol: model.VLESS, Settings: c.settings}
		if err := db.Create(inbound).Error; err != nil {
			t.Fatalf("create %s inbound: %v", c.name, err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: c.email, Enable: true, Up: 10, Down: 20}).Error; err != nil {
			t.Fatalf("create %s client_traffics: %v", c.name, err)
		}
		if c.name == "indented" {
			// The existing migration also repairs a missing link for an existing record.
			if err := db.Create(&model.ClientRecord{Email: c.email, UUID: "u3", TgID: tgId}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	// Model an upgrade from JSON-only storage, before the ClientsTable seeder.
	if err := db.Where("seeder_name = ?", "ClientsTable").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	var records []model.ClientRecord
	if err := database.GetDB().Where("tg_id = ?", tgId).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("ClientsTable migration created %d records, want 3", len(records))
	}

	svc := InboundService{}
	traffics, err := svc.GetClientTrafficTgBot(tgId)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot: %v", err)
	}
	got := make(map[string]bool, len(traffics))
	for _, tr := range traffics {
		got[tr.Email] = true
	}
	if len(traffics) != 3 || !got["compact-user"] || !got["spaced-user"] || !got["indented-user"] {
		t.Fatalf("expected traffic for compact-user, spaced-user and indented-user, got %v", got)
	}

	other, err := svc.GetClientTrafficTgBot(42)
	if err != nil {
		t.Fatalf("GetClientTrafficTgBot(42): %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("expected no traffic for unknown tgId, got %d rows", len(other))
	}
}

type tgLookupSQL struct {
	logger.Interface
	queries []string
}

func (l *tgLookupSQL) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.queries = append(l.queries, sql)
}

func TestGetClientTrafficTgBot_Indexed(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	testIndexedTgLookup(t, database.GetDB())
}

func TestGetClientTrafficTgBot_IndexedPostgres(t *testing.T) {
	testIndexedTgLookup(t, durablePostgresDB(t))
}

func TestGetClientTrafficTgBot_LargeAccount(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()
	inbound := &model.Inbound{UserId: 1, Tag: "large-tg", Port: 41888, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	const size = 1200
	records := make([]model.ClientRecord, size)
	traffics := make([]xray.ClientTraffic, size)
	links := make([]model.ClientInbound, size)
	for i := range size {
		email := fmt.Sprintf("large-%04d", i)
		records[i] = model.ClientRecord{Email: email, UUID: "uuid-" + email, SubID: "sub-" + email, TgID: 6886, Enable: true}
		traffics[i] = xray.ClientTraffic{Email: email, InboundId: inbound.Id, Up: int64(i)}
	}
	if err := db.CreateInBatches(&records, 100).Error; err != nil {
		t.Fatal(err)
	}
	for i := range size {
		links[i] = model.ClientInbound{ClientId: records[i].Id, InboundId: inbound.Id}
	}
	if err := db.CreateInBatches(&links, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.CreateInBatches(&traffics, 100).Error; err != nil {
		t.Fatal(err)
	}
	trace := &tgLookupSQL{Interface: logger.Default}
	previous := db.Logger
	db.Logger = trace
	t.Cleanup(func() { db.Logger = previous })
	got, err := (&InboundService{}).GetClientTrafficTgBot(6886)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != size || len(trace.queries) != 1 {
		t.Fatalf("got %d rows in %d queries, want %d rows in 1 query", len(got), len(trace.queries), size)
	}
	for i, traffic := range got {
		if traffic.Email != records[i].Email || traffic.UUID != records[i].UUID || traffic.SubId != records[i].SubID || !traffic.Enable || traffic.Up != int64(i) {
			t.Fatalf("row %d lost ordering or metadata: %+v", i, traffic)
		}
	}
}

func testIndexedTgLookup(t *testing.T, db *gorm.DB) {
	t.Helper()
	const tgID = int64(6886001)
	prefix := fmt.Sprintf("tg6886-%d-", time.Now().UnixNano())
	inbounds := []model.Inbound{
		{UserId: 1, Tag: prefix + "a", Port: 41886, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`},
		{UserId: 1, Tag: prefix + "b", Port: 41887, Protocol: model.VLESS, Settings: `{"clients":[]}`},
	}
	for i := range inbounds {
		if err := db.Create(&inbounds[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	var ids []int
	t.Cleanup(func() {
		_ = db.Where("client_id IN ?", ids).Delete(&model.ClientInbound{}).Error
		_ = db.Where("email LIKE ?", prefix+"%").Delete(&xray.ClientTraffic{}).Error
		_ = db.Where("id IN ?", ids).Delete(&model.ClientRecord{}).Error
		for i := range inbounds {
			_ = db.Delete(&inbounds[i]).Error
		}
	})
	for i, name := range []string{"a", "b", "no-traffic", "foreign", "unbound", "deleted-inbound", "zero"} {
		record := &model.ClientRecord{Email: prefix + name, UUID: "uuid-" + name, SubID: "sub-" + name, TgID: tgID, Enable: true}
		if name == "foreign" {
			record.TgID++
		}
		if name == "zero" {
			record.TgID = 0
		}
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, record.Id)
		if name == "b" {
			if err := db.Model(record).Update("enable", false).Error; err != nil {
				t.Fatal(err)
			}
		}
		if name != "unbound" {
			links := []model.ClientInbound{{ClientId: record.Id, InboundId: inbounds[0].Id}, {ClientId: record.Id, InboundId: inbounds[1].Id}}
			if name == "deleted-inbound" {
				links = []model.ClientInbound{{ClientId: record.Id, InboundId: -6886}}
			}
			if err := db.Create(&links).Error; err != nil {
				t.Fatal(err)
			}
		}
		if name != "no-traffic" {
			traffic := &xray.ClientTraffic{Email: record.Email, InboundId: -1, Enable: false, Up: int64(10 + i), Down: 20, Total: 500, ExpiryTime: 12345, Reset: 3, LastOnline: 6789}
			if err := db.Create(traffic).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	trace := &tgLookupSQL{Interface: logger.Default}
	previous := db.Logger
	db.Logger = trace
	t.Cleanup(func() { db.Logger = previous })
	svc := &InboundService{}
	for _, id := range []int64{tgID, tgID + 100, 0, -1} {
		trace.queries = nil
		got, err := svc.GetClientTrafficTgBot(id)
		if err != nil {
			t.Fatalf("lookup(%d): %v", id, err)
		}
		if id != tgID {
			if len(got) != 0 {
				t.Fatalf("lookup(%d) returned %v", id, got)
			}
		} else {
			var emails []string
			for _, traffic := range got {
				emails = append(emails, traffic.Email)
			}
			if !reflect.DeepEqual(emails, []string{prefix + "a", prefix + "b"}) {
				t.Fatalf("emails = %v", emails)
			}
			for i, traffic := range got {
				name := []string{"a", "b"}[i]
				if traffic.Enable != (i == 0) || traffic.UUID != "uuid-"+name || traffic.SubId != "sub-"+name || traffic.Up != int64(10+i) || traffic.Down != 20 || traffic.Total != 500 || traffic.ExpiryTime != 12345 || traffic.Reset != 3 || traffic.LastOnline != 6789 || traffic.InboundId != -1 {
					t.Fatalf("incorrect metadata/counters: %+v", traffic)
				}
			}
		}
		wantQueries := 1
		if id <= 0 {
			wantQueries = 0
		}
		if len(trace.queries) != wantQueries {
			t.Fatalf("lookup(%d) executed %d queries, want %d: %v", id, len(trace.queries), wantQueries, trace.queries)
		}
		for _, sql := range trace.queries {
			if strings.Contains(strings.ToLower(sql), "settings") || strings.Contains(strings.ToLower(sql), "json") {
				t.Fatalf("JSON scan in indexed path: %s", sql)
			}
		}
	}
}
