package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// EffectiveFlowsByEmails resolves intended flow for many clients in one batched
// query, taking the flow_override of the lowest inbound_id and skipping emails
// with no non-empty flow anywhere.
func TestEffectiveFlowsByEmails(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()

	const vision = "xtls-rprx-vision"

	// vis@x: attached to inbound 20 (empty flow) and 10 (Vision) — lowest
	// inbound_id (10) wins, so the empty override on 20 must not mask it.
	// plain@x: only an empty flow_override anywhere — absent from the result.
	mkClient := func(id int, email string) {
		if err := db.Create(&model.ClientRecord{Id: id, Email: email, Enable: true}).Error; err != nil {
			t.Fatalf("create client %s: %v", email, err)
		}
	}
	mkLink := func(clientID, inboundID int, flow string) {
		if err := db.Create(&model.ClientInbound{ClientId: clientID, InboundId: inboundID, FlowOverride: flow}).Error; err != nil {
			t.Fatalf("link %d/%d: %v", clientID, inboundID, err)
		}
	}
	mkClient(1, "vis@x")
	mkClient(2, "plain@x")
	mkLink(1, 20, "")     // higher inbound_id, empty
	mkLink(1, 10, vision) // lower inbound_id, Vision
	mkLink(2, 30, "")     // only empty override

	cs := &ClientService{}
	got, err := cs.EffectiveFlowsByEmails(nil, []string{"vis@x", "plain@x", "missing@x"})
	if err != nil {
		t.Fatalf("EffectiveFlowsByEmails: %v", err)
	}

	if got["vis@x"] != vision {
		t.Errorf("vis@x = %q, want %q (lowest inbound_id flow_override)", got["vis@x"], vision)
	}
	if v, ok := got["plain@x"]; ok {
		t.Errorf("plain@x present (%q); want absent (no non-empty flow anywhere)", v)
	}
	if v, ok := got["missing@x"]; ok {
		t.Errorf("missing@x present (%q); want absent (unknown client)", v)
	}

	// Empty input is a no-op (no query).
	if m, err := cs.EffectiveFlowsByEmails(nil, nil); err != nil || len(m) != 0 {
		t.Errorf("empty input: got %v err %v, want empty map", m, err)
	}
}
