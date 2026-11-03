package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func mustUnmarshal(t *testing.T, raw string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
}

func settingsClientEmails(t *testing.T, inboundId int) []string {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().First(&ib, inboundId).Error; err != nil {
		t.Fatalf("load inbound %d: %v", inboundId, err)
	}
	clients, err := (&InboundService{}).GetClients(&ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		emails = append(emails, c.Email)
	}
	return emails
}

// Re-adding a client that is already on the inbound must be an idempotent
// no-op, not a second settings entry: checkEmailsExistForClients exempts a
// matching subId (so one identity can span inbounds), which let retried or
// raced adds duplicate the same email inside one settings array (#5770).
func TestAddInboundClient_SkipsClientsAlreadyOnInbound(t *testing.T) {
	setupBulkDB(t)
	nodeID, _ := setupNodeRuntime(t)

	alice := model.Client{ID: uuid.NewString(), Email: "alice@dup", SubID: "alice-sub-1234567", Enable: true}
	ib := nodeInbound(t, nodeID, 33001, []model.Client{alice})

	svc := &ClientService{}
	inboundSvc := &InboundService{}

	if _, err := svc.AddInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Protocol: model.VLESS, Settings: clientsSettings(t, []model.Client{alice})}); err != nil {
		t.Fatalf("re-add of existing client should be a no-op, got error: %v", err)
	}
	if emails := settingsClientEmails(t, ib.Id); len(emails) != 1 || emails[0] != "alice@dup" {
		t.Fatalf("settings after duplicate re-add: expected exactly [alice@dup], got %v", emails)
	}

	bob := model.Client{ID: uuid.NewString(), Email: "bob@dup", SubID: "bob-sub-123456789", Enable: true}
	if _, err := svc.AddInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Protocol: model.VLESS, Settings: clientsSettings(t, []model.Client{alice, bob})}); err != nil {
		t.Fatalf("mixed add (one duplicate, one new): %v", err)
	}
	if emails := settingsClientEmails(t, ib.Id); len(emails) != 2 || emails[0] != "alice@dup" || emails[1] != "bob@dup" {
		t.Fatalf("settings after mixed add: expected [alice@dup bob@dup], got %v", emails)
	}
}

func TestDedupeSettingsClients(t *testing.T) {
	dup := `{"clients": [` +
		`{"id": "u1", "email": "a@x", "subId": "s1"},` +
		`{"id": "u2", "email": "b@x", "subId": "s2"},` +
		`{"id": "u1", "email": "a@x", "subId": "s1"},` +
		`{"id": "u1", "email": "A@X", "subId": "s1"}]}`
	out, changed := dedupeSettingsClients(dup)
	if !changed {
		t.Fatal("expected duplicates to be removed")
	}
	var parsed struct {
		Clients []model.Client `json:"clients"`
	}
	mustUnmarshal(t, out, &parsed)
	if len(parsed.Clients) != 2 || parsed.Clients[0].Email != "a@x" || parsed.Clients[1].Email != "b@x" {
		t.Fatalf("expected first occurrences [a@x b@x], got %+v", parsed.Clients)
	}

	clean := `{"clients": [{"id": "u1", "email": "a@x"}, {"id": "u2", "email": "b@x"}]}`
	if _, changed := dedupeSettingsClients(clean); changed {
		t.Fatal("clean settings must not be rewritten")
	}
	if _, changed := dedupeSettingsClients(""); changed {
		t.Fatal("empty settings must not be rewritten")
	}
	if _, changed := dedupeSettingsClients("{not json"); changed {
		t.Fatal("invalid settings must not be rewritten")
	}
}

// A node's settings blob may nest any value deeply beside its clients; decoding the whole
// blob and re-indenting it cost the square of the depth for every one of these rewrites.
func TestSettingsClientRewritesStayLinearInTheBlob(t *testing.T) {
	deep := strings.Repeat("[", 5000) + strings.Repeat("]", 5000)
	settings := `{"decoy":` + deep + `,"clients":[{"email":"a@x","expiryTime":-86400000,"enable":true},{"email":"A@x"}]}`
	out, changed := dedupeSettingsClients(settings)
	if !changed || len(out) > 2*len(settings) {
		t.Fatalf("dedupe changed=%v, output %d bytes from %d", changed, len(out), len(settings))
	}
	lifted, changed := liftClientLifecycleInSettings(out, map[string]*xray.ClientTraffic{"a@x": {Email: "a@x", ExpiryTime: 1700000000000}})
	if !changed || len(lifted) > 2*len(settings) || !strings.Contains(lifted, `"expiryTime":1700000000000`) || !strings.Contains(lifted, `"enable":false`) {
		t.Fatalf("lift changed=%v, output %d bytes: %.120s", changed, len(lifted), lifted)
	}
	clients, err := ParseInboundSettingsClients(lifted)
	if err != nil || len(clients) != 1 || clients[0].Email != "a@x" {
		t.Fatalf("clients after the rewrites = %+v, %v", clients, err)
	}
}
