package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func countTableQueries(t *testing.T, table string, fn func()) int {
	t.Helper()
	db := database.GetDB()
	var n int64
	cb := "test:count_q_" + table + "_" + strings.ReplaceAll(t.Name(), "/", "_")
	hook := func(tx *gorm.DB) {
		if tx.Statement == nil {
			return
		}
		if tx.Statement.Table == table || strings.Contains(strings.ToLower(tx.Statement.SQL.String()), table) {
			atomic.AddInt64(&n, 1)
		}
	}
	if err := db.Callback().Query().After("gorm:query").Register(cb, hook); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register(cb, hook); err != nil {
		t.Fatalf("register row callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Remove(cb)
		_ = db.Callback().Row().Remove(cb)
	}()
	fn()
	return int(atomic.LoadInt64(&n))
}

func TestSettingCacheTTLInvalidationHooksAndDBIsolation(t *testing.T) {
	setupBulkDB(t)
	ss := &SettingService{}
	InvalidateSettingsCache()

	if err := ss.SetPort(20531); err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	p1, err := ss.GetPort()
	if err != nil || p1 != 20531 {
		t.Fatalf("GetPort = (%d, %v), want 20531", p1, err)
	}

	hits := countTableQueries(t, "settings", func() {
		for range 5 {
			p, err := ss.GetPort()
			if err != nil || p != 20531 {
				t.Fatalf("cached GetPort = (%d, %v), want 20531", p, err)
			}
		}
	})
	if hits != 0 {
		t.Fatalf("cached GetPort issued %d settings queries, want 0", hits)
	}

	// Direct GORM write must invalidate via the registered write hook.
	if err := database.GetDB().Model(&model.Setting{}).Where("key = ?", "webPort").Update("value", "20533").Error; err != nil {
		t.Fatalf("direct setting update: %v", err)
	}
	if p2, err := ss.GetPort(); err != nil || p2 != 20533 {
		t.Fatalf("GetPort after direct GORM write = (%d, %v), want 20533", p2, err)
	}

	// Switching DB pointer must not return cached values from the previous DB.
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "isolated.db"))
	pOther, err := ss.GetPort()
	if err != nil {
		t.Fatalf("GetPort on new DB: %v", err)
	}
	if pOther == 20533 {
		t.Fatalf("new DB read stale cached port 20533 from previous DB pointer")
	}
}

func TestInboundMetaCacheInvalidationAndSettingsOnlySkip(t *testing.T) {
	setupBulkDB(t)
	InvalidateAllInboundMeta()

	clients := []model.Client{{Email: "meta@x", ID: "aaaaaaaa-0000-0000-0000-000000000001", Enable: true}}
	ib := mkInbound(t, 44101, model.VLESS, clientsSettings(t, clients))

	inboundSvc := &InboundService{}
	metas, errs := inboundSvc.GetInboundMetaBatch([]int{ib.Id})
	if len(errs) != 0 || metas[ib.Id] == nil {
		t.Fatalf("GetInboundMetaBatch initial load failed: errs=%v", errs)
	}
	if metas[ib.Id].Port != 44101 {
		t.Fatalf("meta port = %d, want 44101", metas[ib.Id].Port)
	}
	if strings.Contains(metas[ib.Id].SettingsNoClients, "meta@x") {
		t.Fatalf("SettingsNoClients still contains clients array: %s", metas[ib.Id].SettingsNoClients)
	}

	// Settings-only updates must not evict structural metadata.
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", ib.Settings).Error; err != nil {
		t.Fatalf("settings-only update: %v", err)
	}
	metaQueries := countTableQueries(t, "inbounds", func() {
		m2, err2 := inboundSvc.GetInboundMetaBatch([]int{ib.Id})
		if len(err2) != 0 || m2[ib.Id] == nil {
			t.Fatalf("cached GetInboundMetaBatch failed: %v", err2)
		}
	})
	if metaQueries != 0 {
		t.Fatalf("cached GetInboundMetaBatch issued %d DB queries, want 0", metaQueries)
	}

	// Structural column updates must evict the cached entry immediately.
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("remark", "renamed-ib").Error; err != nil {
		t.Fatalf("remark update: %v", err)
	}
	m3, err3 := inboundSvc.GetInboundMetaBatch([]int{ib.Id})
	if len(err3) != 0 || m3[ib.Id] == nil || m3[ib.Id].Remark != "renamed-ib" {
		t.Fatalf("meta after structural update = (%+v, %v), want remark renamed-ib", m3[ib.Id], err3)
	}
}

func TestClientListAggregatesInOneQueryAndPreservesOrder(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()

	const totalClients = 2*sqlInChunk + 5
	clients := make([]model.ClientRecord, totalClients)
	for i := range clients {
		clients[i] = model.ClientRecord{
			Email:   fmt.Sprintf("agg-%04d@x", i),
			SubID:   fmt.Sprintf("sub-%04d", i),
			Enable:  true,
			Reverse: `{"tag":"rev-1"}`,
		}
	}
	if err := db.CreateInBatches(clients, 200).Error; err != nil {
		t.Fatalf("seed clients: %v", err)
	}
	links := make([]model.ClientInbound, 0, totalClients+3)
	for _, ib := range []int{9, 2, 7, 1} {
		links = append(links, model.ClientInbound{ClientId: clients[0].Id, InboundId: ib})
	}
	for i := 1; i < totalClients; i++ {
		links = append(links, model.ClientInbound{ClientId: clients[i].Id, InboundId: 1 + i%3})
	}
	if err := db.CreateInBatches(links, 250).Error; err != nil {
		t.Fatalf("seed links: %v", err)
	}

	var rows []ClientWithAttachments
	ciQueries := countTableQueries(t, "client_inbounds", func() {
		var err error
		rows, err = (&ClientService{}).List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
	})
	if ciQueries != 1 {
		t.Fatalf("List issued %d client_inbounds queries for %d clients, want 1", ciQueries, totalClients)
	}
	if len(rows) != totalClients {
		t.Fatalf("List returned %d rows, want %d", len(rows), totalClients)
	}
	if !slices.Equal(rows[0].InboundIds, []int{9, 2, 7, 1}) {
		t.Fatalf("rows[0].InboundIds = %v, want [9 2 7 1]", rows[0].InboundIds)
	}

	// Verify single-pass MarshalJSON emits nested reverse object, inboundIds, and traffic.
	rows[0].Traffic = &xray.ClientTraffic{Email: rows[0].Email, Up: 10, Down: 20, Enable: true}
	encoded, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	rev, ok := decoded["reverse"].(map[string]any)
	if !ok || rev["tag"] != "rev-1" {
		t.Fatalf("reverse = %#v, want {tag: rev-1}", decoded["reverse"])
	}
	if _, ok := decoded["inboundIds"].([]any); !ok {
		t.Fatalf("inboundIds missing from marshaled ClientWithAttachments: %s", encoded)
	}
	if _, ok := decoded["traffic"].(map[string]any); !ok {
		t.Fatalf("traffic missing from marshaled ClientWithAttachments: %s", encoded)
	}

	// Verify PostgreSQL dialect SQL uses string_agg ordered by ctid.
	pgSQL := clientInboundAggregationSQL(database.DialectPostgres)
	if !strings.Contains(pgSQL, "string_agg(ci.inbound_id::text, ',' ORDER BY ci.ctid)") {
		t.Fatalf("unexpected postgres aggregation SQL: %s", pgSQL)
	}
}

func TestTunnelAllowedIPsSkipsNonTunnelInboundsAfterMetaWarm(t *testing.T) {
	setupBulkDB(t)
	InvalidateAllInboundMeta()
	clientSvc := &ClientService{}
	inboundSvc := &InboundService{}

	ids := make([]int, 0, 5)
	for i := range 5 {
		ib := mkInbound(t, 44200+i, model.VLESS, `{"clients":[]}`)
		ids = append(ids, ib.Id)
	}
	if _, err := clientSvc.TunnelAllowedIPsByInbound(inboundSvc, "u@x", ids); err != nil {
		t.Fatalf("warm TunnelAllowedIPsByInbound: %v", err)
	}
	q := countTableQueries(t, "inbounds", func() {
		res, err := clientSvc.TunnelAllowedIPsByInbound(inboundSvc, "u@x", ids)
		if err != nil || len(res) != 0 {
			t.Fatalf("TunnelAllowedIPsByInbound = (%v, %v)", res, err)
		}
	})
	if q != 0 {
		t.Fatalf("TunnelAllowedIPsByInbound for non-tunnel inbounds issued %d queries, want 0", q)
	}
}

func TestBuildClientPayloadsBatchesQueriesAcrossRecords(t *testing.T) {
	setupBulkDB(t)
	InvalidateAllInboundMeta()
	db := database.GetDB()
	clientSvc := &ClientService{}
	inboundSvc := &InboundService{}

	ib1 := mkInbound(t, 44301, model.VLESS, `{"clients":[]}`)
	ib2 := mkInbound(t, 44302, model.Shadowsocks, `{"method":"chacha20-ietf-poly1305","clients":[]}`)

	recs := make([]*model.ClientRecord, 0, 10)
	for i := range 10 {
		rec := &model.ClientRecord{
			Email:  fmt.Sprintf("batch-%02d@x", i),
			SubID:  "shared-sub",
			TgID:   777001,
			Enable: true,
		}
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib1.Id, FlowOverride: "xtls-rprx-vision"}).Error; err != nil {
			t.Fatalf("create link1: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib2.Id}).Error; err != nil {
			t.Fatalf("create link2: %v", err)
		}
		mkTraffic(t, ib1.Id, rec.Email, int64(i+1)*100, int64(i+1)*200, 0, 0, true)
		recs = append(recs, rec)
	}

	var payloads []ClientDetailPayload
	ciQueries := countTableQueries(t, "client_inbounds", func() {
		var err error
		payloads, err = clientSvc.BuildClientPayloads(inboundSvc, recs)
		if err != nil {
			t.Fatalf("BuildClientPayloads: %v", err)
		}
	})
	if ciQueries != 1 {
		t.Fatalf("BuildClientPayloads issued %d client_inbounds queries for 10 records, want 1", ciQueries)
	}
	if len(payloads) != 10 {
		t.Fatalf("len(payloads) = %d, want 10", len(payloads))
	}
	for i, p := range payloads {
		if p.Client.Flow != "xtls-rprx-vision" {
			t.Fatalf("payload[%d].Client.Flow = %q, want xtls-rprx-vision", i, p.Client.Flow)
		}
		wantUsed := int64(i+1) * 300
		if p.UsedTraffic != wantUsed {
			t.Fatalf("payload[%d].UsedTraffic = %d, want %d", i, p.UsedTraffic, wantUsed)
		}
		if !slices.Equal(p.InboundIds, []int{ib1.Id, ib2.Id}) {
			t.Fatalf("payload[%d].InboundIds = %v, want [%d %d]", i, p.InboundIds, ib1.Id, ib2.Id)
		}
	}
}
