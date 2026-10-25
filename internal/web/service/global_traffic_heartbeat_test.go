package service

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func globalTrafficClock(t *testing.T, db *gorm.DB, queued bool) *atomic.Int64 {
	t.Helper()
	StopTrafficWriter()
	old := db.NowFunc
	clock := &atomic.Int64{}
	clock.Store(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli())
	db.NowFunc = func() time.Time { return time.UnixMilli(clock.Load()) }
	t.Cleanup(func() {
		StopTrafficWriter()
		db.NowFunc = old
	})
	if queued {
		StartTrafficWriter()
	}
	return clock
}

func countGlobalTrafficWrites(t *testing.T, db *gorm.DB) *atomic.Int64 {
	t.Helper()
	rows := &atomic.Int64{}
	const name = "test:global_traffic_affected_rows"
	if err := db.Callback().Create().After("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Error == nil && tx.Statement.Table == "client_global_traffics" {
			rows.Add(tx.RowsAffected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Create().Remove(name); err != nil {
			t.Error(err)
		}
	})
	return rows
}

func pushGlobalTraffic(t *testing.T, guid string, rows ...*xray.ClientTraffic) {
	t.Helper()
	if err := (&InboundService{}).AcceptGlobalTraffic(guid, rows); err != nil {
		t.Fatal(err)
	}
}

func globalTrafficRow(t *testing.T, db *gorm.DB, guid, email string) model.ClientGlobalTraffic {
	t.Helper()
	var row model.ClientGlobalTraffic
	if err := db.Where("master_guid = ? AND email = ?", guid, email).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestGlobalTrafficHeartbeatBounds(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, true)
	seedClientRow(t, "bounds", 1, 10, 20, 0)
	writes := countGlobalTrafficWrites(t, db)
	start := clock.Load()
	push := func(up, down, wantWrites int64) {
		t.Helper()
		writes.Store(0)
		pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "bounds", Up: up, Down: down})
		if got := writes.Load(); got != wantWrites {
			t.Fatalf("at %d counters %d/%d: affected rows=%d want %d", clock.Load()-start, up, down, got, wantWrites)
		}
		got := globalTrafficRow(t, db, "master", "bounds")
		if got.Up != up || got.Down != down {
			t.Fatalf("counters=%d/%d want %d/%d", got.Up, got.Down, up, down)
		}
	}
	push(100, 200, 1)
	clock.Store(start + time.Hour.Milliseconds() - 1)
	push(100, 200, 0)
	if got := globalTrafficRow(t, db, "master", "bounds").UpdatedAt; got != start {
		t.Fatalf("early heartbeat timestamp=%d want %d", got, start)
	}
	clock.Add(1)
	push(100, 200, 1)
	if got := globalTrafficRow(t, db, "master", "bounds").UpdatedAt; got != clock.Load() {
		t.Fatalf("boundary timestamp=%d want %d", got, clock.Load())
	}
	clock.Add(1)
	push(100, 200, 0)
	for _, pair := range [][2]int64{{101, 200}, {99, 200}, {99, 201}, {99, 199}, {0, 199}, {0, 0}} {
		push(pair[0], pair[1], 1)
	}
	local := readTraffic(t, db, "bounds")
	if local.Up != 10 || local.Down != 20 {
		t.Fatalf("local traffic changed: %+v", local)
	}
}

func TestGlobalTrafficHeartbeatNullableAndBatch(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, true)
	var payload []*xray.ClientTraffic
	for i := range 405 {
		email := fmt.Sprintf("batch-%03d", i)
		seedClientRow(t, email, 1, 0, 0, 0)
		payload = append(payload, &xray.ClientTraffic{Email: email, Up: 10, Down: 20})
	}
	pushGlobalTraffic(t, "master", payload[:404]...)
	clock.Add(time.Minute.Milliseconds())
	for i, column := range []string{"up", "down", "updated_at"} {
		if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", payload[i].Email).UpdateColumn(column, nil).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", payload[3].Email).UpdateColumn("updated_at", clock.Load()-time.Hour.Milliseconds()).Error; err != nil {
		t.Fatal(err)
	}
	payload[4].Up = 11
	payload = append(payload, nil, &xray.ClientTraffic{Email: "ghost", Up: 999}, &xray.ClientTraffic{Email: payload[5].Email, Up: 12, Down: 23})
	writes := countGlobalTrafficWrites(t, db)
	pushGlobalTraffic(t, "master", payload...)
	if got := writes.Load(); got != 7 {
		t.Fatalf("mixed two-full-batch payload affected %d rows want 7", got)
	}
	for i := range 6 {
		got := globalTrafficRow(t, db, "master", payload[i].Email)
		up, down := payload[i].Up, payload[i].Down
		if i == 5 {
			up, down = 12, 23
		}
		if got.Up != up || got.Down != down || got.UpdatedAt != clock.Load() {
			t.Fatalf("row %d=%+v want %d/%d at %d", i, got, up, down, clock.Load())
		}
	}
	var count int64
	if err := db.Model(&model.ClientGlobalTraffic{}).Count(&count).Error; err != nil || count != 405 {
		t.Fatalf("known rows=%d want 405 err=%v", count, err)
	}
}

func TestGlobalTrafficHeartbeatLongIdleAndRecovery(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, true)
	seedClientRow(t, "active", 1, 1, 2, 100)
	seedClientRow(t, "absent", 1, 1, 2, 100)
	ib := &model.Inbound{UserId: 1, Tag: "idle", Port: 43124, Enable: true, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&ClientService{}).SyncInbound(db, ib.Id, []model.Client{
		{Email: "active", Enable: true, TotalGB: 100}, {Email: "absent", Enable: true, TotalGB: 100},
	}); err != nil {
		t.Fatal(err)
	}
	start := clock.Load()
	pushGlobalTraffic(t, "active-master", &xray.ClientTraffic{Email: "active", Up: 80, Down: 70})
	pushGlobalTraffic(t, "absent-master", &xray.ClientTraffic{Email: "absent", Up: 80, Down: 70})
	writes := countGlobalTrafficWrites(t, db)
	for tick := 1; tick <= 5760; tick++ {
		clock.Store(start + int64(tick)*30_000)
		pushGlobalTraffic(t, "active-master", &xray.ClientTraffic{Email: "active", Up: 80, Down: 70})
	}
	if got := writes.Load(); got != 48 {
		t.Fatalf("48h idle pushes affected %d rows want 48", got)
	}
	assertUsage := func(email string, up, down int64, depleted bool) {
		t.Helper()
		rows := []*xray.ClientTraffic{{Email: email, Up: 1, Down: 2}}
		overlayGlobalTraffic(db, rows)
		if rows[0].Up != up || rows[0].Down != down {
			t.Fatalf("%s overlay=%d/%d want %d/%d", email, rows[0].Up, rows[0].Down, up, down)
		}
		cond, args := depletedCond(db)
		var count int64
		if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Where(cond, args...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if (count == 1) != depleted {
			t.Fatalf("%s depleted count=%d want depleted=%v", email, count, depleted)
		}
		page, err := (&ClientService{}).ListPaged(&InboundService{}, &SettingService{}, ClientPageParams{PageSize: 10, Search: email, Filter: "depleted"})
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if depleted {
			want = 1
		}
		if len(page.Items) != want || (want == 1 && page.Items[0].Email != email) {
			t.Fatalf("%s depleted page=%+v want %d rows", email, page.Items, want)
		}
	}
	assertUsage("active", 80, 70, true)
	assertUsage("absent", 1, 2, false)
	last := clock.Load()
	clock.Add(23 * time.Hour.Milliseconds())
	assertUsage("active", 80, 70, true)
	clock.Store(last + 24*time.Hour.Milliseconds())
	assertUsage("active", 80, 70, true)
	clock.Add(1)
	assertUsage("active", 1, 2, false)
	pushGlobalTraffic(t, "active-master", &xray.ClientTraffic{Email: "active", Up: 80, Down: 70})
	assertUsage("active", 80, 70, true)
}

func TestGlobalTrafficHeartbeatConcurrentPairs(t *testing.T) {
	for _, queued := range []bool{true, false} {
		t.Run(fmt.Sprintf("queued=%v", queued), func(t *testing.T) {
			db := initGlobalHeartbeatDB(t)
			globalTrafficClock(t, db, queued)
			seedClientRow(t, "concurrent", 1, 1, 2, 0)
			var wg sync.WaitGroup
			errs := make(chan error, 24)
			start := make(chan struct{})
			for i := range 24 {
				wg.Go(func() {
					<-start
					guid := "same"
					if i >= 12 {
						guid = fmt.Sprintf("master-%d", i)
					}
					errs <- (&InboundService{}).AcceptGlobalTraffic(guid, []*xray.ClientTraffic{{Email: "concurrent", Up: int64(i + 100), Down: int64(1000 - i)}})
				})
			}
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			row := globalTrafficRow(t, db, "same", "concurrent")
			if row.Up < 100 || row.Up > 111 || row.Up+row.Down != 1100 {
				t.Fatalf("torn submitted pair: %+v", row)
			}
			for i := 12; i < 24; i++ {
				got := globalTrafficRow(t, db, fmt.Sprintf("master-%d", i), "concurrent")
				if got.Up != int64(i+100) || got.Down != int64(1000-i) {
					t.Fatalf("master %d pair=%+v", i, got)
				}
			}
			pushGlobalTraffic(t, "same", &xray.ClientTraffic{Email: "concurrent", Up: 3, Down: 4})
			row = globalTrafficRow(t, db, "same", "concurrent")
			if row.Up != 3 || row.Down != 4 {
				t.Fatalf("ordered lower reset=%+v", row)
			}
			rows := []*xray.ClientTraffic{{Email: "concurrent", Up: 1, Down: 2}}
			overlayGlobalTraffic(db, rows)
			if rows[0].Up != 123 || rows[0].Down != 988 {
				t.Fatalf("componentwise max=%d/%d want 123/988", rows[0].Up, rows[0].Down)
			}
		})
	}
}

func TestGlobalTrafficHeartbeatQuotaAndLifecycle(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	globalTrafficClock(t, db, true)
	svc := &InboundService{}
	ib := &model.Inbound{
		UserId: 1, Tag: "quota", Port: 43123, Enable: true, Protocol: model.VLESS,
		Settings: `{"clients":[{"email":"quota","enable":true,"totalGB":1000}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	clients := &ClientService{}
	if err := clients.SyncInbound(db, ib.Id, []model.Client{{Email: "quota", Enable: true, TotalGB: 1000}}); err != nil {
		t.Fatal(err)
	}
	seedClientRow(t, "quota", ib.Id, 10, 20, 1000)
	invalidations := &atomic.Int64{}
	release := RegisterSubDataInvalidator(func() { invalidations.Add(1) })
	defer release()
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: 800, Down: 700})
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: 800, Down: 700})
	if invalidations.Load() != 0 {
		t.Fatalf("ingestion invalidated subscriptions %d times", invalidations.Load())
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, count, _, err := svc.disableInvalidClients(tx, newTrafficMutationBatch())
		if count != 1 {
			t.Errorf("disabled rows=%d want 1", count)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if invalidations.Load() != 1 {
		t.Fatalf("quota disable invalidations=%d want 1", invalidations.Load())
	}
	if err := db.First(ib, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Clients []struct {
			Email  string `json:"email"`
			Enable bool   `json:"enable"`
		} `json:"clients"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Clients) != 1 || settings.Clients[0].Email != "quota" || settings.Clients[0].Enable || readTraffic(t, db, "quota").Enable {
		t.Fatalf("disabled settings=%s traffic=%+v", ib.Settings, readTraffic(t, db, "quota"))
	}
	if err := svc.ResetClientTrafficByEmail("quota"); err != nil {
		t.Fatal(err)
	}
	assertRows := func(want int64) {
		t.Helper()
		var count int64
		if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", "quota").Count(&count).Error; err != nil || count != want {
			t.Fatalf("side rows=%d want %d err=%v", count, want, err)
		}
	}
	assertRows(0)
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: 0, Down: 0})
	assertRows(1)
	if _, err := clients.DeleteByEmail(svc, "quota", false); err != nil {
		t.Fatal(err)
	}
	assertRows(0)
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: 7, Down: 8})
	assertRows(0)
	if _, err := clients.CreateOne(svc, ib.Id, model.Client{Email: "quota", Enable: true}); err != nil {
		t.Fatal(err)
	}
	pushGlobalTraffic(t, "master", &xray.ClientTraffic{Email: "quota", Up: 7, Down: 8})
	assertRows(1)
	got := globalTrafficRow(t, db, "master", "quota")
	if got.Up != 7 || got.Down != 8 {
		t.Fatalf("recreated counters=%+v", got)
	}
}

func TestGlobalTrafficHeartbeatForwarding(t *testing.T) {
	db := initGlobalHeartbeatDB(t)
	clock := globalTrafficClock(t, db, true)
	seedClientRow(t, "forward", 1, 10, 20, 0)
	if err := db.Create(&model.NodeClientTraffic{NodeId: 7, Email: "forward"}).Error; err != nil {
		t.Fatal(err)
	}
	assertForward := func(up, down int64) {
		t.Helper()
		rows, err := (&InboundService{}).GetNodeClientTraffics(7)
		if err != nil || len(rows) != 1 || rows[0].Up != up || rows[0].Down != down {
			t.Fatalf("forward=%+v want %d/%d err=%v", rows, up, down, err)
		}
	}
	pushGlobalTraffic(t, "upstream", &xray.ClientTraffic{Email: "forward", Up: 900, Down: 800})
	writes := countGlobalTrafficWrites(t, db)
	clock.Add(time.Minute.Milliseconds())
	pushGlobalTraffic(t, "upstream", &xray.ClientTraffic{Email: "forward", Up: 900, Down: 800})
	assertForward(900, 800)
	if writes.Load() != 0 {
		t.Fatalf("unchanged forward push affected %d rows", writes.Load())
	}
	pushGlobalTraffic(t, "upstream", &xray.ClientTraffic{Email: "forward", Up: 50, Down: 60})
	assertForward(50, 60)
	pushGlobalTraffic(t, "upstream", &xray.ClientTraffic{Email: "forward", Up: 0, Down: 0})
	assertForward(10, 20)
}
