package sub

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func setupInfoNodeTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(t.TempDir() + "/test_infonode.db"); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		_ = database.CloseDB()
	})
	db := database.GetDB()
	if err := db.AutoMigrate(
		&model.Inbound{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&xray.ClientTraffic{},
		&model.Setting{},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
}

func TestSubService_InfoNode_Active(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id:             1,
		UserId:         1,
		Up:             0,
		Down:           0,
		Total:          0,
		Remark:         "Germany-VLESS",
		Enable:         true,
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"c1-uuid","email":"user1@test.com","subId":"sub-active","enable":true,"totalGB":10737418240,"expiryTime":0}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:      1,
		Email:   "user1@test.com",
		SubID:   "sub-active",
		UUID:    "c1-uuid",
		Enable:  true,
		TotalGB: 10737418240, // 10 GB
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
		Up:        1073741824, // 1 GB
		Down:      1073741824, // 1 GB
		Total:     10737418240,
		Enable:    true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewSubService("{{EMAIL}}|📊{{TRAFFIC_LEFT}}")
	svc.subInfoNodeEnable = true
	svc.subscriptionBody = true

	links, emails, _, traffic, err := svc.GetSubs("sub-active", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links (dummy info node + 1 vless link), got %d: %v", len(links), links)
	}
	if len(emails) != 1 || emails[0] != "user1@test.com" {
		t.Fatalf("emails = %v, want [user1@test.com]", emails)
	}
	if !traffic.Enable {
		t.Fatalf("traffic should be enabled")
	}

	// First link must be the dummy socks node
	if !strings.HasPrefix(links[0], "socks://127.0.0.1:1080#") {
		t.Fatalf("first link must be dummy socks node, got: %q", links[0])
	}
	decodedRemark, _ := url.QueryUnescape(strings.TrimPrefix(links[0], "socks://127.0.0.1:1080#"))
	if !strings.Contains(decodedRemark, "user1@test.com") || !strings.Contains(decodedRemark, "8.00GB") {
		t.Fatalf("expected dummy remark to contain user1@test.com and 8.00GB, got: %q", decodedRemark)
	}

	// Second link must be the clean vless link without traffic left tokens
	if !strings.HasPrefix(links[1], "vless://") {
		t.Fatalf("second link must be vless, got: %q", links[1])
	}
	if strings.Contains(links[1], "8.00GB") {
		t.Fatalf("server link must have clean remark without usage stats, got: %q", links[1])
	}
}

func TestSubService_InfoNode_Expired(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	expiredTime := time.Now().Add(-24 * time.Hour).UnixMilli()
	ib := &model.Inbound{
		Id:             1,
		UserId:         1,
		Remark:         "Germany-VLESS",
		Enable:         true,
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       fmt.Sprintf(`{"clients":[{"id":"c1-uuid","email":"exp@test.com","subId":"sub-exp","enable":true,"expiryTime":%d}]}`, expiredTime),
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:         1,
		Email:      "exp@test.com",
		SubID:      "sub-exp",
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

	svc := NewSubService("{{INBOUND}}|{{EMAIL}}")
	svc.subInfoNodeEnable = true
	svc.subExpiredTemplate = service.DefaultSubExpiredTemplate
	svc.subscriptionBody = true

	links, _, _, _, err := svc.GetSubs("sub-exp", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected ONLY 1 link (dummy expired node), got %d: %v", len(links), links)
	}
	if !strings.HasPrefix(links[0], "socks://127.0.0.1:1080#") {
		t.Fatalf("expired link must be dummy socks node, got: %q", links[0])
	}
	decodedRemark, _ := url.QueryUnescape(strings.TrimPrefix(links[0], "socks://127.0.0.1:1080#"))
	if !strings.Contains(decodedRemark, "Expired") || !strings.Contains(decodedRemark, "exp@test.com") {
		t.Fatalf("expected expired remark, got: %q", decodedRemark)
	}
}

func TestSubService_InfoNode_TrafficDepleted(t *testing.T) {
	setupInfoNodeTestDB(t)
	db := database.GetDB()

	ib := &model.Inbound{
		Id:             1,
		UserId:         1,
		Remark:         "Germany-VLESS",
		Enable:         true,
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"c1-uuid","email":"dep@test.com","subId":"sub-dep","enable":true,"totalGB":5368709120}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{
		Id:      1,
		Email:   "dep@test.com",
		SubID:   "sub-dep",
		UUID:    "c1-uuid",
		Enable:  true,
		TotalGB: 5368709120, // 5 GB
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
		Up:        3221225472, // 3 GB
		Down:      2147483648, // 2 GB (total 5 GB used = depleted)
		Total:     5368709120,
		Enable:    true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewSubService("{{INBOUND}}|{{EMAIL}}")
	svc.subInfoNodeEnable = true
	svc.subTrafficDepletedTemplate = service.DefaultSubTrafficDepletedTemplate
	svc.subscriptionBody = true

	links, _, _, _, err := svc.GetSubs("sub-dep", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected ONLY 1 link (dummy depleted node), got %d: %v", len(links), links)
	}
	if !strings.HasPrefix(links[0], "socks://127.0.0.1:1080#") {
		t.Fatalf("depleted link must be dummy socks node, got: %q", links[0])
	}
	decodedRemark, _ := url.QueryUnescape(strings.TrimPrefix(links[0], "socks://127.0.0.1:1080#"))
	if !strings.Contains(decodedRemark, "Traffic Depleted") || !strings.Contains(decodedRemark, "dep@test.com") {
		t.Fatalf("expected depleted remark, got: %q", decodedRemark)
	}
}
