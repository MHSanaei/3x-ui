package service

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupBulkDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func clientsSettings(t *testing.T, clients []model.Client) string {
	t.Helper()
	b, err := json.Marshal(map[string][]model.Client{"clients": clients})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	return string(b)
}

func emailsOf(clients []model.Client) []string {
	out := make([]string, 0, len(clients))
	for _, c := range clients {
		out = append(out, c.Email)
	}
	return out
}

func sortedEmails(list []model.Client) []string {
	out := emailsOf(list)
	sort.Strings(out)
	return out
}

func mkInbound(t *testing.T, port int, proto model.Protocol, settings string) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		Tag:      string(proto) + "-" + filepath.Base(t.TempDir()),
		Enable:   true,
		Port:     port,
		Protocol: proto,
		Settings: settings,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound %d: %v", port, err)
	}
	return ib
}

// TestBulkAttachDetach_VLESS exercises the batched attach/detach round-trip on
// VLESS inbounds: linkage, settings JSON, idempotency, skip, and record survival.
func TestBulkAttachDetach_VLESS(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "alice@x", ID: "11111111-1111-1111-1111-111111111111", SubID: "sa", Enable: true},
		{Email: "bob@x", ID: "22222222-2222-2222-2222-222222222222", SubID: "sb", Enable: true},
		{Email: "carol@x", ID: "33333333-3333-3333-3333-333333333333", SubID: "sc", Enable: true},
	}

	ib1 := mkInbound(t, 20001, model.VLESS, clientsSettings(t, source))
	ib2 := mkInbound(t, 20002, model.VLESS, `{"clients":[]}`)
	ib3 := mkInbound(t, 20003, model.VLESS, `{"clients":[]}`)

	if err := svc.SyncInbound(nil, ib1.Id, source); err != nil {
		t.Fatalf("seed source linkage: %v", err)
	}

	emails := emailsOf(source)

	res, _, err := svc.BulkAttach(inboundSvc, emails, []int{ib2.Id, ib3.Id})
	if err != nil {
		t.Fatalf("BulkAttach: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("BulkAttach errors: %v", res.Errors)
	}
	if len(res.Skipped) != 0 {
		t.Fatalf("BulkAttach skipped unexpectedly: %v", res.Skipped)
	}
	if len(res.Attached) != 6 {
		t.Fatalf("expected 6 attach entries (3 clients x 2 inbounds), got %d: %v", len(res.Attached), res.Attached)
	}

	for _, ib := range []*model.Inbound{ib2, ib3} {
		list, err := svc.ListForInbound(nil, ib.Id)
		if err != nil {
			t.Fatalf("ListForInbound(%d): %v", ib.Id, err)
		}
		if got := sortedEmails(list); len(got) != 3 {
			t.Fatalf("inbound %d: expected 3 linked clients, got %v", ib.Id, got)
		}
		reloaded, err := inboundSvc.GetInbound(ib.Id)
		if err != nil {
			t.Fatalf("GetInbound(%d): %v", ib.Id, err)
		}
		jsonClients, err := inboundSvc.GetClients(reloaded)
		if err != nil {
			t.Fatalf("GetClients(%d): %v", ib.Id, err)
		}
		if len(jsonClients) != 3 {
			t.Fatalf("inbound %d settings JSON: expected 3 clients, got %d", ib.Id, len(jsonClients))
		}
	}

	res2, _, err := svc.BulkAttach(inboundSvc, emails, []int{ib2.Id, ib3.Id})
	if err != nil {
		t.Fatalf("BulkAttach (idempotent): %v", err)
	}
	if len(res2.Attached) != 0 {
		t.Fatalf("re-attach should add nothing, got Attached=%v", res2.Attached)
	}
	if len(res2.Skipped) != 6 {
		t.Fatalf("re-attach should skip all 6, got Skipped=%v", res2.Skipped)
	}

	dres, _, err := svc.BulkDetach(inboundSvc, emails, []int{ib2.Id, ib3.Id})
	if err != nil {
		t.Fatalf("BulkDetach: %v", err)
	}
	if len(dres.Errors) != 0 {
		t.Fatalf("BulkDetach errors: %v", dres.Errors)
	}
	if len(dres.Detached) != 3 {
		t.Fatalf("expected 3 detached emails, got %v", dres.Detached)
	}

	for _, ib := range []*model.Inbound{ib2, ib3} {
		list, err := svc.ListForInbound(nil, ib.Id)
		if err != nil {
			t.Fatalf("ListForInbound after detach(%d): %v", ib.Id, err)
		}
		if len(list) != 0 {
			t.Fatalf("inbound %d should have no clients after detach, got %v", ib.Id, sortedEmails(list))
		}
		reloaded, _ := inboundSvc.GetInbound(ib.Id)
		jsonClients, _ := inboundSvc.GetClients(reloaded)
		if len(jsonClients) != 0 {
			t.Fatalf("inbound %d settings JSON should be empty after detach, got %d", ib.Id, len(jsonClients))
		}
	}

	for _, e := range emails {
		rec, err := svc.GetRecordByEmail(nil, e)
		if err != nil {
			t.Fatalf("record %q should survive detach: %v", e, err)
		}
		ids, err := svc.GetInboundIdsForRecord(rec.Id)
		if err != nil {
			t.Fatalf("GetInboundIdsForRecord(%q): %v", e, err)
		}
		if len(ids) != 1 || ids[0] != ib1.Id {
			t.Fatalf("record %q should remain attached only to source inbound %d, got %v", e, ib1.Id, ids)
		}
	}
}

// TestBulkDetach_SkipsUnattached verifies emails not on any requested inbound
// land in Skipped, not Detached, and produce no error.
func TestBulkDetach_SkipsUnattached(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "only-on-1@x", ID: "44444444-4444-4444-4444-444444444444", SubID: "s1", Enable: true},
	}
	ib1 := mkInbound(t, 21001, model.VLESS, clientsSettings(t, source))
	ib2 := mkInbound(t, 21002, model.VLESS, `{"clients":[]}`)
	if err := svc.SyncInbound(nil, ib1.Id, source); err != nil {
		t.Fatalf("seed: %v", err)
	}

	dres, restart, err := svc.BulkDetach(inboundSvc, []string{"only-on-1@x"}, []int{ib2.Id})
	if err != nil {
		t.Fatalf("BulkDetach: %v", err)
	}
	if restart {
		t.Fatalf("no-op detach should not require restart")
	}
	if len(dres.Detached) != 0 {
		t.Fatalf("nothing should be detached, got %v", dres.Detached)
	}
	if len(dres.Skipped) != 1 || dres.Skipped[0] != "only-on-1@x" {
		t.Fatalf("expected the email in Skipped, got %v", dres.Skipped)
	}
	if len(dres.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", dres.Errors)
	}
}

// TestBulkAttachDetach_Trojan checks the protocol-specific key matching in the
// batched detach path (Trojan keys on password, not id).
func TestBulkAttachDetach_Trojan(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{
		{Email: "t1@x", Password: "pw-t1", SubID: "t1", Enable: true},
		{Email: "t2@x", Password: "pw-t2", SubID: "t2", Enable: true},
	}
	ib1 := mkInbound(t, 22001, model.Trojan, clientsSettings(t, source))
	ib2 := mkInbound(t, 22002, model.Trojan, `{"clients":[]}`)
	if err := svc.SyncInbound(nil, ib1.Id, source); err != nil {
		t.Fatalf("seed: %v", err)
	}

	emails := emailsOf(source)
	if res, _, err := svc.BulkAttach(inboundSvc, emails, []int{ib2.Id}); err != nil {
		t.Fatalf("BulkAttach: %v", err)
	} else if len(res.Errors) != 0 || len(res.Attached) != 2 {
		t.Fatalf("attach result unexpected: attached=%v errors=%v", res.Attached, res.Errors)
	}

	list, _ := svc.ListForInbound(nil, ib2.Id)
	if len(list) != 2 {
		t.Fatalf("expected 2 trojan clients on ib2, got %v", sortedEmails(list))
	}

	dres, _, err := svc.BulkDetach(inboundSvc, emails, []int{ib2.Id})
	if err != nil {
		t.Fatalf("BulkDetach: %v", err)
	}
	if len(dres.Detached) != 2 || len(dres.Errors) != 0 {
		t.Fatalf("detach result unexpected: detached=%v errors=%v", dres.Detached, dres.Errors)
	}
	if list, _ := svc.ListForInbound(nil, ib2.Id); len(list) != 0 {
		t.Fatalf("trojan clients should be gone from ib2, got %v", sortedEmails(list))
	}
}
