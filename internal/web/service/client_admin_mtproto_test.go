package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAdminMTProtoRenewalCompletesOnOldAndNewSidecarsWithoutReplay(t *testing.T) {
	for _, old := range []bool{false, true} {
		t.Run(strconv.FormatBool(old), func(t *testing.T) {
			if old {
				t.Setenv("MTG_FAKE_RESET_UNSUPPORTED", "1")
			}
			port, err := mtproto.FreeLocalPort()
			if err != nil {
				t.Fatal(err)
			}
			_, logPath := startQuotaSidecar(t, port, model.Client{Enable: true, TotalGB: 1000})
			svc := &ClientAdminWorkflowService{}
			op, err := svc.PrepareRenew(77, "mtga", ClientRenewalSpec{Mode: "replace", QuotaBytes: 3000, Days: 30})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
				t.Fatal(err)
			}
			pidFile := filepath.Join(filepath.Dir(logPath), "mtg-pids.txt")
			wantSpawns := 1
			if old {
				wantSpawns = 2
			}
			waitForSpawns(t, pidFile, wantSpawns)
			receipt, err := svc.Load(op.Token, 77)
			if err != nil || receipt.State != "done" {
				t.Fatalf("MTProto renewal stayed pending: %+v %v", receipt, err)
			}
			if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "mtga").Update("up", 321).Error; err != nil {
				t.Fatal(err)
			}
			for range 3 {
				if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
					t.Fatal(err)
				}
			}
			if traffic := readTraffic(t, database.GetDB(), "mtga"); traffic.Up != 321 || traffic.Total != 3000 {
				t.Fatalf("MTProto replay reset new usage: %+v", traffic)
			}
			if got := countSpawns(t, pidFile); got != wantSpawns {
				t.Fatalf("completed receipt restarted sidecar again: %d", got)
			}
			if !old {
				if got := quotaResets(t, logPath); len(got) != 1 || got[0] != "mtga" {
					t.Fatalf("reset replayed: %v", got)
				}
			}
		})
	}
}

func TestOlderCommittedRenewalDoesNotResetANewerMTProtoPackage(t *testing.T) {
	statusFile := filepath.Join(t.TempDir(), "reset-status")
	t.Setenv("MTG_FAKE_RESET_STATUS_FILE", statusFile)
	if err := os.WriteFile(statusFile, []byte("503"), 0o600); err != nil {
		t.Fatal(err)
	}
	port, err := mtproto.FreeLocalPort()
	if err != nil {
		t.Fatal(err)
	}
	_, logPath := startQuotaSidecar(t, port, model.Client{Enable: true, TotalGB: 1000})
	svc := &ClientAdminWorkflowService{}
	op, err := svc.PrepareRenew(77, "mtga", ClientRenewalSpec{Mode: "replace", QuotaBytes: 3000, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); !errors.Is(err, ErrClientOperationPending) {
		t.Fatalf("expected failed quota delivery: %v", err)
	}
	if err := os.WriteFile(statusFile, []byte("200"), 0o600); err != nil {
		t.Fatal(err)
	}
	next, err := svc.PrepareRenew(77, "mtga", ClientRenewalSpec{Mode: "replace", QuotaBytes: 5000, Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), next.Token, 77); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "mtga").Update("up", 321).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(context.Background(), op.Token, 77); err != nil {
		t.Fatal(err)
	}
	if got := quotaResets(t, logPath); len(got) != 1 || got[0] != "mtga" {
		t.Fatalf("older receipt reset newer runtime counters: %v", got)
	}
	if got := readTraffic(t, database.GetDB(), "mtga"); got.Up != 321 || got.Total != 5000 {
		t.Fatalf("older receipt reapplied accounting: %+v", got)
	}
}
