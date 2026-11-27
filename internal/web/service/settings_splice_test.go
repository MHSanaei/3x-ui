package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gv, wv any
	if err := json.Unmarshal([]byte(got), &gv); err != nil {
		t.Fatalf("unmarshal got: %v\nraw: %s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wv); err != nil {
		t.Fatalf("unmarshal want: %v\nraw: %s", err, want)
	}
	if !reflect.DeepEqual(gv, wv) {
		t.Fatalf("JSON mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestFastSettingsIndexSpliceRoundTrip(t *testing.T) {
	seed := `{
  "decryption": "none",
  "testseed": [1, 2, 3],
  "clients": [
    {"email": "alpha@example.com", "id": "u-1", "enable": true, "flow": "xtls-rprx-vision", "comment": "quote\"and\\slash"},
    {"email": "beta@example.com", "id": "u-2", "enable": false, "flow": ""}
  ]
}`
	idx, ok := parseFastSettingsIndex(seed)
	if !ok || !idx.canSplice() {
		t.Fatalf("parseFastSettingsIndex failed: ok=%v", ok)
	}
	if !idx.hasTestseed || idx.visionCnt != 1 {
		t.Fatalf("metadata mismatch: hasTestseed=%v visionCnt=%d", idx.hasTestseed, idx.visionCnt)
	}
	if !idx.hasFoldEmail("ALPHA@example.com") {
		t.Fatal("expected case-insensitive match for alpha@example.com")
	}

	// Append 40 clients across the maxUnindexedTail=32 rebuild threshold.
	cur := idx
	for i := range 40 {
		em := fmt.Sprintf("extra-%02d@example.com", i)
		enc, sp, err := marshalClientWire(model.Client{
			Email:  em,
			ID:     fmt.Sprintf("u-extra-%02d", i),
			SubID:  fmt.Sprintf("sub-%02d", i),
			Enable: true,
		}, "", 1700000000000)
		if err != nil {
			t.Fatalf("marshalClientWire %d: %v", i, err)
		}
		_, cur = cur.spliceAppend([][]byte{enc}, []clientSpan{sp})
	}
	if len(cur.spans) != 42 {
		t.Fatalf("spans = %d, want 42", len(cur.spans))
	}
	if !cur.hasFoldEmail("EXTRA-39@example.com") {
		t.Fatal("missing appended client after tail index rebuild")
	}

	// Clearing the last vision flow when testseed is present must trigger fallback.
	posAlpha, found := cur.findFoldIndex("alpha@example.com")
	if !found {
		t.Fatal("alpha@example.com not found")
	}
	encNoFlow := []byte(`{"email":"alpha@example.com","id":"u-1","enable":true,"flow":""}`)
	if _, _, okSplice := cur.spliceReplaceOne(posAlpha, encNoFlow, clientSpan{email: "alpha@example.com", enable: true}, true); okSplice {
		t.Fatal("expected spliceReplaceOne to fall back when last vision client drops flow with testseed present")
	}

	// Keeping vision flow succeeds in-place.
	encKeepFlow := []byte(`{"email":"alpha@example.com","id":"u-1","enable":true,"flow":"xtls-rprx-vision","comment":"updated"}`)
	replacedRaw, cur, okSplice := cur.spliceReplaceOne(posAlpha, encKeepFlow, clientSpan{email: "alpha@example.com", enable: true, flow: "xtls-rprx-vision"}, true)
	if !okSplice {
		t.Fatal("spliceReplaceOne failed unexpectedly")
	}
	if !strings.Contains(replacedRaw, `"comment":"updated"`) {
		t.Fatalf("updated comment missing in spliced JSON: %s", replacedRaw)
	}

	// spliceReplaceMany updates multiple spans in one pass.
	posBeta, _ := cur.findFoldIndex("beta@example.com")
	posExtra0, _ := cur.findFoldIndex("extra-00@example.com")
	manyRaw, cur, okMany := cur.spliceReplaceMany([]clientReplacement{
		{pos: posExtra0, encoded: []byte(`{"email":"extra-00@example.com","id":"u-extra-00","enable":false}`), span: clientSpan{email: "extra-00@example.com", enable: false}},
		{pos: posBeta, encoded: []byte(`{"email":"beta@example.com","id":"u-2","enable":true}`), span: clientSpan{email: "beta@example.com", enable: true}},
	}, false)
	if !okMany {
		t.Fatal("spliceReplaceMany failed")
	}
	var parsed struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(manyRaw), &parsed); err != nil {
		t.Fatalf("invalid JSON after spliceReplaceMany: %v", err)
	}
	if len(parsed.Clients) != 42 || !parsed.Clients[1].Enable || parsed.Clients[2].Enable {
		t.Fatalf("unexpected client states after spliceReplaceMany: beta=%v extra0=%v", parsed.Clients[1].Enable, parsed.Clients[2].Enable)
	}

	// spliceRemove drops requested positions and leaves valid JSON.
	rmRaw, cur := cur.spliceRemove(map[int]struct{}{posBeta: {}, posExtra0: {}})
	if len(cur.spans) != 40 || cur.hasFoldEmail("beta@example.com") || cur.hasFoldEmail("extra-00@example.com") {
		t.Fatalf("spliceRemove left stale entries: len=%d", len(cur.spans))
	}
	if !json.Valid([]byte(rmRaw)) {
		t.Fatalf("invalid JSON after spliceRemove: %s", rmRaw)
	}

	// Legacy shapes: clients:null and missing clients must refuse direct splicing.
	for _, legacy := range []string{`{"decryption":"none","clients":null}`, `{"decryption":"none"}`} {
		li, ok := parseFastSettingsIndex(legacy)
		if !ok {
			t.Fatalf("parseFastSettingsIndex failed on valid legacy JSON %s", legacy)
		}
		if li.canSplice() {
			t.Fatalf("legacy JSON %s must not be marked spliceable", legacy)
		}
	}

	// scanTopLevelStringFields extracts top-level keys without parsing clients.
	enc, dec, okScan := scanTopLevelStringFields(`{"clients":[{"email":"a"}],"encryption":"mlkem.abc","decryption":"none"}`, "encryption", "decryption")
	if !okScan || enc != "mlkem.abc" || dec != "none" {
		t.Fatalf("scanTopLevelStringFields = (%q, %q, %v), want (mlkem.abc, none, true)", enc, dec, okScan)
	}
	assertJSONEqual(t, `{"a":1,"b":2}`, `{"b":2,"a":1}`)
}

func TestInboundSettingsIdxCacheRollbackAndInvalidation(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	InvalidateAllInboundSettingsCache()

	ib := mkInbound(t, 33101, model.VLESS, `{"decryption":"none","clients":[{"email":"seed@example.com","id":"11111111-1111-4111-8111-111111111111","enable":true}]}`)
	db := database.GetDB()

	// Warm the cache inside a committed serialized tx.
	if err := runSerializedTx(func(tx *gorm.DB) error {
		_, idxMap, err := loadInboundsForClientWrite(tx, []int{ib.Id})
		if err != nil {
			return err
		}
		if idxMap[ib.Id] == nil || !idxMap[ib.Id].hasFoldEmail("seed@example.com") {
			t.Fatalf("expected warm fastSettingsIndex for inbound %d", ib.Id)
		}
		return nil
	}); err != nil {
		t.Fatalf("warm tx: %v", err)
	}
	if inboundSettingsIdxCache.getOne(db, ib.Id) == nil {
		t.Fatal("expected inbound index cached after commit")
	}

	// A rolled-back serialized tx must NOT pollute the cache with uncommitted splices.
	wantRollbackErr := fmt.Errorf("force rollback")
	err := runSerializedTx(func(tx *gorm.DB) error {
		_, idxMap, lErr := loadInboundsForClientWrite(tx, []int{ib.Id})
		if lErr != nil {
			return lErr
		}
		enc, sp, _ := marshalClientWire(model.Client{Email: "rolledback@example.com", ID: "22222222-2222-4222-8222-222222222222", Enable: true}, "", 1700000000000)
		newRaw, newIdx := idxMap[ib.Id].spliceAppend([][]byte{enc}, []clientSpan{sp})
		if cErr := commitInboundSettingsDirectWithIdx(tx, ib.Id, newRaw, newIdx); cErr != nil {
			return cErr
		}
		return wantRollbackErr
	})
	if !errors.Is(err, wantRollbackErr) {
		t.Fatalf("err = %v, want %v", err, wantRollbackErr)
	}
	cached := inboundSettingsIdxCache.getOne(db, ib.Id)
	if cached == nil || cached.hasFoldEmail("rolledback@example.com") {
		t.Fatal("rolled-back splice leaked into inboundSettingsIdxCache")
	}

	// Non-settings traffic column updates must NOT evict the cache.
	if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Updates(map[string]any{"up": 123, "down": 456}).Error; err != nil {
		t.Fatalf("traffic update: %v", err)
	}
	if inboundSettingsIdxCache.getOne(db, ib.Id) == nil {
		t.Fatal("non-settings update evicted inboundSettingsIdxCache")
	}

	// Direct settings column writes MUST evict the cache immediately.
	if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", `{"decryption":"none","clients":[]}`).Error; err != nil {
		t.Fatalf("direct settings update: %v", err)
	}
	if inboundSettingsIdxCache.getOne(db, ib.Id) != nil {
		t.Fatal("direct settings write failed to evict inboundSettingsIdxCache")
	}
}

func TestCommitInboundsSettingsBatchChunked(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	// Exceed batchChunk=50 to exercise multi-chunk CASE UPDATE.
	const total = 55
	updates := make([]inboundSettingsUpdate, 0, total)
	ids := make([]int, 0, total)
	for i := range total {
		ib := mkInbound(t, 33200+i, model.VLESS, `{"decryption":"none","clients":[]}`)
		ids = append(ids, ib.Id)
		newRaw := fmt.Sprintf(`{"decryption":"none","clients":[{"email":"u%02d@example.com","enable":true}]}`, i)
		idx, _ := parseFastSettingsIndex(newRaw)
		updates = append(updates, inboundSettingsUpdate{inboundId: ib.Id, newSettings: newRaw, newIdx: idx})
	}

	if err := runSerializedTx(func(tx *gorm.DB) error {
		return commitInboundsSettingsBatch(tx, updates)
	}); err != nil {
		t.Fatalf("commitInboundsSettingsBatch: %v", err)
	}

	inboundSvc := &InboundService{}
	for i, id := range ids {
		ib, err := inboundSvc.GetInbound(id)
		if err != nil {
			t.Fatalf("GetInbound(%d): %v", id, err)
		}
		want := fmt.Sprintf("u%02d@example.com", i)
		if !strings.Contains(ib.Settings, want) {
			t.Fatalf("inbound %d settings missing %s: %s", id, want, ib.Settings)
		}
	}
}

func TestAddAndDeleteSkipOrphanScan(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const bystanderCount = 450
	seedClients := make([]model.Client, bystanderCount)
	for i := range bystanderCount {
		seedClients[i] = model.Client{
			Email:  fmt.Sprintf("bystander-%03d@example.com", i),
			ID:     fmt.Sprintf("30000000-0000-4000-8000-%012d", i+1),
			SubID:  fmt.Sprintf("bsub-%03d", i),
			Enable: true,
		}
	}
	ib := mkInbound(t, 33401, model.VLESS, clientsSettings(t, seedClients))
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	if err := svc.SyncInbound(nil, ib.Id, seedClients); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}

	db := database.GetDB()
	var largeClientScan atomic.Int32
	cbName := "test:detect_orphan_scan"
	if err := db.Callback().Query().After("gorm:query").Register(cbName, func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		sqlStr := strings.ToLower(tx.Statement.SQL.String())
		// compactOrphans issued chunked IN queries of up to 400 emails against clients.
		if strings.Contains(sqlStr, "from `clients`") || strings.Contains(sqlStr, `from "clients"`) || strings.Contains(sqlStr, "from clients") {
			if len(tx.Statement.Vars) >= 50 {
				largeClientScan.Add(1)
			}
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(cbName) })

	addPayload := &model.Inbound{
		Id: ib.Id,
		Settings: clientsSettings(t, []model.Client{{
			Email:  "new-single@example.com",
			ID:     "40000000-0000-4000-8000-000000000001",
			SubID:  "sub-new-single",
			Enable: true,
		}}),
	}
	if _, err := svc.AddInboundClient(inboundSvc, addPayload); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}
	if _, err := svc.DelInboundClientByEmail(inboundSvc, ib.Id, "new-single@example.com", false, true); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}
	if got := largeClientScan.Load(); got != 0 {
		t.Fatalf("detected %d O(N) bystander scans on clients table during single add/delete (issue #7217)", got)
	}
}
