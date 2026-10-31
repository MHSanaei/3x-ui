package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const stillTestPSK = "Pm4Hn0f8i8b6v3gJ0vZQ3yHq4mUe7qkq0m3o2l9sY9c="

func stillAWGInbound(t *testing.T, port int) *model.Inbound {
	t.Helper()
	return mkInbound(t, port, model.AmneziaWG, `{"server":{"privateKey":"server","publicKey":"server-pub","subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
}

// A metadata-only edit of an AmneziaWG client (the bot/API case: renew the
// expiry, nothing else) must not wipe or rotate the client's preSharedKey, nor
// its key pair or tunnel address -- in the inbound settings JSON (what the
// running peer is built from) and in the client record alike.
func TestStillAWGPartialUpdateKeepsPreSharedKey(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}
	ib := stillAWGInbound(t, 52110)

	seed := model.Client{
		Email: "psk@awg", Enable: true,
		PrivateKey: "client-private", PublicKey: "client-public",
		PreSharedKey: stillTestPSK,
		AllowedIPs:   []string{"10.8.1.7/32"},
	}
	seed.AllowedIPsByInbound = map[int][]string{ib.Id: seed.AllowedIPs}
	if _, err := svc.CreateOne(inboundSvc, ib.Id, seed); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	rec := lookupClientRecord(t, seed.Email)
	if rec.PreSharedKey != stillTestPSK {
		t.Fatalf("seed did not store the preSharedKey: %q", rec.PreSharedKey)
	}

	// Exactly what a partial API body turns into: no key fields at all.
	partial := model.Client{Email: seed.Email, Enable: true, ExpiryTime: 1893456000000}
	if _, err := svc.Update(inboundSvc, rec.Id, partial, 0); err != nil {
		t.Fatalf("partial update: %v", err)
	}

	after, err := inboundSvc.GetInbound(ib.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients := mustAWGClients(t, after)
	if len(clients) != 1 {
		t.Fatalf("expected one client, got %d", len(clients))
	}
	got := clients[0]
	if got.PreSharedKey != stillTestPSK {
		t.Fatalf("inbound settings lost the preSharedKey: %q", got.PreSharedKey)
	}
	if got.PrivateKey != seed.PrivateKey || got.PublicKey != seed.PublicKey {
		t.Fatalf("inbound settings rotated the key pair: %+v", got)
	}
	if len(got.AllowedIPs) != 1 || got.AllowedIPs[0] != "10.8.1.7/32" {
		t.Fatalf("inbound settings changed allowedIPs: %v", got.AllowedIPs)
	}
	if got.ExpiryTime != partial.ExpiryTime {
		t.Fatalf("the requested change was not applied: expiry %d", got.ExpiryTime)
	}

	stored := lookupClientRecord(t, seed.Email)
	if stored.PreSharedKey != stillTestPSK || stored.PrivateKey != seed.PrivateKey || stored.PublicKey != seed.PublicKey {
		t.Fatalf("client record lost tunnel credentials: %+v", stored)
	}
}

// The same partial body through the API path (FillAbsentClientFields before
// Update) must keep every generic field it did not mention, while explicit
// zeros still clear.
func TestStillPartialAPIUpdateKeepsUnsentFields(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}
	ib := stillAWGInbound(t, 52111)

	seed := model.Client{
		Email: "full@awg", Enable: true,
		PrivateKey: "client-private", PublicKey: "client-public",
		PreSharedKey: stillTestPSK,
		AllowedIPs:   []string{"10.8.1.8/32"},
		LimitIP:      3, TotalGB: 50 << 30, ExpiryTime: 1700000000000,
		TgID: 123456, Comment: "laptop", Group: "family",
		Reset: 30, ResetMax: 4,
	}
	seed.AllowedIPsByInbound = map[int][]string{ib.Id: seed.AllowedIPs}
	if _, err := svc.CreateOne(inboundSvc, ib.Id, seed); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", seed.Email).
		UpdateColumn("limit_hwid", 2).Error; err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"expiryTime": 1893456000000}`)
	var req model.Client
	req.ExpiryTime = 1893456000000
	limitHwid := 0
	if err := svc.FillAbsentClientFields(seed.Email, &req, &limitHwid, ClientPayloadKeys(body)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateByEmail(inboundSvc, seed.Email, req, limitHwid); err != nil {
		t.Fatalf("partial API update: %v", err)
	}

	rec := lookupClientRecord(t, seed.Email)
	if !rec.Enable {
		t.Fatal("partial update disabled the client")
	}
	if rec.ExpiryTime != 1893456000000 {
		t.Fatalf("expiry not applied: %d", rec.ExpiryTime)
	}
	if rec.LimitIP != 3 || rec.TotalGB != 50<<30 || rec.TgID != 123456 || rec.Comment != "laptop" ||
		rec.Group != "family" || rec.Reset != 30 || rec.ResetMax != 4 || rec.LimitHwid != 2 {
		t.Fatalf("partial update reset unsent fields: %+v", rec)
	}
	if rec.PreSharedKey != stillTestPSK {
		t.Fatalf("partial update lost the preSharedKey: %q", rec.PreSharedKey)
	}

	// Explicit values, zeros included, still win.
	body = []byte(`{"enable": false, "comment": "", "totalGB": 0}`)
	req = model.Client{}
	limitHwid = 0
	if err := svc.FillAbsentClientFields(seed.Email, &req, &limitHwid, ClientPayloadKeys(body)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateByEmail(inboundSvc, seed.Email, req, limitHwid); err != nil {
		t.Fatalf("explicit update: %v", err)
	}
	rec = lookupClientRecord(t, seed.Email)
	if rec.Enable || rec.Comment != "" || rec.TotalGB != 0 {
		t.Fatalf("explicit zeros were not applied: %+v", rec)
	}
	if rec.ExpiryTime != 1893456000000 || rec.LimitIP != 3 || rec.Group != "family" || rec.LimitHwid != 2 {
		t.Fatalf("second partial update reset unsent fields: %+v", rec)
	}
}

func TestStillClientPayloadKeys(t *testing.T) {
	keys := ClientPayloadKeys([]byte(`{"email":"a","enable":false}`))
	if !keys["email"] || !keys["enable"] || keys["comment"] {
		t.Fatalf("unexpected key set: %v", keys)
	}
	if len(ClientPayloadKeys([]byte(`not json`))) != 0 {
		t.Fatal("non-object body must yield an empty key set")
	}
}
