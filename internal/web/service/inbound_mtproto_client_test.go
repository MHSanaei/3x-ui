package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestClientCrudMtprotoAppliesImmediately(t *testing.T) {
	setupConflictDB(t)
	pidFile := installFakeMtg(t)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}

	created, _, err := inboundSvc.AddInbound(&model.Inbound{
		Enable:   true,
		Listen:   "",
		Port:     46201,
		Protocol: model.MTProto,
		Settings: `{"clients":[{"email":"first","secret":"` + mtprotoTestSecretA + `","enable":true}]}`,
	})
	if err != nil {
		t.Fatalf("AddInbound: %v", err)
	}
	t.Cleanup(func() { mtproto.GetManager().Remove(created.Id) })
	waitForSpawns(t, pidFile, 1)

	t.Run("add client rewrites the served config", func(t *testing.T) {
		payload := &model.Inbound{
			Id:       created.Id,
			Settings: `{"clients":[{"email":"second","secret":"` + mtprotoTestSecretB + `","enable":true}]}`,
		}
		needRestart, err := clientSvc.AddInboundClient(inboundSvc, payload)
		if err != nil {
			t.Fatalf("AddInboundClient: %v", err)
		}
		if needRestart {
			t.Fatal("adding an mtproto client must not request an xray restart")
		}
		cfg := readMtgConfig(t, created.Id)
		if !strings.Contains(cfg, `"second" = "`+mtprotoTestSecretB+`"`) {
			t.Fatalf("new client must be in the served config:\n%s", cfg)
		}
		if !strings.Contains(cfg, `"first" = "`+mtprotoTestSecretA+`"`) {
			t.Fatalf("existing client must remain served:\n%s", cfg)
		}
	})

	t.Run("delete client drops it from the served config", func(t *testing.T) {
		if _, err := clientSvc.DelInboundClientByEmail(inboundSvc, created.Id, "second", false, true); err != nil {
			t.Fatalf("DelInboundClientByEmail: %v", err)
		}
		cfg := readMtgConfig(t, created.Id)
		if strings.Contains(cfg, mtprotoTestSecretB) {
			t.Fatalf("deleted client must leave the served config:\n%s", cfg)
		}
		if !strings.Contains(cfg, mtprotoTestSecretA) {
			t.Fatalf("surviving client must stay served:\n%s", cfg)
		}
	})
}
