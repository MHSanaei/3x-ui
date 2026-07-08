package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientDelete_CleansNodeBaselines(t *testing.T) {
	db := initTrafficTestDB(t)

	const email = "orphan@example.com"
	rec := &model.ClientRecord{Email: email, SubID: "sub-orphan", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: email, Up: 10, Down: 20}).Error; err != nil {
		t.Fatalf("seed node baseline 1: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 2, Email: email, Up: 30, Down: 40}).Error; err != nil {
		t.Fatalf("seed node baseline 2: %v", err)
	}

	svc := &ClientService{}
	if _, err := svc.Delete(&InboundService{}, rec.Id, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var cnt int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected node baselines cleaned on client delete, found %d", cnt)
	}
}

func TestClientDelete_KeepTrafficPreservesNodeBaselines(t *testing.T) {
	db := initTrafficTestDB(t)

	const email = "kept@example.com"
	rec := &model.ClientRecord{Email: email, SubID: "sub-kept", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 1, Email: email, Up: 10, Down: 20}).Error; err != nil {
		t.Fatalf("seed node baseline: %v", err)
	}

	svc := &ClientService{}
	if _, err := svc.Delete(&InboundService{}, rec.Id, true); err != nil {
		t.Fatalf("Delete(keepTraffic): %v", err)
	}

	var cnt int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if cnt != 1 {
		t.Errorf("keepTraffic delete must preserve node baselines, found %d", cnt)
	}
}

func TestDeleteByEmail_FallbackCleansNodeBaselines(t *testing.T) {
	db := initTrafficTestDB(t)

	const email = "recordless@example.com"
	ib := &model.Inbound{
		UserId:   1,
		Tag:      "local-fallback",
		Enable:   true,
		Port:     41001,
		Protocol: model.VLESS,
		Settings: `{"clients": [{"id": "5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a001", "email": "recordless@example.com", "enable": true}]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Create(&model.NodeClientTraffic{NodeId: 3, Email: email, Up: 5, Down: 6}).Error; err != nil {
		t.Fatalf("seed node baseline: %v", err)
	}

	svc := &ClientService{}
	if _, err := svc.DeleteByEmail(&InboundService{}, email, false); err != nil {
		t.Fatalf("DeleteByEmail: %v", err)
	}

	var cnt int64
	if err := db.Model(&model.NodeClientTraffic{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		t.Fatalf("count baselines: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected node baselines cleaned on record-less delete, found %d", cnt)
	}
}
