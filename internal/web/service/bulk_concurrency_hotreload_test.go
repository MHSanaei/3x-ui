package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type recordingBatchRuntime struct {
	mu              sync.Mutex
	addUserCalls    int
	removeUserCalls int
	batchAddCalls   int
	batchRmCalls    int
	addedEmails     []string
	removedEmails   []string
	addReplaceFlags []bool
}

func (r *recordingBatchRuntime) Name() string { return "rec-local" }
func (r *recordingBatchRuntime) AddInbound(context.Context, *model.Inbound) error {
	return nil
}

func (r *recordingBatchRuntime) DelInbound(context.Context, *model.Inbound) error {
	return nil
}

func (r *recordingBatchRuntime) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	return nil
}

func (r *recordingBatchRuntime) UpdateUser(context.Context, *model.Inbound, string, model.Client) error {
	return nil
}

func (r *recordingBatchRuntime) DeleteUser(context.Context, *model.Inbound, string) error {
	return nil
}

func (r *recordingBatchRuntime) AddClient(context.Context, *model.Inbound, model.Client) error {
	return nil
}
func (r *recordingBatchRuntime) DeleteClient(context.Context, string) error { return nil }
func (r *recordingBatchRuntime) RestartXray(context.Context) error          { return nil }
func (r *recordingBatchRuntime) ResetClientTraffic(context.Context, *model.Inbound, string) error {
	return nil
}

func (r *recordingBatchRuntime) ResetInboundTraffic(context.Context, *model.Inbound) error {
	return nil
}
func (r *recordingBatchRuntime) ResetAllTraffics(context.Context) error { return nil }

func (r *recordingBatchRuntime) AddUser(_ context.Context, _ *model.Inbound, user map[string]any) error {
	r.mu.Lock()
	r.addUserCalls++
	if em, _ := user["email"].(string); em != "" {
		r.addedEmails = append(r.addedEmails, em)
	}
	r.mu.Unlock()
	return nil
}

func (r *recordingBatchRuntime) RemoveUser(_ context.Context, _ *model.Inbound, email string) error {
	r.mu.Lock()
	r.removeUserCalls++
	r.removedEmails = append(r.removedEmails, email)
	r.mu.Unlock()
	return nil
}

func (r *recordingBatchRuntime) AddUsers(_ context.Context, _ *model.Inbound, ops []runtime.UserAddOp) []error {
	r.mu.Lock()
	r.batchAddCalls++
	for _, op := range ops {
		r.addReplaceFlags = append(r.addReplaceFlags, op.Replace)
		if em, _ := op.User["email"].(string); em != "" {
			r.addedEmails = append(r.addedEmails, em)
		}
	}
	r.mu.Unlock()
	return make([]error, len(ops))
}

func (r *recordingBatchRuntime) RemoveUsers(_ context.Context, _ *model.Inbound, emails []string) []error {
	r.mu.Lock()
	r.batchRmCalls++
	r.removedEmails = append(r.removedEmails, emails...)
	r.mu.Unlock()
	return make([]error, len(emails))
}

func (r *recordingBatchRuntime) resetCounts() {
	r.mu.Lock()
	r.addUserCalls = 0
	r.removeUserCalls = 0
	r.batchAddCalls = 0
	r.batchRmCalls = 0
	r.addedEmails = nil
	r.removedEmails = nil
	r.addReplaceFlags = nil
	r.mu.Unlock()
}

func TestBulkCreateConcurrentEmailAndSubIDRace(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	ib1 := mkInbound(t, 34101, model.VLESS, `{"decryption":"none","clients":[]}`)
	ib2 := mkInbound(t, 34102, model.VLESS, `{"decryption":"none","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	// Case 1: Concurrent case-insensitive email race must produce exactly one winner.
	var winners atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	variants := []string{"RaceUser@example.com", "raceuser@example.com", "RACEUSER@EXAMPLE.COM"}
	for i, em := range variants {
		wg.Add(1)
		go func(idx int, email string) {
			defer wg.Done()
			<-start
			res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{
				Client:     model.Client{Email: email, SubID: fmt.Sprintf("sub-race-%d", idx), Enable: true},
				InboundIds: []int{ib1.Id},
			}})
			if err == nil {
				winners.Add(int32(res.Created))
			}
		}(i, em)
	}
	close(start)
	wg.Wait()
	if got := winners.Load(); got != 1 {
		t.Fatalf("case-insensitive race winners = %d, want 1", got)
	}

	// Case 2: Concurrent subId collision across disjoint inbounds must produce exactly one winner.
	var subWinners atomic.Int32
	start2 := make(chan struct{})
	for i, ibID := range []int{ib1.Id, ib2.Id} {
		wg.Add(1)
		go func(idx, targetIB int) {
			defer wg.Done()
			<-start2
			res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{
				Client:     model.Client{Email: fmt.Sprintf("sub-racer-%d@example.com", idx), SubID: "shared-raced-subid", Enable: true},
				InboundIds: []int{targetIB},
			}})
			if err == nil {
				subWinners.Add(int32(res.Created))
			}
		}(i, ibID)
	}
	close(start2)
	wg.Wait()
	if got := subWinners.Load(); got != 1 {
		t.Fatalf("disjoint-inbound subId race winners = %d, want 1", got)
	}

	// Case 3: Hijack attempt (existing email, different subId) must fail only itself, preserving innocent sibling.
	res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{
		{
			Client:     model.Client{Email: "raceuser@example.com", SubID: "hijack-subid", Enable: true},
			InboundIds: []int{ib2.Id},
		},
		{
			Client:     model.Client{Email: "innocent@example.com", SubID: "innocent-subid", Enable: true},
			InboundIds: []int{ib2.Id},
		},
	})
	if err != nil {
		t.Fatalf("BulkCreate mixed batch: %v", err)
	}
	if res.Created != 1 || len(res.Skipped) != 1 || res.Skipped[0].Email != "raceuser@example.com" {
		t.Fatalf("unexpected mixed batch result: %+v", res)
	}
	if rec := lookupClientRecord(t, "innocent@example.com"); rec.Id == 0 {
		t.Fatal("innocent sibling client was not persisted")
	}
}

func TestBulkCreateErrorIsolationAndNegativeTotalGB(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	ib := mkInbound(t, 34201, model.VLESS, `{"decryption":"none","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{
		{
			Client:     model.Client{Email: "neg@example.com", SubID: "sub-neg", Enable: true, TotalGB: -1024},
			InboundIds: []int{ib.Id},
		},
		{
			Client:     model.Client{Email: "missing-ib@example.com", SubID: "sub-miss", Enable: true},
			InboundIds: []int{ib.Id, 999999},
		},
		{
			Client:     model.Client{Email: "healthy@example.com", SubID: "sub-ok", Enable: true},
			InboundIds: []int{ib.Id},
		},
	})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if res.Created != 1 || len(res.Skipped) != 2 {
		t.Fatalf("result = %+v, want Created=1 Skipped=2", res)
	}
	if _, ok := settingsClient(t, ib.Id, "missing-ib@example.com"); ok {
		t.Fatal("failed client partially landed in healthy inbound")
	}
	if _, ok := settingsClient(t, ib.Id, "healthy@example.com"); !ok {
		t.Fatal("healthy sibling missing from inbound")
	}
}

func TestAttachAndBulkAttachPropagateMintedCredentials(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	vlessIb := mkInbound(t, 34301, model.VLESS, `{"decryption":"none","clients":[]}`)
	trojan1 := mkInbound(t, 34302, model.Trojan, `{"clients":[]}`)
	trojan2 := mkInbound(t, 34303, model.Trojan, `{"clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	// Seed two VLESS-only clients with empty passwords.
	for _, em := range []string{"att-one@example.com", "att-bulk@example.com"} {
		if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
			Client:     model.Client{Email: em, Enable: true},
			InboundIds: []int{vlessIb.Id},
		}); err != nil {
			t.Fatalf("seed %s: %v", em, err)
		}
	}

	rec1 := lookupClientRecord(t, "att-one@example.com")
	if _, err := svc.Attach(inboundSvc, rec1.Id, []int{trojan1.Id, trojan2.Id}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	c1a, ok1a := settingsClient(t, trojan1.Id, "att-one@example.com")
	c1b, ok1b := settingsClient(t, trojan2.Id, "att-one@example.com")
	if !ok1a || !ok1b || c1a.Password == "" || c1a.Password != c1b.Password {
		t.Fatalf("Attach minted mismatched passwords across trojan inbounds: %q vs %q", c1a.Password, c1b.Password)
	}

	res, _, err := svc.BulkAttach(inboundSvc, []string{"att-bulk@example.com"}, []int{trojan1.Id, trojan2.Id})
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("BulkAttach: res=%+v err=%v", res, err)
	}
	c2a, ok2a := settingsClient(t, trojan1.Id, "att-bulk@example.com")
	c2b, ok2b := settingsClient(t, trojan2.Id, "att-bulk@example.com")
	if !ok2a || !ok2b || c2a.Password == "" || c2a.Password != c2b.Password {
		t.Fatalf("BulkAttach minted mismatched passwords across trojan inbounds: %q vs %q", c2a.Password, c2b.Password)
	}
}

func TestAttachAndBulkAttachSkipXrayAddForDepletedClient(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	recRt := &recordingBatchRuntime{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(recRt)

	ib1 := mkInbound(t, 34401, model.VLESS, `{"decryption":"none","clients":[]}`)
	ib2 := mkInbound(t, 34402, model.VLESS, `{"decryption":"none","clients":[]}`)
	ib3 := mkInbound(t, 34403, model.VLESS, `{"decryption":"none","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "depleted@example.com", Enable: true, TotalGB: 1000},
		InboundIds: []int{ib1.Id},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Simulate quota depletion on client_traffics while settings still hold enable=true.
	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "depleted@example.com").
		Updates(map[string]any{"enable": false, "up": 600, "down": 500}).Error; err != nil {
		t.Fatalf("mark depleted: %v", err)
	}
	recRt.resetCounts()

	rec := lookupClientRecord(t, "depleted@example.com")
	if _, err := svc.Attach(inboundSvc, rec.Id, []int{ib2.Id}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if _, _, err := svc.BulkAttach(inboundSvc, []string{"depleted@example.com"}, []int{ib3.Id}); err != nil {
		t.Fatalf("BulkAttach: %v", err)
	}
	if len(recRt.addedEmails) != 0 {
		t.Fatalf("depleted client was added to live Xray during Attach/BulkAttach: %v", recRt.addedEmails)
	}
	var tr xray.ClientTraffic
	if err := db.Where("email = ?", "depleted@example.com").First(&tr).Error; err != nil {
		t.Fatalf("load traffic: %v", err)
	}
	if tr.Enable {
		t.Fatal("Attach/BulkAttach re-enabled depleted client_traffics row")
	}
}

func TestUpdateMetadataOnlySkipsXrayReload(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	recRt := &recordingBatchRuntime{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(recRt)

	ib1 := mkInbound(t, 34501, model.VLESS, `{"decryption":"none","clients":[]}`)
	ib2 := mkInbound(t, 34502, model.VLESS, `{"decryption":"none","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "meta@example.com", Enable: true, TotalGB: 10 << 30},
		InboundIds: []int{ib1.Id, ib2.Id},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	rec := lookupClientRecord(t, "meta@example.com")
	recRt.resetCounts()

	// Metadata-only edit (TotalGB + Comment) must skip RemoveUser/AddUser on active client.
	edited := *rec.ToClient()
	edited.TotalGB = 20 << 30
	edited.Comment = "renewed"
	if nr, err := svc.Update(inboundSvc, rec.Id, edited, rec.LimitHwid); err != nil || nr {
		t.Fatalf("metadata Update = (%v, %v), want (false, nil)", nr, err)
	}
	if recRt.addUserCalls != 0 || recRt.removeUserCalls != 0 || recRt.batchAddCalls != 0 {
		t.Fatalf("metadata-only edit triggered Xray RPCs: add=%d remove=%d batchAdd=%d",
			recRt.addUserCalls, recRt.removeUserCalls, recRt.batchAddCalls)
	}

	// Runtime-visible edit (UUID rotation) MUST trigger RemoveUser + AddUser on both inbounds.
	edited.ID = "99999999-1111-4111-8111-111111111111"
	if _, err := svc.Update(inboundSvc, rec.Id, edited, rec.LimitHwid); err != nil {
		t.Fatalf("credential Update: %v", err)
	}
	if recRt.removeUserCalls != 2 || recRt.addUserCalls != 2 {
		t.Fatalf("UUID edit RPCs: remove=%d add=%d, want 2 and 2", recRt.removeUserCalls, recRt.addUserCalls)
	}

	// Depleted client (client_traffics.enable=false, settings.enable=true) MUST re-add to Xray on quota edit.
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "meta@example.com").
		Update("enable", false).Error; err != nil {
		t.Fatalf("mark traffic disabled: %v", err)
	}
	recRt.resetCounts()
	edited.TotalGB = 30 << 30
	if _, err := svc.Update(inboundSvc, rec.Id, edited, rec.LimitHwid); err != nil {
		t.Fatalf("depleted quota Update: %v", err)
	}
	if recRt.addUserCalls != 2 {
		t.Fatalf("depleted client re-add calls = %d, want 2 across both inbounds", recRt.addUserCalls)
	}
}

func TestBulkResetTrafficWithRestartOnlyWhenNeeded(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	recRt := &recordingBatchRuntime{}
	mgr := useTestRuntimeManager(t)
	mgr.SetLocalRuntimeOverride(recRt)

	ib := mkInbound(t, 34601, model.VLESS, `{"decryption":"none","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     model.Client{Email: "active-reset@example.com", Enable: true, TotalGB: 1000},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "active-reset@example.com").
		Updates(map[string]any{"up": 200, "down": 300}).Error; err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	affected, needRestart, err := svc.BulkResetTrafficWithRestart(inboundSvc, []string{"active-reset@example.com"})
	if err != nil {
		t.Fatalf("BulkResetTrafficWithRestart: %v", err)
	}
	if affected != 1 || needRestart {
		t.Fatalf("active reset = (affected=%d, needRestart=%v), want (1, false)", affected, needRestart)
	}

	// Depleted client with unavailable runtime must report needRestart=true.
	if _, _, err := svc.BulkSetEnable(inboundSvc, []string{"active-reset@example.com"}, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "active-reset@example.com").
		Updates(map[string]any{"up": 600, "down": 500, "enable": false}).Error; err != nil {
		t.Fatalf("seed depleted: %v", err)
	}
	mgr.SetLocalRuntimeOverride(nil)
	affected, needRestart, err = svc.BulkResetTrafficWithRestart(inboundSvc, []string{"active-reset@example.com"})
	if err != nil {
		t.Fatalf("BulkResetTrafficWithRestart depleted: %v", err)
	}
	if affected != 1 || !needRestart {
		t.Fatalf("depleted reset without runtime = (affected=%d, needRestart=%v), want (1, true)", affected, needRestart)
	}
}

func TestBulkDeleteAndDetachHonorRestartOnDisable(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	recRt := &recordingBatchRuntime{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(recRt)
	if err := (&SettingService{}).SetRestartXrayOnClientDisable(true); err != nil {
		t.Fatalf("SetRestartXrayOnClientDisable: %v", err)
	}

	ib1 := mkInbound(t, 34701, model.VLESS, `{"decryption":"none","clients":[]}`)
	ib2 := mkInbound(t, 34702, model.VLESS, `{"decryption":"none","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	for _, em := range []string{"drop-det@example.com", "drop-del@example.com"} {
		if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
			Client:     model.Client{Email: em, Enable: true},
			InboundIds: []int{ib1.Id, ib2.Id},
		}); err != nil {
			t.Fatalf("Create %s: %v", em, err)
		}
	}

	_, nrDetach, err := svc.BulkDetach(inboundSvc, []string{"drop-det@example.com"}, []int{ib1.Id})
	if err != nil || !nrDetach {
		t.Fatalf("BulkDetach with restartXrayOnClientDisable = (nr=%v, err=%v), want (true, nil)", nrDetach, err)
	}

	_, nrDelete, err := svc.BulkDelete(inboundSvc, []string{"drop-del@example.com"}, false)
	if err != nil || !nrDelete {
		t.Fatalf("BulkDelete with restartXrayOnClientDisable = (nr=%v, err=%v), want (true, nil)", nrDelete, err)
	}
}

func TestBulkSetEnableUsesBatchRuntimeAndReplaceFlag(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	recRt := &recordingBatchRuntime{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(recRt)

	ib := mkInbound(t, 34801, model.Shadowsocks, `{"method":"chacha20-ietf-poly1305","clients":[]}`)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	emails := []string{"ben-1@example.com", "ben-2@example.com", "ben-3@example.com"}
	payloads := make([]ClientCreatePayload, len(emails))
	for i, em := range emails {
		payloads[i] = ClientCreatePayload{
			Client:     model.Client{Email: em, Enable: true},
			InboundIds: []int{ib.Id},
		}
	}
	if _, _, err := svc.BulkCreate(inboundSvc, payloads); err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	// BulkCreate must pass Replace=false for brand-new users.
	if recRt.batchAddCalls != 1 || len(recRt.addReplaceFlags) != 3 {
		t.Fatalf("BulkCreate batchAddCalls=%d flags=%v, want 1 and 3", recRt.batchAddCalls, recRt.addReplaceFlags)
	}
	for i, repl := range recRt.addReplaceFlags {
		if repl {
			t.Fatalf("BulkCreate op %d used Replace=true, want false", i)
		}
	}

	recRt.resetCounts()
	resDis, _, err := svc.BulkSetEnable(inboundSvc, emails, false)
	if err != nil || resDis.Changed != 3 {
		t.Fatalf("BulkSetEnable(false) = (%+v, %v)", resDis, err)
	}
	if recRt.batchRmCalls != 1 || len(recRt.removedEmails) != 3 {
		t.Fatalf("BulkSetEnable(false) batchRmCalls=%d removed=%v", recRt.batchRmCalls, recRt.removedEmails)
	}

	recRt.resetCounts()
	resEn, _, err := svc.BulkSetEnable(inboundSvc, emails, true)
	if err != nil || resEn.Changed != 3 {
		t.Fatalf("BulkSetEnable(true) = (%+v, %v)", resEn, err)
	}
	// Re-enabling must keep Replace=true so legacy Shadowsocks stays idempotent.
	if recRt.batchAddCalls != 1 || len(recRt.addReplaceFlags) != 3 {
		t.Fatalf("BulkSetEnable(true) batchAddCalls=%d flags=%v", recRt.batchAddCalls, recRt.addReplaceFlags)
	}
	for i, repl := range recRt.addReplaceFlags {
		if !repl {
			t.Fatalf("BulkSetEnable(true) op %d used Replace=false, want true", i)
		}
	}
	for _, em := range emails {
		if c, ok := settingsClient(t, ib.Id, em); !ok || !c.Enable {
			t.Fatalf("client %s not re-enabled in settings", em)
		}
		if !strings.EqualFold(lookupClientRecord(t, em).Email, em) {
			t.Fatalf("client record %s missing", em)
		}
	}
}
