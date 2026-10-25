package service

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedStatsIdentity(t testing.TB, inboundIDs []int, email string) model.ClientRecord {
	t.Helper()
	db := database.GetDB()
	rec := model.ClientRecord{Email: email, UUID: "uuid-" + email, SubID: "sub-" + email, Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range inboundIDs {
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: id}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return rec
}

func TestClientStatsIdentitiesBatchesPreloadedInbounds(t *testing.T) {
	setupBulkDB(t)
	var inbounds []*model.Inbound
	for i := range 8 {
		ib := mkInbound(t, 57000+i, model.VLESS, `{"clients":[]}`)
		rec := seedStatsIdentity(t, []int{ib.Id}, fmt.Sprintf("batch-%d", i))
		ib.ClientStats = []xray.ClientTraffic{{Email: rec.Email, Up: 123, Down: 456, UUID: "stale", SubId: "stale"}}
		inbounds = append(inbounds, ib)
	}
	queries := countClientTableQueries(t, "stats_batch", func() { (&InboundService{}).enrichClientStats(database.GetDB(), inbounds) })
	if queries != 1 {
		t.Fatalf("clients queries = %d, want 1 for eight preloaded inbounds", queries)
	}
	for _, ib := range inbounds {
		st := ib.ClientStats[0]
		if st.UUID != "uuid-"+st.Email || st.SubId != "sub-"+st.Email || st.Up != 123 || st.Down != 456 {
			t.Fatalf("enriched stats = %+v", st)
		}
	}
}

func TestBackfillClientStatsChunkBoundaryAndEmpty(t *testing.T) {
	setupBulkDB(t)
	var inbounds []*model.Inbound
	for i := range 401 {
		inbounds = append(inbounds, &model.Inbound{Id: i + 1})
	}
	seedStatsIdentity(t, []int{1, 401}, "boundary")
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "boundary", InboundId: 999, Up: 7}).Error; err != nil {
		t.Fatal(err)
	}
	inbounds = append(inbounds, &model.Inbound{Id: 1}, &model.Inbound{Id: 401})
	queries := countClientTableQueries(t, "stats_boundary", func() { (&InboundService{}).enrichClientStats(database.GetDB(), inbounds) })
	if queries != 2 {
		t.Fatalf("clients queries = %d, want 2 for 401 distinct IDs plus duplicates", queries)
	}
	for _, i := range []int{0, 400} {
		if len(inbounds[i].ClientStats) != 1 || inbounds[i].ClientStats[0].Up != 7 {
			t.Fatalf("inbound %d stats = %+v", i, inbounds[i].ClientStats)
		}
	}
	queries = countClientTableQueries(t, "stats_empty", func() { (&InboundService{}).backfillClientStats(database.GetDB(), nil) })
	if queries != 0 {
		t.Fatalf("empty request queries = %d", queries)
	}
}

func TestBackfillClientStatsProjectionFailurePreservesStats(t *testing.T) {
	setupBulkDB(t)
	ib := &model.Inbound{Id: 1, ClientStats: []xray.ClientTraffic{{Email: "preloaded", UUID: "old", SubId: "old-sub", Up: 5}}}
	seedStatsIdentity(t, []int{1}, "preloaded")
	seedStatsIdentity(t, []int{1}, "missing")
	db := database.GetDB()
	if err := db.Create(&xray.ClientTraffic{Email: "missing", InboundId: 2, Up: 9}).Error; err != nil {
		t.Fatal(err)
	}
	want := append([]xray.ClientTraffic(nil), ib.ClientStats...)
	injected := errors.New("stats projection unavailable")
	name := "test:stats_projection_failure"
	queries := 0
	if err := db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "clients" {
			queries++
			if queries == 2 {
				tx.AddError(injected)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
	inbounds := []*model.Inbound{ib}
	for id := 2; id <= 401; id++ {
		inbounds = append(inbounds, &model.Inbound{Id: id})
	}
	(&InboundService{}).enrichClientStats(db, inbounds)
	if !reflect.DeepEqual(ib.ClientStats, want) {
		t.Fatalf("projection failure mutated stats: got %+v, want %+v", ib.ClientStats, want)
	}
}

func TestBackfillClientStatsTrafficPageFailureKeepsFreshPreload(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	ib := &model.Inbound{Id: 1, ClientStats: []xray.ClientTraffic{{Email: "preloaded", UUID: "old", SubId: "old-sub", Up: 5}}}
	seedStatsIdentity(t, []int{1}, "preloaded")
	for i := range 401 {
		email := fmt.Sprintf("page-%03d", i)
		seedStatsIdentity(t, []int{1}, email)
		if err := db.Create(&xray.ClientTraffic{Email: email, InboundId: 2, Up: int64(i)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	pages := 0
	name := "test:stats_traffic_page_failure"
	if err := db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "client_traffics" {
			pages++
			if pages == 2 {
				tx.AddError(errors.New("second traffic page unavailable"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
	(&InboundService{}).enrichClientStats(db, []*model.Inbound{ib})
	want := []xray.ClientTraffic{{Email: "preloaded", UUID: "uuid-preloaded", SubId: "sub-preloaded", Up: 5}}
	if pages != 2 || !reflect.DeepEqual(ib.ClientStats, want) {
		t.Fatalf("pages=%d stats=%+v, want two pages and %+v without partial append", pages, ib.ClientStats, want)
	}
}

func TestEnrichClientStatsPreservesSiblingTrafficAndClientOrder(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	for _, email := range []string{"second", "First", "absent", "FIRST"} {
		ids := []int{12}
		if email == "second" {
			ids = append(ids, 13)
		}
		seedStatsIdentity(t, ids, email)
	}
	rows := []xray.ClientTraffic{
		{Email: "First", InboundId: 999, Enable: true, Up: 123, Down: 456, Total: 789, ExpiryTime: 12345, LastOnline: 2468, Reset: 3},
		{Email: "second", InboundId: 999, Enable: false, Up: 10, Down: 20},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	prefix := xray.ClientTraffic{Email: "unlinked-prefix", Up: 77}
	inbounds := []*model.Inbound{{Id: 12, ClientStats: []xray.ClientTraffic{prefix}}, {Id: 13}, {Id: 14}}
	for range 2 {
		(&InboundService{}).enrichClientStats(db, inbounds)
	}
	wantFirst := rows[0]
	wantFirst.UUID = "uuid-FIRST"
	wantFirst.SubId = "sub-FIRST"
	wantSecond := rows[1]
	wantSecond.UUID = "uuid-second"
	wantSecond.SubId = "sub-second"
	if want := []xray.ClientTraffic{prefix, wantSecond, wantFirst}; !reflect.DeepEqual(inbounds[0].ClientStats, want) {
		t.Fatalf("stats = %+v, want %+v", inbounds[0].ClientStats, want)
	}
	if !reflect.DeepEqual(inbounds[1].ClientStats, []xray.ClientTraffic{wantSecond}) || len(inbounds[2].ClientStats) != 0 {
		t.Fatalf("shared/empty stats = %+v / %+v", inbounds[1].ClientStats, inbounds[2].ClientStats)
	}
	if err := db.Model(&model.ClientRecord{}).Where("email = ?", "FIRST").Updates(map[string]any{"uuid": "", "sub_id": ""}).Error; err != nil {
		t.Fatal(err)
	}
	(&InboundService{}).enrichClientStats(db, inbounds)
	if st := inbounds[0].ClientStats[2]; st.UUID != "" || st.SubId != "" {
		t.Fatalf("empty identity failed to clear stale values: %+v", st)
	}
}

func TestClientStatsIdentitiesUsesSuppliedTransaction(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	rec := seedStatsIdentity(t, []int{1}, "transaction")
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Model(&rec).Updates(map[string]any{"uuid": "uncommitted", "sub_id": "uncommitted-sub"}).Error; err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{Id: 1, ClientStats: []xray.ClientTraffic{{Email: rec.Email}}}
	(&InboundService{}).enrichClientStats(tx, []*model.Inbound{ib})
	if st := ib.ClientStats[0]; st.UUID != "uncommitted" || st.SubId != "uncommitted-sub" {
		t.Fatalf("transaction enrichment = %+v, want uncommitted identity", st)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	identities, err := (&ClientService{}).listStatsIdentitiesForInbounds(nil, []int{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	want := []clientStatsIdentity{{UUID: "uuid-transaction", Email: "transaction", SubID: "sub-transaction"}}
	if !reflect.DeepEqual(identities[1], want) {
		t.Fatalf("fallback after rollback = %+v, want %+v", identities[1], want)
	}
}
