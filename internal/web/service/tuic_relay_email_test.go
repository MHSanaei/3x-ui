package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestTuicClientAddRejectsOversizeEmailBeforeWriting(t *testing.T) {
	for _, email := range []string{strings.Repeat("a", 256), strings.Repeat("я", 128)} {
		t.Run(email[:2], func(t *testing.T) {
			setupBulkDB(t)
			inbound := mkInbound(t, 45421, model.TUIC, `{"clients":[]}`)
			client := model.Client{Email: email, ID: uuid.NewString(), Password: "secret", Enable: true}
			payload := &model.Inbound{Id: inbound.Id, Settings: clientsSettings(t, []model.Client{client})}
			_, err := (&ClientService{}).AddInboundClient(&InboundService{}, payload)
			if err == nil || !strings.Contains(err.Error(), "255") {
				t.Fatalf("oversized TUIC email was not rejected at save: %v", err)
			}
			var count int64
			if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", email).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("rejected client was persisted: count=%d err=%v", count, err)
			}
		})
	}
}

func TestTuicLegacyOversizeEmailDoesNotDisableItsHealthySibling(t *testing.T) {
	setupBulkDB(t)
	clients := []model.Client{
		{Email: strings.Repeat("x", 256), ID: uuid.NewString(), Password: "long", Enable: true},
		{Email: "healthy@test", ID: uuid.NewString(), Password: "short", Enable: true},
	}
	inbound := mkInbound(t, 45422, model.TUIC, clientsSettings(t, clients))
	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	var accounts []relayAccount
	for _, relay := range cfg.InboundConfigs {
		if relay.TuicInboundID != inbound.Id {
			continue
		}
		var settings struct {
			Accounts []relayAccount `json:"accounts"`
		}
		if err := json.Unmarshal(relay.Settings, &settings); err != nil {
			t.Fatal(err)
		}
		accounts = settings.Accounts
	}
	if len(accounts) != 1 || accounts[0].User != "healthy@test" {
		t.Fatalf("legacy invalid account contaminated the relay: got %d accounts", len(accounts))
	}
	instances, err := (&InboundService{}).DesiredTuicInstances()
	if err != nil || len(instances) != 1 || len(instances[0].Clients) != 1 || instances[0].Clients[0].Email != "healthy@test" {
		t.Fatalf("native admission disagrees with relay accounts: %+v err=%v", instances, err)
	}
}

func TestTuicSharedRenameCannotBypassEmailLimitWithInboundFilter(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	client := model.Client{Email: "shared@test", ID: uuid.NewString(), Password: "secret", SubID: "shared-sub", Enable: true}
	tuicInbound := mkInbound(t, 45423, model.TUIC, clientsSettings(t, []model.Client{client}))
	vless := mkInbound(t, 45424, model.VLESS, clientsSettings(t, []model.Client{client}))
	for _, inbound := range []*model.Inbound{tuicInbound, vless} {
		if err := svc.SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
			t.Fatal(err)
		}
	}
	stored := lookupClientRecord(t, client.Email)
	updated := client
	updated.Email = strings.Repeat("я", 128)
	_, err := svc.Update(&InboundService{}, stored.Id, updated, 0, vless.Id)
	if err == nil || !strings.Contains(err.Error(), "255") {
		t.Fatalf("filtered VLESS edit bypassed the attached TUIC email limit: %v", err)
	}
	var current model.ClientRecord
	if err := database.GetDB().First(&current, stored.Id).Error; err != nil || current.Email != client.Email {
		t.Fatalf("rejected shared rename mutated the identity: email=%q err=%v", current.Email, err)
	}
}

func TestTuicEmailBoundaryAndOtherProtocolsRemainUsable(t *testing.T) {
	for _, tc := range []struct {
		protocol model.Protocol
		email    string
	}{
		{model.TUIC, strings.Repeat("a", 255)},
		{model.TUIC, strings.Repeat("я", 127) + "a"},
		{model.VLESS, strings.Repeat("a", 256)},
	} {
		t.Run(string(tc.protocol)+"-"+tc.email[:2], func(t *testing.T) {
			setupBulkDB(t)
			inbound := mkInbound(t, 45425, tc.protocol, `{"clients":[]}`)
			client := model.Client{Email: tc.email, ID: uuid.NewString(), Password: "secret", Enable: true}
			_, err := (&ClientService{}).AddInboundClient(&InboundService{}, &model.Inbound{Id: inbound.Id, Settings: clientsSettings(t, []model.Client{client})})
			if err != nil {
				t.Fatal(err)
			}
			stored := lookupClientRecord(t, tc.email)
			if stored.Email != tc.email || stored.UUID != client.ID {
				t.Fatal("accepted client identity was not preserved")
			}
		})
	}
}
