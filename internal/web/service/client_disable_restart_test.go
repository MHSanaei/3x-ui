package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The manual switch applies through the runtime, and the core's API removal
// drops the credential only: whether the live session ends is what the setting
// asks for, exactly as on the auto-disable path #6533 reports from.
func TestManualClientDisableHonoursRestartSetting(t *testing.T) {
	const email = "manual-disable@example.com"

	for _, tc := range []struct {
		name    string
		setting bool
		want    bool
	}{
		{"setting on", true, true},
		{"setting off", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			setRestartOnClientDisable(t, tc.setting)

			mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
			mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
			runtime.SetManager(mgr)
			t.Cleanup(func() { runtime.SetManager(nil) })

			seedInboundConflict(t, "manual-disable", "0.0.0.0", 50055, model.VLESS, `{"network":"tcp"}`,
				`{"clients":[{"email":"`+email+`","id":"5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a333","enable":true}]}`)
			inbound := loadInboundByTag(t, "manual-disable")

			inboundSvc := InboundService{}
			clientSvc := ClientService{}
			clients, err := inboundSvc.GetClients(inbound)
			if err != nil {
				t.Fatalf("GetClients: %v", err)
			}
			if err := clientSvc.SyncInbound(nil, inbound.Id, clients); err != nil {
				t.Fatalf("SyncInbound: %v", err)
			}
			if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: email, Enable: true}).Error; err != nil {
				t.Fatalf("seed traffic: %v", err)
			}

			changed, needRestart, err := clientSvc.SetClientEnableByEmail(&inboundSvc, email, false)
			if err != nil {
				t.Fatalf("SetClientEnableByEmail: %v", err)
			}
			if !changed {
				t.Fatal("the disable must be recorded")
			}
			if needRestart != tc.want {
				t.Fatalf("needRestart = %v, want %v", needRestart, tc.want)
			}
		})
	}
}
