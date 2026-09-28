package service

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// startQuotaSidecar runs a local MTProto inbound for mtga and mtgb under the fake
// mtg and returns its API log once the sidecar answers.
func startQuotaSidecar(t *testing.T, port int, mtga model.Client) (*model.Inbound, string) {
	t.Helper()
	setupConflictDB(t)
	pidFile, logPath := installFakeMtgAPI(t)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	mtga.Email, mtga.Secret = "mtga", mtprotoTestSecretA
	clients := []model.Client{mtga, {Email: "mtgb", Secret: mtprotoTestSecretB, Enable: true}}
	ib := &model.Inbound{Tag: "mt-quota", Enable: true, Port: port, Protocol: model.MTProto, Settings: clientsSettings(t, clients)}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	for _, c := range clients {
		row := xray.ClientTraffic{InboundId: ib.Id, Email: c.Email, Enable: true, Up: 5, Total: c.TotalGB, ExpiryTime: c.ExpiryTime, Reset: c.Reset}
		if err := database.GetDB().Create(&row).Error; err != nil {
			t.Fatalf("seed traffic: %v", err)
		}
	}
	// A running sidecar needs a served client, so prime with the healthy set.
	inst, ok := mtproto.InstanceFromInbound(&model.Inbound{
		Id: ib.Id, Tag: ib.Tag, Port: port, Protocol: model.MTProto,
		Settings: clientsSettings(t, []model.Client{{Email: "mtgb", Secret: mtprotoTestSecretB, Enable: true}}),
	})
	if !ok {
		t.Fatal("seed inbound must produce an mtg instance")
	}
	if err := mtproto.GetManager().Ensure(inst); err != nil {
		t.Fatalf("start mtg: %v", err)
	}
	t.Cleanup(func() { mtproto.GetManager().Remove(ib.Id) })
	waitForSpawns(t, pidFile, 1)
	waitFakeMtgLog(t, logPath, "ready")
	return ib, logPath
}

func quotaResets(t *testing.T, logPath string) []string {
	t.Helper()
	var out []string
	for _, line := range fakeMtgLog(t, logPath) {
		if name, ok := strings.CutPrefix(line, "reset:"); ok {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Every path that zeroes a client's panel counters must zero the sidecar's own
// quota counter too, or the sidecar keeps refusing the client.
func TestPanelResetsZeroSidecarQuota(t *testing.T) {
	t.Run("bulk reset", func(t *testing.T) {
		_, logPath := startQuotaSidecar(t, 46201, model.Client{Enable: true})
		if _, err := (&ClientService{}).BulkResetTraffic(&InboundService{}, []string{"mtga"}); err != nil {
			t.Fatalf("BulkResetTraffic: %v", err)
		}
		if got := quotaResets(t, logPath); !slices.Equal(got, []string{"mtga"}) {
			t.Fatalf("sidecar quota resets %v, want [mtga]", got)
		}
	})
	t.Run("inbound clients", func(t *testing.T) {
		ib, logPath := startQuotaSidecar(t, 46207, model.Client{Enable: true})
		if err := (&ClientService{}).ResetAllClientTraffics(&InboundService{}, ib.Id); err != nil {
			t.Fatalf("ResetAllClientTraffics: %v", err)
		}
		if got := quotaResets(t, logPath); !slices.Equal(got, []string{"mtga", "mtgb"}) {
			t.Fatalf("sidecar quota resets %v, want [mtga mtgb]", got)
		}
	})
	t.Run("reset all", func(t *testing.T) {
		_, logPath := startQuotaSidecar(t, 46202, model.Client{Enable: true})
		if _, err := (&ClientService{}).ResetAllTraffics(); err != nil {
			t.Fatalf("ResetAllTraffics: %v", err)
		}
		if got := quotaResets(t, logPath); !slices.Equal(got, []string{"mtga", "mtgb"}) {
			t.Fatalf("sidecar quota resets %v, want [mtga mtgb]", got)
		}
	})
	t.Run("auto renew", func(t *testing.T) {
		expired := time.Now().Add(-time.Hour).UnixMilli()
		_, logPath := startQuotaSidecar(t, 46203, model.Client{Enable: true, Reset: 30, ExpiryTime: expired})
		if _, _, err := (&InboundService{}).AddTraffic(nil, nil); err != nil {
			t.Fatalf("AddTraffic: %v", err)
		}
		if got := quotaResets(t, logPath); !slices.Equal(got, []string{"mtga"}) {
			t.Fatalf("sidecar quota resets %v, want [mtga]", got)
		}
	})
}

// Resetting inbound counters leaves every client's usage in place, so the
// sidecar's quota counters must stay too or clients get their quota again free.
func TestInboundResetAllKeepsSidecarQuota(t *testing.T) {
	_, logPath := startQuotaSidecar(t, 46204, model.Client{Enable: true})
	if err := (&InboundService{}).ResetAllTraffics(); err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}
	if got := quotaResets(t, logPath); len(got) != 0 {
		t.Fatalf("inbound reset zeroed sidecar quotas %v, want none", got)
	}
}

// Resetting one inbound's clients zeroes only their sidecar quotas, not those
// of MTProto clients whose usage the reset left in place.
func TestInboundClientResetKeepsOtherSidecarQuotas(t *testing.T) {
	_, logPath := startQuotaSidecar(t, 46205, model.Client{Enable: true})
	other := mkInbound(t, 46206, model.VLESS, clientsSettings(t, []model.Client{{Email: "vless-only", ID: "11111111-1111-1111-1111-1111111111ab", Enable: true}}))
	if err := (&ClientService{}).SyncInbound(nil, other.Id, []model.Client{{Email: "vless-only", ID: "11111111-1111-1111-1111-1111111111ab", Enable: true}}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	if err := (&ClientService{}).ResetAllClientTraffics(&InboundService{}, other.Id); err != nil {
		t.Fatalf("ResetAllClientTraffics: %v", err)
	}
	if got := quotaResets(t, logPath); len(got) != 0 {
		t.Fatalf("resetting another inbound zeroed sidecar quotas %v, want none", got)
	}
}
