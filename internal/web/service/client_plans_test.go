package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func planTestDB(t *testing.T) []int {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "plans.db"))
	var ids []int
	for i, protocol := range []model.Protocol{model.VLESS, model.Trojan} {
		ib := &model.Inbound{UserId: 1, Tag: string(protocol), Port: 8400 + i, Protocol: protocol, Settings: `{"clients":[]}`, Enable: true}
		if err := database.GetDB().Create(ib).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, ib.Id)
	}
	return ids
}

func TestClientPlansPersistValidateAndUseOptimisticRevisions(t *testing.T) {
	ids := planTestDB(t)
	svc := &ClientPlanService{}
	plan, err := svc.Save(ClientPlan{Name: "A", QuotaBytes: 30 << 30, Days: 30, InboundIDs: ids, Enabled: true, RenewalMode: "replace"})
	if err != nil {
		t.Fatal(err)
	}
	other := &ClientPlanService{}
	stored, err := other.Resolve("a")
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != plan.ID || stored.Revision != 1 || stored.QuotaBytes != 30<<30 || len(stored.InboundIDs) != 2 {
		t.Fatalf("plan did not persist: %+v", stored)
	}
	stored.Days = 90
	updated, err := other.Save(*stored)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("revision = %d", updated.Revision)
	}
	if _, err := svc.Save(*plan); err == nil {
		t.Fatal("stale administrator overwrote edited plan")
	}
	if _, err := svc.Save(ClientPlan{Name: "A", QuotaBytes: 1, Days: 1, InboundIDs: ids, RenewalMode: "replace"}); err == nil {
		t.Fatal("duplicate name accepted")
	}
	var setting model.Setting
	if err := database.GetDB().Where("key = ?", clientPlansSettingKey).First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"uuid", "password", "subId", "tgId", "secret"} {
		if strings.Contains(setting.Value, `"`+secret+`"`) {
			t.Errorf("preset stores %s", secret)
		}
	}
	if err := svc.Delete(plan.ID, plan.Revision); err == nil {
		t.Fatal("stale deletion accepted")
	}
	if err := svc.Delete(updated.ID, updated.Revision); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("deleted plan remains: %+v", list)
	}
}

func TestClientPlansRejectInvalidAndDeletedInboundsWithoutWrites(t *testing.T) {
	ids := planTestDB(t)
	svc := &ClientPlanService{}
	valid := ClientPlan{Name: "A", QuotaBytes: 30 << 30, Days: 30, InboundIDs: ids, Enabled: true, RenewalMode: "replace"}
	for _, mutate := range []func(*ClientPlan){
		func(p *ClientPlan) { p.Name = "bad name" }, func(p *ClientPlan) { p.QuotaBytes = -1 },
		func(p *ClientPlan) { p.Days = -1 }, func(p *ClientPlan) { p.LimitIP = -1 },
		func(p *ClientPlan) { p.InboundIDs = []int{9999} }, func(p *ClientPlan) { p.InboundIDs = nil },
		func(p *ClientPlan) { p.RenewalMode = "unknown" },
	} {
		p := valid
		mutate(&p)
		if _, err := svc.Save(p); err == nil {
			t.Errorf("invalid plan accepted: %+v", p)
		}
	}
	stored, err := svc.Save(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Delete(&model.Inbound{}, ids[0]).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Validate(*stored, "ali"); err == nil {
		t.Fatal("deleted inbound accepted at execution")
	}
	var count int64
	if err := database.GetDB().Model(&model.Setting{}).Where("key = ?", clientPlansSettingKey).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("invalid writes: count=%d err=%v", count, err)
	}
}

func TestClientPlanConcurrentEditsHaveOneWinner(t *testing.T) {
	ids := planTestDB(t)
	svc := &ClientPlanService{}
	p, err := svc.Save(ClientPlan{Name: "A", Days: 30, InboundIDs: ids, RenewalMode: "extend"})
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			edit := *p
			edit.Days += i + 1
			if _, err := svc.Save(edit); err == nil {
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent edits applied %d times", successes.Load())
	}
	rows, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Revision != 2 {
		t.Fatalf("corrupted persisted plans: %s", data)
	}
}

func TestClientPlanRejectsUnsupportedAndIncompatibleProtocols(t *testing.T) {
	ids := planTestDB(t)
	svc := &ClientPlanService{}
	plan := ClientPlan{Name: "A", Days: 30, InboundIDs: ids, RenewalMode: "replace"}
	db := database.GetDB()
	if err := db.Model(&model.Inbound{}).Where("id = ?", ids[1]).Update("protocol", model.HTTP).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(plan); err == nil {
		t.Fatal("inbound-level authentication accepted as a multi-client plan")
	}
	for i, method := range []string{"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm"} {
		if err := db.Model(&model.Inbound{}).Where("id = ?", ids[i]).Updates(map[string]any{"protocol": model.Shadowsocks, "settings": `{"clients":[],"method":"` + method + `"}`}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Save(plan); err == nil {
		t.Fatal("incompatible Shadowsocks key sizes accepted")
	}
}

func TestClientPlanRevalidatesNodeProtocolVersion(t *testing.T) {
	ids := planTestDB(t)
	node := seedVersionedNode(t, "v3.9.0")
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ids[1]).Updates(map[string]any{"node_id": node.Id, "protocol": model.MASQUE}).Error; err != nil {
		t.Fatal(err)
	}
	plan := ClientPlan{Name: "A", Days: 30, InboundIDs: ids, RenewalMode: "replace"}
	if _, err := (&ClientPlanService{}).Save(plan); err == nil {
		t.Fatal("plan accepted MASQUE on a node predating protocol support")
	}
	if err := database.GetDB().Model(node).Update("panel_version", "v3.11.0").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&ClientPlanService{}).Save(plan); err != nil {
		t.Fatal(err)
	}
}
