package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedInbound(t *testing.T, remark string, port int) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		Tag:      fmt.Sprintf("%s-%d-tag", remark, port),
		Remark:   remark,
		Enable:   true,
		Port:     port,
		Protocol: model.VLESS,
		Settings: `{"clients":[]}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	return ib
}

func reloadClients(t *testing.T, inboundID int) []model.Client {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().First(&ib, inboundID).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	clients, err := (&InboundService{}).GetClients(&ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	return clients
}

func TestEnsureUserClient_CreatesThenReuses(t *testing.T) {
	setupBulkDB(t)
	ib := seedInbound(t, "oauth-users", 24101)
	prov := &OAuthProvisionService{}
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	cfg := config.OAuthConfig{UserInboundRemarks: []string{"oauth-users"}, UserTotalGB: 10, UserExpiryDays: 30, UserLimitIP: 2}

	sub1, _, err := prov.EnsureUserClient(inboundSvc, clientSvc, cfg, "alice@corp")
	if err != nil {
		t.Fatalf("first EnsureUserClient: %v", err)
	}
	if sub1 == "" {
		t.Fatal("first login returned empty subId")
	}

	clients := reloadClients(t, ib.Id)
	if len(clients) != 1 {
		t.Fatalf("client count after first login = %d, want 1", len(clients))
	}
	got := clients[0]
	if got.Email != "alice@corp" || got.SubID != sub1 {
		t.Fatalf("client = %s/%s, want alice@corp/%s", got.Email, got.SubID, sub1)
	}
	if got.TotalGB != 10*1024*1024*1024 {
		t.Errorf("TotalGB = %d, want 10GiB", got.TotalGB)
	}
	if got.LimitIP != 2 {
		t.Errorf("LimitIP = %d, want 2", got.LimitIP)
	}
	if got.ExpiryTime == 0 {
		t.Error("ExpiryTime = 0, want a future expiry from 30 days")
	}
	if !got.Enable {
		t.Error("client is not enabled")
	}

	sub2, _, err := prov.EnsureUserClient(inboundSvc, clientSvc, cfg, "alice@corp")
	if err != nil {
		t.Fatalf("second EnsureUserClient: %v", err)
	}
	if sub2 != sub1 {
		t.Fatalf("second login subId = %q, want stable %q", sub2, sub1)
	}
	if clients := reloadClients(t, ib.Id); len(clients) != 1 {
		t.Fatalf("client count after second login = %d, want 1 (idempotent)", len(clients))
	}
}

func TestEnsureUserClient_AttachesToEveryMatchingRemark(t *testing.T) {
	setupBulkDB(t)
	ib1 := seedInbound(t, "vpn", 24201)
	ib2 := seedInbound(t, "vpn", 24202)
	seedInbound(t, "other", 24203)
	prov := &OAuthProvisionService{}
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	cfg := config.OAuthConfig{UserInboundRemarks: []string{"vpn"}}

	sub, _, err := prov.EnsureUserClient(inboundSvc, clientSvc, cfg, "bob@corp")
	if err != nil {
		t.Fatalf("EnsureUserClient: %v", err)
	}
	for _, ib := range []*model.Inbound{ib1, ib2} {
		clients := reloadClients(t, ib.Id)
		if len(clients) != 1 || clients[0].Email != "bob@corp" || clients[0].SubID != sub {
			t.Fatalf("inbound %q clients = %+v, want bob@corp/%s", ib.Remark, clients, sub)
		}
	}
}

func TestEnsureUserClient_Errors(t *testing.T) {
	setupBulkDB(t)
	seedInbound(t, "oauth-users", 24101)
	prov := &OAuthProvisionService{}
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}

	t.Run("empty email", func(t *testing.T) {
		if _, _, err := prov.EnsureUserClient(inboundSvc, clientSvc, config.OAuthConfig{UserInboundRemarks: []string{"oauth-users"}}, ""); err == nil {
			t.Fatal("want error for empty email")
		}
	})
	t.Run("no inbound remark configured", func(t *testing.T) {
		if _, _, err := prov.EnsureUserClient(inboundSvc, clientSvc, config.OAuthConfig{}, "a@b"); err == nil {
			t.Fatal("want error for missing inbound remark")
		}
	})
	t.Run("inbound remark not found", func(t *testing.T) {
		if _, _, err := prov.EnsureUserClient(inboundSvc, clientSvc, config.OAuthConfig{UserInboundRemarks: []string{"nope"}}, "a@b"); err == nil {
			t.Fatal("want error for unknown inbound remark")
		}
	})
}
