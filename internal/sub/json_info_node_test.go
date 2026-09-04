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

func TestSubJson_InfoNode_Active(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id:       1,
		UserId:   1,
		Remark:   "Germany-VLESS",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"c1-uuid","email":"user1@test.com","subId":"sub-json","enable":true,"totalGB":10737418240}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:      1,
		Email:   "user1@test.com",
		SubID:   "sub-json",
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
	jsonSvc := NewSubJsonService("", "", "", sub)

	out, _, err := jsonSvc.GetJson("sub-json", "sub.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}

	if !strings.Contains(out, "user1@test.com|📊8.00GB") {
		t.Fatalf("expected dummy remark in JSON remarks, got:\n%s", out)
	}
	if !strings.Contains(out, `"protocol": "socks"`) && !strings.Contains(out, `"protocol":"socks"`) {
		t.Fatalf("expected socks outbound in JSON, got:\n%s", out)
	}
}

func TestSubJson_InfoNode_Expired(t *testing.T) {
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
		Settings: fmt.Sprintf(`{"clients":[{"id":"c1-uuid","email":"exp@test.com","subId":"sub-json-exp","enable":true,"expiryTime":%d}]}`, expiredTime),
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:         1,
		Email:      "exp@test.com",
		SubID:      "sub-json-exp",
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
	jsonSvc := NewSubJsonService("", "", "", sub)

	out, _, err := jsonSvc.GetJson("sub-json-exp", "sub.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}

	if !strings.Contains(out, "Expired") || !strings.Contains(out, "exp@test.com") {
		t.Fatalf("expected expired remark in JSON remarks, got:\n%s", out)
	}
	if strings.Contains(out, "Germany-VLESS") {
		t.Fatalf("expired subscription must NOT contain working inbound in JSON, got:\n%s", out)
	}
}

func TestSubJson_InfoNode_Depleted(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id:       1,
		UserId:   1,
		Remark:   "Germany-VLESS",
		Enable:   true,
		Port:     443,
		Protocol: model.VLESS,
		Settings: `{"clients":[{"id":"c1-uuid","email":"dep@test.com","subId":"sub-json-dep","enable":true,"totalGB":5368709120}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:      1,
		Email:   "dep@test.com",
		SubID:   "sub-json-dep",
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
	jsonSvc := NewSubJsonService("", "", "", sub)

	out, _, err := jsonSvc.GetJson("sub-json-dep", "sub.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}

	if !strings.Contains(out, "Traffic Depleted") || !strings.Contains(out, "dep@test.com") {
		t.Fatalf("expected depleted remark in JSON remarks, got:\n%s", out)
	}
	if strings.Contains(out, "Germany-VLESS") {
		t.Fatalf("depleted subscription must NOT contain working inbound in JSON, got:\n%s", out)
	}
}
