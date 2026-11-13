package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const partialTestPSK = "Pm4Hn0f8i8b6v3gJ0vZQ3yHq4mUe7qkq0m3o2l9sY9c="

func updateByEmailJSON(t *testing.T, svc *ClientService, email, body string) {
	t.Helper()
	if _, err := svc.UpdateByEmailJSON(&InboundService{}, email, []byte(body)); err != nil {
		t.Fatalf("UpdateByEmailJSON(%s): %v", body, err)
	}
}

func inboundClientByEmail(t *testing.T, inboundId int, email string) model.Client {
	t.Helper()
	inboundSvc := &InboundService{}
	ib, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		t.Fatalf("GetInbound(%d): %v", inboundId, err)
	}
	clients, err := inboundSvc.GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients(%d): %v", inboundId, err)
	}
	for _, c := range clients {
		if c.Email == email {
			return c
		}
	}
	t.Fatalf("inbound %d has no client %q", inboundId, email)
	return model.Client{}
}

// A body carrying only email and expiryTime used to disable the client and zero its
// limits, Telegram id, comment, group, renewal settings and limitHwid.
func TestUpdateByEmailJSONKeepsOmittedFields(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	ib := mkInbound(t, 52111, model.AmneziaWG, `{"server":{"privateKey":"server","publicKey":"server-pub","subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)

	seed := model.Client{
		Email: "full@awg", Enable: true,
		PrivateKey: "client-private", PublicKey: "client-public",
		PreSharedKey: partialTestPSK,
		AllowedIPs:   []string{"10.8.1.8/32"},
		LimitIP:      3, TotalGB: 50 << 30, ExpiryTime: 1700000000000,
		TgID: 123456, Comment: "laptop", Group: "family",
		Reset: 30, ResetMax: 4,
	}
	seed.AllowedIPsByInbound = map[int][]string{ib.Id: seed.AllowedIPs}
	if _, err := svc.CreateOne(&InboundService{}, ib.Id, seed); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", seed.Email).
		UpdateColumn("limit_hwid", 2).Error; err != nil {
		t.Fatal(err)
	}

	updateByEmailJSON(t, svc, seed.Email, `{"email": "full@awg", "expiryTime": 1893456000000}`)

	rec := lookupClientRecord(t, seed.Email)
	if !rec.Enable {
		t.Fatal("partial update disabled the client")
	}
	if rec.ExpiryTime != 1893456000000 {
		t.Fatalf("expiry not applied: %d", rec.ExpiryTime)
	}
	if rec.LimitIP != 3 || rec.TotalGB != 50<<30 || rec.TgID != 123456 || rec.Comment != "laptop" ||
		rec.Group != "family" || rec.Reset != 30 || rec.ResetMax != 4 || rec.LimitHwid != 2 {
		t.Fatalf("partial update reset omitted fields: %+v", rec)
	}
	peer := inboundClientByEmail(t, ib.Id, seed.Email)
	if !peer.Enable || peer.PreSharedKey != partialTestPSK || peer.PrivateKey != seed.PrivateKey {
		t.Fatalf("inbound peer lost its state: %+v", peer)
	}

	updateByEmailJSON(t, svc, seed.Email, `{"enable": false, "comment": "", "totalGB": 0}`)
	rec = lookupClientRecord(t, seed.Email)
	if rec.Enable || rec.Comment != "" || rec.TotalGB != 0 {
		t.Fatalf("explicit zeros were not applied: %+v", rec)
	}
	if rec.ExpiryTime != 1893456000000 || rec.LimitIP != 3 || rec.Group != "family" || rec.LimitHwid != 2 {
		t.Fatalf("second partial update reset omitted fields: %+v", rec)
	}
}

// encoding/json binds "limitIP" to limitIp; a case-sensitive presence check then
// overwrote the decoded zero with the stored value.
func TestUpdateByEmailJSONMatchesKeysCaseInsensitively(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	ib := mkInbound(t, 52112, model.VLESS, `{"clients":[]}`)
	seed := model.Client{
		Email: "case@vless", ID: "0d1b6f8e-3c2a-4f61-9a43-5b7f0c2e9d11", Enable: true,
		LimitIP: 3, TgID: 123456, ExpiryTime: 1700000000000,
	}
	if _, err := svc.CreateOne(&InboundService{}, ib.Id, seed); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	updateByEmailJSON(t, svc, seed.Email, `{"limitIP": 0, "tgID": 0, "expirytime": 1893456000000}`)

	rec := lookupClientRecord(t, seed.Email)
	if rec.LimitIP != 0 || rec.TgID != 0 || rec.ExpiryTime != 1893456000000 {
		t.Fatalf("differently-cased keys were not applied: limitIp=%d tgId=%d expiry=%d", rec.LimitIP, rec.TgID, rec.ExpiryTime)
	}
	if !rec.Enable {
		t.Fatal("partial update disabled the client")
	}
}

// clients.flow is unreliable for a multi-inbound client (#4792): an omitted flow
// must keep each inbound's own flow instead of copying that column everywhere.
func TestUpdateByEmailJSONOmittedFlowKeepsEachInboundsFlow(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	const vision = "xtls-rprx-vision"
	reality := mkInboundStream(t, 52113, model.VLESS, `{"clients":[]}`, realityStream)
	ws := mkInboundStream(t, 52114, model.VLESS, `{"clients":[]}`, wsStream)
	seed := model.Client{
		Email: "flow@vless", ID: "6a0f2c1e-8b4d-4e7a-9c35-1d2e3f4a5b6c", Enable: true, Flow: vision,
		Reverse: &model.ClientReverse{Tag: "back"},
	}
	if _, err := svc.Create(&InboundService{}, &ClientCreatePayload{Client: seed, InboundIds: []int{reality.Id, ws.Id}}); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if rec := lookupClientRecord(t, seed.Email); rec.Reverse == "" {
		t.Fatal("seed did not store the reverse tag")
	}
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", seed.Email).
		UpdateColumn("flow", "").Error; err != nil {
		t.Fatalf("zero clients.flow: %v", err)
	}

	updateByEmailJSON(t, svc, seed.Email, `{"expiryTime": 1893456000000}`)

	if got := inboundClientByEmail(t, reality.Id, seed.Email); got.Flow != vision || got.ExpiryTime != 1893456000000 {
		t.Fatalf("Reality inbound: flow=%q expiry=%d, want %q and the new expiry", got.Flow, got.ExpiryTime, vision)
	}
	if got := inboundClientByEmail(t, ws.Id, seed.Email); got.Flow != "" {
		t.Fatalf("WS inbound gained flow %q", got.Flow)
	}
	if rec := lookupClientRecord(t, seed.Email); rec.Reverse == "" {
		t.Fatal("an omitted reverse must keep the tag")
	}
	updateByEmailJSON(t, svc, seed.Email, `{"reverse": null}`)
	if rec := lookupClientRecord(t, seed.Email); rec.Reverse != "" {
		t.Fatalf("an explicit null reverse must clear the tag: %q", rec.Reverse)
	}

	updateByEmailJSON(t, svc, seed.Email, `{"flow": ""}`)
	if got := inboundClientByEmail(t, reality.Id, seed.Email); got.Flow != "" {
		t.Fatalf("an explicit empty flow must clear Vision, got %q", got.Flow)
	}
}
