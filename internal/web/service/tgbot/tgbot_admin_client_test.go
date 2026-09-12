package tgbot

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedInboundClient(t *testing.T, port int, clients []model.Client) *Tgbot {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	raw, err := json.MarshalIndent(map[string][]model.Client{"clients": clients}, "", "  ")
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound := &model.Inbound{
		Tag:      "vless-admin-client-test",
		Enable:   true,
		Port:     port,
		Protocol: model.VLESS,
		Settings: string(raw),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	tg := &Tgbot{}
	if err := tg.clientService.SyncInbound(nil, inbound.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	return tg
}

func mustRecord(t *testing.T, tg *Tgbot, email string) *model.ClientRecord {
	t.Helper()
	record, err := tg.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail(%q): %v", email, err)
	}
	return record
}

// A stale or mistyped state must not be able to steer an edit at the wrong
// client, so the prefix and a non-empty target are both required.
func TestStateTarget(t *testing.T) {
	tests := []struct {
		name   string
		state  string
		prefix string
		want   string
		wantOK bool
	}{
		{"email edit", stateEditEmailPrefix + "alice@x", stateEditEmailPrefix, "alice@x", true},
		{"comment edit", stateEditCommentPrefix + "bob@x", stateEditCommentPrefix, "bob@x", true},
		{"trims whitespace", stateEditEmailPrefix + "  carol@x  ", stateEditEmailPrefix, "carol@x", true},
		{"wrong prefix", stateEditCommentPrefix + "alice@x", stateEditEmailPrefix, "", false},
		{"no target", stateEditEmailPrefix, stateEditEmailPrefix, "", false},
		{"blank target", stateEditEmailPrefix + "   ", stateEditEmailPrefix, "", false},
		{"unrelated state", statePmText, stateEditEmailPrefix, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := stateTarget(tc.state, tc.prefix)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("stateTarget(%q, %q) = (%q, %v), want (%q, %v)", tc.state, tc.prefix, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestNormalizeFieldValue(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"paid until March", "paid until March"},
		{fieldClearToken, ""},
		{"", ""},
		{"--", "--"},
	}
	for _, tc := range tests {
		if got := normalizeFieldValue(tc.in); got != tc.want {
			t.Fatalf("normalizeFieldValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Renaming must not silently rotate the client's credentials: the customer's
// installed config keeps working and only the panel-side label changes.
func TestEditClientRecordRenameKeepsCredentials(t *testing.T) {
	const uuid = "aaaaaaaa-0000-0000-0000-0000000000a1"
	tg := seedInboundClient(t, 31001, []model.Client{
		{Email: "old@x", ID: uuid, SubID: "sub-rename", Enable: true, Comment: "keep me"},
	})
	originalID := mustRecord(t, tg, "old@x").Id

	if err := tg.editClientRecord("old@x", func(c *model.Client) { c.Email = "new@x" }); err != nil {
		t.Fatalf("editClientRecord: %v", err)
	}

	if _, err := tg.clientService.GetRecordByEmail(nil, "old@x"); err == nil {
		t.Fatal("old email still resolves to a record")
	}
	record := mustRecord(t, tg, "new@x")
	if record.Id != originalID {
		t.Fatalf("record id = %d, want %d (rename must not create a second record)", record.Id, originalID)
	}
	if record.UUID != uuid {
		t.Fatalf("uuid = %q, want %q", record.UUID, uuid)
	}
	if record.SubID != "sub-rename" {
		t.Fatalf("subId = %q, want %q", record.SubID, "sub-rename")
	}
	if record.Comment != "keep me" {
		t.Fatalf("comment = %q, want %q", record.Comment, "keep me")
	}
}

// A leaked subscription URL is revoked by issuing a new subId, and that must be
// the only thing that changes — the installed config must keep working.
func TestEditClientRecordNewSubIDKeepsUUID(t *testing.T) {
	const uuid = "aaaaaaaa-0000-0000-0000-0000000000a2"
	tg := seedInboundClient(t, 31002, []model.Client{
		{Email: "leak@x", ID: uuid, SubID: "sub-leaked", Enable: true},
	})

	if err := tg.editClientRecord("leak@x", func(c *model.Client) { c.SubID = "sub-fresh" }); err != nil {
		t.Fatalf("editClientRecord: %v", err)
	}

	record := mustRecord(t, tg, "leak@x")
	if record.SubID != "sub-fresh" {
		t.Fatalf("subId = %q, want %q", record.SubID, "sub-fresh")
	}
	if record.UUID != uuid {
		t.Fatalf("uuid = %q, want %q", record.UUID, uuid)
	}
}

func TestEditClientRecordClearsComment(t *testing.T) {
	tg := seedInboundClient(t, 31003, []model.Client{
		{Email: "note@x", ID: "aaaaaaaa-0000-0000-0000-0000000000a3", SubID: "sub-note", Enable: true, Comment: "expires soon"},
	})

	if err := tg.editClientRecord("note@x", func(c *model.Client) { c.Comment = normalizeFieldValue(fieldClearToken) }); err != nil {
		t.Fatalf("editClientRecord: %v", err)
	}

	if comment := mustRecord(t, tg, "note@x").Comment; comment != "" {
		t.Fatalf("comment = %q, want empty", comment)
	}
}

func TestEditClientRecordRejectsDuplicateEmail(t *testing.T) {
	tg := seedInboundClient(t, 31004, []model.Client{
		{Email: "first@x", ID: "aaaaaaaa-0000-0000-0000-0000000000a4", SubID: "sub-first", Enable: true},
		{Email: "second@x", ID: "aaaaaaaa-0000-0000-0000-0000000000a5", SubID: "sub-second", Enable: true},
	})

	if err := tg.editClientRecord("first@x", func(c *model.Client) { c.Email = "second@x" }); err == nil {
		t.Fatal("rename onto an existing email succeeded, want an error")
	}
	if email := mustRecord(t, tg, "first@x").Email; email != "first@x" {
		t.Fatalf("email after failed rename = %q, want %q", email, "first@x")
	}
}

// A disabled inbound carries no traffic, so offering it to a customer points at a
// dead route; an admin still sees it, because attached-but-off explains a ticket.
func TestDescribeAttachedInboundsHidesDisabledFromCustomers(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	live := &model.Inbound{Tag: "live", Remark: "live-inbound", Enable: true, Port: 32001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	off := &model.Inbound{Tag: "off", Remark: "off-inbound", Enable: false, Port: 32002, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	for _, ib := range []*model.Inbound{live, off} {
		if err := database.GetDB().Create(ib).Error; err != nil {
			t.Fatalf("create inbound: %v", err)
		}
	}

	tg := &Tgbot{}
	ids := []int{live.Id, off.Id}

	customer := tg.describeAttachedInbounds(ids, true)
	if strings.Contains(customer, "off-inbound") {
		t.Fatalf("customer view %q must not list a disabled inbound", customer)
	}
	if !strings.Contains(customer, "live-inbound") {
		t.Fatalf("customer view %q must still list the working inbound", customer)
	}

	admin := tg.describeAttachedInbounds(ids, false)
	if !strings.Contains(admin, "off-inbound") || !strings.Contains(admin, "❌") {
		t.Fatalf("admin view %q must list the disabled inbound and mark it", admin)
	}
}

// A client attached only to disabled inbounds must not render a bare header
// with an empty list after its entries are filtered out.
func TestDescribeAttachedInboundsAllDisabled(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	off := &model.Inbound{Tag: "off-only", Remark: "off-only", Enable: false, Port: 32003, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(off).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	tg := &Tgbot{}
	if got := tg.describeAttachedInbounds([]int{off.Id}, true); got != "" {
		t.Fatalf("customer view = %q, want empty so the caller omits the line", got)
	}
}
