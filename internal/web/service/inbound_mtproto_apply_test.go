package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func mtgConfigPath(t *testing.T, inboundId int) string {
	t.Helper()
	return filepath.Join(os.Getenv("XUI_BIN_FOLDER"), "mtproto", fmt.Sprintf("mtg-%d.toml", inboundId))
}

func readMtgConfig(t *testing.T, inboundId int) string {
	t.Helper()
	data, err := os.ReadFile(mtgConfigPath(t, inboundId))
	if err != nil {
		t.Fatalf("read mtg config: %v", err)
	}
	return string(data)
}

func TestUpdateInboundMtprotoUnchangedDoesNotRestart(t *testing.T) {
	setupConflictDB(t)
	pidFile := installFakeMtg(t)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "mt-apply", "", 46101, model.MTProto,
		"",
		`{"clients":[`+
			`{"email":"mtga","secret":"`+mtprotoTestSecretA+`","enable":true},`+
			`{"email":"mtgb","secret":"`+mtprotoTestSecretB+`","enable":true}]}`)
	seeded := loadInboundByTag(t, "mt-apply")
	seedClientTraffic(t, seeded.Id, "mtga", true)
	seedClientTraffic(t, seeded.Id, "mtgb", true)

	svc := &InboundService{}
	primed, ok := mtproto.InstanceFromInbound(seeded)
	if !ok {
		t.Fatal("seed inbound must produce an mtg instance")
	}
	if err := mtproto.GetManager().Ensure(primed); err != nil {
		t.Fatalf("prime mtg: %v", err)
	}
	t.Cleanup(func() { mtproto.GetManager().Remove(seeded.Id) })
	waitForSpawns(t, pidFile, 1)
	primedConfig := readMtgConfig(t, seeded.Id)

	saveAndAssertKept := func(t *testing.T, mutate func(*model.Inbound)) {
		t.Helper()
		update := *loadInboundByTag(t, "mt-apply")
		mutate(&update)
		_, needRestart, err := svc.UpdateInbound(&update)
		if err != nil {
			t.Fatalf("UpdateInbound: %v", err)
		}
		if needRestart {
			t.Fatal("an mtproto-only edit must not request an xray restart")
		}
		assertNoNewSpawns(t, pidFile, 1)
		if got := readMtgConfig(t, seeded.Id); got != primedConfig {
			t.Fatalf("config rewritten on a no-op edit:\nbefore:\n%s\nafter:\n%s", primedConfig, got)
		}
	}

	t.Run("unchangedSaveKeepsProcess", func(t *testing.T) {
		saveAndAssertKept(t, func(*model.Inbound) {})
	})

	t.Run("remarkOnlyEditKeepsProcess", func(t *testing.T) {
		saveAndAssertKept(t, func(ib *model.Inbound) { ib.Remark = "renamed while users stay connected" })
	})

	t.Run("rekeyedSecretRestartsProcess", func(t *testing.T) {
		update := *loadInboundByTag(t, "mt-apply")
		update.Settings = strings.Replace(update.Settings, mtprotoTestSecretA, mtprotoTestSecretD, 1)
		if !strings.Contains(update.Settings, mtprotoTestSecretD) {
			t.Fatal("fixture must contain the re-keyed secret")
		}
		_, needRestart, err := svc.UpdateInbound(&update)
		if err != nil {
			t.Fatalf("UpdateInbound: %v", err)
		}
		if needRestart {
			t.Fatal("an mtproto secret change must not request an xray restart")
		}
		waitForSpawns(t, pidFile, 2)
		if got := readMtgConfig(t, seeded.Id); !strings.Contains(got, mtprotoTestSecretD) {
			t.Fatalf("restarted config must carry the new secret:\n%s", got)
		}
	})
}
