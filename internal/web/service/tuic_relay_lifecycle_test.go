package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTuicFailedReconcileRetriesWithoutRequestingCoreRestart(t *testing.T) {
	setupBulkDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", `{broken`); err != nil {
		t.Fatal(err)
	}
	wasStopped := isManuallyStopped.Swap(false)
	wasRestart := isNeedXrayRestart.Swap(false)
	wasReconcile := isNeedXrayReconcile.Swap(false)
	t.Cleanup(func() {
		isManuallyStopped.Store(wasStopped)
		isNeedXrayRestart.Store(wasRestart)
		isNeedXrayReconcile.Store(wasReconcile)
	})
	svc := &XrayService{}
	rt := runtime.NewLocal(runtime.LocalDeps{SetNeedRestart: svc.SetToNeedRestart, RequestReconcile: svc.SetToNeedReconcile})
	inbound := &model.Inbound{Id: 940004, Protocol: model.TUIC}
	if err := rt.UpdateInbound(context.Background(), inbound, inbound); err != nil {
		t.Fatal(err)
	}
	svc.ApplyPendingRestart()
	if isNeedXrayRestart.Load() || !isNeedXrayReconcile.Load() {
		t.Fatalf("failed TUIC reconciliation became a core restart or lost retry: restart=%t reconcile=%t", isNeedXrayRestart.Load(), isNeedXrayReconcile.Load())
	}
}

func TestTuicQuotaResetDoesNotChangeRelayAdmission(t *testing.T) {
	for _, reset := range []string{"telegram", "scheduled"} {
		t.Run(reset, func(t *testing.T) {
			setupBulkDB(t)
			client := model.Client{Email: "renewed@test", ID: uuid.NewString(), Password: "secret", Enable: true}
			ib := mkInbound(t, 45381, model.TUIC, clientsSettings(t, []model.Client{client}))
			svc := &InboundService{}
			if err := (&ClientService{}).SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
				t.Fatal(err)
			}
			if err := svc.AddClientStat(database.GetDB(), ib.Id, &client); err != nil {
				t.Fatal(err)
			}
			if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", client.Email).Update("enable", false).Error; err != nil {
				t.Fatal(err)
			}
			before, err := (&XrayService{}).GetXrayConfig()
			if err != nil {
				t.Fatal(err)
			}
			if reset == "telegram" {
				err = svc.ResetClientTrafficByEmail(client.Email)
			} else {
				_, err = (&ClientService{}).ResetAllClientTraffics(svc, ib.Id)
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := (&XrayService{}).GetXrayConfig()
			if err != nil {
				t.Fatal(err)
			}
			if !before.Equals(after) {
				t.Fatal("quota reset changed Xray relay admission; the renewed TUIC client needs an unrelated Xray reload")
			}
			instances, err := svc.DesiredTuicInstances()
			if err != nil || len(instances) != 1 || len(instances[0].Clients) != 1 || instances[0].Clients[0].Email != client.Email {
				t.Fatalf("renewed client missing from native admission: %+v, %v", instances, err)
			}
		})
	}
}

func TestTuicClientChurnCanApplyWithoutRestartingOtherProtocols(t *testing.T) {
	setupBulkDB(t)
	clients := []model.Client{{Email: "first@test", ID: uuid.NewString(), Password: "secret", Enable: true}}
	ib := mkInbound(t, 45382, model.TUIC, clientsSettings(t, clients))
	svc := &XrayService{}
	if err := (&SettingService{}).SetRestartXrayOnClientDisable(true); err != nil {
		t.Fatal(err)
	}
	before, err := svc.GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	clients = append(clients, model.Client{Email: "новый@test", ID: uuid.NewString(), Password: "second", Enable: true})
	for _, step := range []string{"add", "remove", "empty"} {
		switch step {
		case "remove":
			clients = clients[1:]
		case "empty":
			clients = nil
		}
		if err := database.GetDB().Model(ib).Update("settings", clientsSettings(t, clients)).Error; err != nil {
			t.Fatal(err)
		}
		after, err := svc.GetXrayConfig()
		if err != nil {
			t.Fatal(err)
		}
		diff, hot := xray.ComputeHotDiff(before, after)
		if !hot || diff.Empty() || svc.restartToDropClients(diff) {
			t.Fatalf("TUIC %s would restart unrelated protocols: hot=%t diff=%+v", step, hot, diff)
		}
		before = after
	}
}

func TestBrokenTuicInboundDoesNotBlockOtherInbounds(t *testing.T) {
	for _, fault := range []string{"invalid-settings", "invalid-shape", "template-tag"} {
		t.Run(fault, func(t *testing.T) {
			setupBulkDB(t)
			ordinary := mkInbound(t, 45383, model.VLESS, `{"decryption":"none","clients":[]}`)
			broken := mkInbound(t, 45384, model.TUIC, clientsSettings(t, []model.Client{{Email: "broken@test", ID: uuid.NewString(), Password: "secret", Enable: true}}))
			switch fault {
			case "invalid-settings":
				broken.Settings = `{broken`
			case "invalid-shape":
				broken.Settings = `{"server":"broken"}`
			case "template-tag":
				broken.Tag = "template-control"
				template := map[string]any{"inbounds": []any{map[string]any{"tag": broken.Tag, "listen": "127.0.0.1", "port": 45385, "protocol": "vless", "settings": map[string]any{"decryption": "none", "clients": []any{}}}}, "outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}}}
				if err := (&SettingService{}).saveSetting("xrayTemplateConfig", string(mustJSON(t, template))); err != nil {
					t.Fatal(err)
				}
			}
			if err := database.GetDB().Save(broken).Error; err != nil {
				t.Fatal(err)
			}
			cfg, err := (&XrayService{}).GetXrayConfig()
			if err != nil {
				t.Fatalf("one broken TUIC row blocked all protocols: %v", err)
			}
			count := 0
			for _, inbound := range cfg.InboundConfigs {
				if inbound.Tag == ordinary.Tag {
					count++
					if inbound.Protocol != "vless" || inbound.Port != ordinary.Port {
						t.Fatalf("TUIC replaced the unrelated inbound: %+v", inbound)
					}
				}
			}
			if count != 1 {
				t.Fatalf("unrelated inbound appears %d times, want once", count)
			}
		})
	}
}

func TestTuicCannotSaveTemplateReservedTag(t *testing.T) {
	setupBulkDB(t)
	template := `{"api":{"tag":"management"},"inbounds":[{"tag":"template-control","listen":"127.0.0.1","port":45385,"protocol":"vless","settings":{"decryption":"none","clients":[]}}],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", template); err != nil {
		t.Fatal(err)
	}
	for i, tag := range []string{"api", "management", "template-control"} {
		inbound := &model.Inbound{Tag: tag, Protocol: model.TUIC, Listen: "127.0.0.1", Port: 45390 + i, Settings: `{"clients":[]}`}
		saved, _, err := (&InboundService{}).AddInbound(inbound)
		if err != nil {
			t.Fatal(err)
		}
		var stored model.Inbound
		if err := database.GetDB().First(&stored, saved.Id).Error; err != nil {
			t.Fatal(err)
		}
		if stored.Tag == tag || stored.Tag != saved.Tag {
			t.Fatalf("reserved tag %q was accepted or API returned a different tag: saved=%q stored=%q", tag, saved.Tag, stored.Tag)
		}
	}
}
