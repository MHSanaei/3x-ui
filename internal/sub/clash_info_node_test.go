package sub

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSubClash_InfoNode_Active(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id:       1,
		UserId:   1,
		Remark:   "Germany-VLESS",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"c1-uuid","email":"user1@test.com","subId":"sub-clash","enable":true,"totalGB":10737418240}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:      1,
		Email:   "user1@test.com",
		SubID:   "sub-clash",
		UUID:    "c1-uuid",
		Enable:  true,
		TotalGB: 10737418240,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{InboundId: 1, ClientId: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: 1,
		Email:     "user1@test.com",
		Up:        1073741824,
		Down:      1073741824,
		Total:     10737418240,
		Enable:    true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	sub := NewSubService("{{EMAIL}}|📊{{TRAFFIC_LEFT}}")
	sub.subInfoNodeEnable = true
	clash := NewSubClashService(false, "", sub)

	out, _, err := clash.GetClash("sub-clash", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}

	if !strings.Contains(out, "type: socks5") || !strings.Contains(out, "server: 127.0.0.1") {
		t.Fatalf("expected socks5 dummy node in clash YAML, got:\n%s", out)
	}
	if !strings.Contains(out, "user1@test.com|📊8.00GB") {
		t.Fatalf("expected expanded remark on dummy proxy, got:\n%s", out)
	}
}

func TestSubClash_InfoNode_Expired(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	expiredTime := time.Now().Add(-24 * time.Hour).UnixMilli()
	ib := &model.Inbound{
		Id:       1,
		UserId:   1,
		Remark:   "Germany-VLESS",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: fmt.Sprintf(`{"clients":[{"id":"c1-uuid","email":"exp@test.com","subId":"sub-clash-exp","enable":true,"expiryTime":%d}]}`, expiredTime),
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:         1,
		Email:      "exp@test.com",
		SubID:      "sub-clash-exp",
		UUID:       "c1-uuid",
		Enable:     true,
		ExpiryTime: expiredTime,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{InboundId: 1, ClientId: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId:  1,
		Email:      "exp@test.com",
		ExpiryTime: expiredTime,
		Enable:     true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	sub := NewSubService("{{INBOUND}}")
	sub.subInfoNodeEnable = true
	sub.subExpiredTemplate = service.DefaultSubExpiredTemplate
	clash := NewSubClashService(false, "", sub)

	out, _, err := clash.GetClash("sub-clash-exp", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}

	if !strings.Contains(out, "type: socks5") || !strings.Contains(out, "server: 127.0.0.1") {
		t.Fatalf("expected socks5 dummy node in clash YAML, got:\n%s", out)
	}
	if !strings.Contains(out, "Expired") || !strings.Contains(out, "exp@test.com") {
		t.Fatalf("expected expired remark in clash YAML, got:\n%s", out)
	}
	if strings.Contains(out, "Germany-VLESS") {
		t.Fatalf("expired subscription must NOT contain working inbound, got:\n%s", out)
	}
}

func TestSubClash_InfoNode_Depleted(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id:       1,
		UserId:   1,
		Remark:   "Germany-VLESS",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"c1-uuid","email":"dep@test.com","subId":"sub-clash-dep","enable":true,"totalGB":5368709120}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:      1,
		Email:   "dep@test.com",
		SubID:   "sub-clash-dep",
		UUID:    "c1-uuid",
		Enable:  true,
		TotalGB: 5368709120,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{InboundId: 1, ClientId: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: 1,
		Email:     "dep@test.com",
		Up:        3221225472,
		Down:      2147483648,
		Total:     5368709120,
		Enable:    true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	sub := NewSubService("{{INBOUND}}")
	sub.subInfoNodeEnable = true
	sub.subTrafficDepletedTemplate = service.DefaultSubTrafficDepletedTemplate
	clash := NewSubClashService(false, "", sub)

	out, _, err := clash.GetClash("sub-clash-dep", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}

	if !strings.Contains(out, "type: socks5") || !strings.Contains(out, "server: 127.0.0.1") {
		t.Fatalf("expected socks5 dummy node in clash YAML, got:\n%s", out)
	}
	if !strings.Contains(out, "Traffic Depleted") || !strings.Contains(out, "dep@test.com") {
		t.Fatalf("expected depleted remark in clash YAML, got:\n%s", out)
	}
	if strings.Contains(out, "Germany-VLESS") {
		t.Fatalf("depleted subscription must NOT contain working inbound, got:\n%s", out)
	}
}
