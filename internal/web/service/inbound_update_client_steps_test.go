package service

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A master's push carries clients without stamps: the node keeps the ones it
// stored and dates a new client once, the same in settings and in its record.
func TestInboundUpdateFromMasterStampsClientTimestamps(t *testing.T) {
	setupBulkDB(t)
	const storedCreated, storedUpdated = int64(1700000000000), int64(1700000500000)
	known := model.Client{Email: "known@stamp", ID: "aaaaaaaa-0000-0000-0000-000000000301", Enable: true}
	added := model.Client{Email: "added@stamp", ID: "aaaaaaaa-0000-0000-0000-000000000302", Enable: true}
	stored := known
	stored.CreatedAt, stored.UpdatedAt = storedCreated, storedUpdated
	ib := mkInbound(t, 23301, model.VLESS, clientsSettings(t, []model.Client{stored}))

	push := *ib
	push.Settings = clientsSettings(t, []model.Client{known, added})
	before := time.Now().Unix() * 1000
	if _, _, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&push); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	if got, _ := settingsClient(t, ib.Id, known.Email); got.CreatedAt != storedCreated || got.UpdatedAt != storedUpdated {
		t.Fatalf("known client stamps = %d/%d, want the stored %d/%d", got.CreatedAt, got.UpdatedAt, storedCreated, storedUpdated)
	}
	got, _ := settingsClient(t, ib.Id, added.Email)
	if got.CreatedAt < before {
		t.Fatalf("new client created_at = %d, want the save's time (>= %d)", got.CreatedAt, before)
	}
	if got.UpdatedAt != 0 {
		t.Fatalf("new client updated_at = %d, want none: an inbound save is not a client edit", got.UpdatedAt)
	}
	if rec := lookupClientRecord(t, added.Email); rec.CreatedAt != got.CreatedAt {
		t.Fatalf("new client record created at %d, its settings entry at %d", rec.CreatedAt, got.CreatedAt)
	}
}

// The form posts no clients, so the stored list is the one saved; an entry that
// predates the stamps still gets its date.
func TestInboundFormSaveDatesAClientStoredWithoutCreatedAt(t *testing.T) {
	setupBulkDB(t)
	ib := mkInbound(t, 23302, model.VLESS, clientsSettings(t, []model.Client{
		{Email: "legacy@stamp", ID: "aaaaaaaa-0000-0000-0000-000000000303", Enable: true},
	}))

	form := *ib
	form.Settings = `{"decryption":"none"}`
	form.Remark = "edited"
	before := time.Now().Unix() * 1000
	if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	got, ok := settingsClient(t, ib.Id, "legacy@stamp")
	if !ok {
		t.Fatal("the stored client was dropped by a form save that posted none")
	}
	if got.CreatedAt < before {
		t.Fatalf("created_at = %d, want the save's time (>= %d)", got.CreatedAt, before)
	}
}

// Switching between SS-2022 ciphers of different key sizes leaves every stored
// client key the wrong length, and xray-core then rejects those users.
func TestInboundFormSaveRegeneratesKeysAfterAShadowsocksMethodResize(t *testing.T) {
	setupBulkDB(t)
	key16 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	key32 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	ib := mkInbound(t, 23303, model.Shadowsocks,
		`{"method":"2022-blake3-aes-256-gcm","password":"`+key32+`","network":"tcp","clients":[{"email":"psk@ss","password":"`+key32+`","enable":true}]}`)
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, []model.Client{{Email: "psk@ss", Password: key32, Enable: true}}); err != nil {
		t.Fatalf("seed client record: %v", err)
	}

	form := *ib
	form.Settings = `{"method":"2022-blake3-aes-128-gcm","password":"` + key16 + `","network":"tcp"}`
	if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	got, _ := settingsClient(t, ib.Id, "psk@ss")
	if decoded, err := base64.StdEncoding.DecodeString(got.Password); err != nil || len(decoded) != 16 {
		t.Fatalf("client key after the switch to aes-128 = %q, want a 16-byte key (decode err %v)", got.Password, err)
	}
	if rec := lookupClientRecord(t, "psk@ss"); rec.Password != got.Password {
		t.Fatalf("client record still holds key %q, settings hold %q", rec.Password, got.Password)
	}
}

// Vision is stripped from a client while its XHTTP inbound has no vlessenc; the
// save that sets the encryption is the one that has to put it back.
func TestInboundFormSaveRestoresVisionOnceEncryptionIsSet(t *testing.T) {
	db := initFlowTestDB(t)
	ibSvc := &InboundService{}
	const client = `"id":"ce8d33df-3a64-4f10-8f9b-91c3a8e0d101","email":"vision@x","subId":"s1","enable":true`

	sibling := &model.Inbound{
		Tag: "sib", Enable: true, Port: 52201, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{` + client + `,"flow":"` + visionFlow + `"}]}`,
	}
	target := &model.Inbound{
		Tag: "tgt", Enable: true, Port: 52202, Protocol: model.VLESS,
		StreamSettings: `{"network":"xhttp","security":"reality"}`,
		Settings:       `{"clients":[{` + client + `,"flow":""}]}`,
	}
	for _, ib := range []*model.Inbound{sibling, target} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create %s: %v", ib.Tag, err)
		}
		clients, err := ibSvc.GetClients(ib)
		if err != nil {
			t.Fatalf("GetClients %s: %v", ib.Tag, err)
		}
		if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
			t.Fatalf("sync %s: %v", ib.Tag, err)
		}
	}

	form := *target
	form.Settings = `{"decryption":"mlkem768x25519plus.native.0rtt.KEY","encryption":"mlkem768x25519plus.native.0rtt.KEY"}`
	if _, _, err := ibSvc.UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	if got, _ := settingsClient(t, target.Id, "vision@x"); got.Flow != visionFlow {
		t.Fatalf("flow after encryption was set = %q, want %q", got.Flow, visionFlow)
	}
}
