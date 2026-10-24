package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func newKeepAliveRangeClient(t *testing.T, email, value string) model.Client {
	t.Helper()
	privateKey, publicKey, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate peer keypair: %v", err)
	}
	keepAlive := model.KeepAliveValue(value)
	return model.Client{
		Email: email, Enable: true, PrivateKey: privateKey, PublicKey: publicKey,
		KeepAlive: &keepAlive,
	}
}

func keepAliveTestAWGInbound(t *testing.T, port int) *model.Inbound {
	t.Helper()
	return mkInbound(t, port, model.AmneziaWG, `{"server":{"privateKey":"server","publicKey":"server-pub","subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
}

func TestClientCreateAllowsAmneziaWGKeepAliveRangeAndPersistsNativeValue(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	inbound := keepAliveTestAWGInbound(t, 51871)
	client := newKeepAliveRangeClient(t, "range@awg", "15-25")

	if _, err := clientSvc.CreateOne(inboundSvc, inbound.Id, client); err != nil {
		t.Fatalf("create AmneziaWG client with keepAlive range: %v", err)
	}
	var record model.ClientRecord
	if err := database.GetDB().Where("email = ?", client.Email).First(&record).Error; err != nil {
		t.Fatalf("load saved client: %v", err)
	}
	if record.KeepAlive != "15-25" {
		t.Fatalf("saved keepAlive = %q, want 15-25", record.KeepAlive)
	}
	savedInbound, err := inboundSvc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := inboundSvc.GetClients(savedInbound)
	if err != nil {
		t.Fatalf("load client settings: %v", err)
	}
	if len(clients) != 1 || clients[0].KeepAliveText() != "15-25" {
		t.Fatalf("inbound keepAlive = %#v, want 15-25", clients)
	}
}

func TestClientCreateRejectsKeepAliveRangeForWireGuardBeforeAnyWrite(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	wireguard := mkInbound(t, 51872, model.WireGuard, `{"secretKey":"server","clients":[]}`)
	awg := keepAliveTestAWGInbound(t, 51873)
	client := newKeepAliveRangeClient(t, "range@mixed", "15-25")

	_, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
		Client: client, InboundIds: []int{awg.Id, wireguard.Id},
	})
	if err == nil || !strings.Contains(err.Error(), "keepAlive ranges are supported only for AmneziaWG clients") {
		t.Fatalf("mixed-protocol create error = %v, want WireGuard range rejection", err)
	}
	var records int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", client.Email).Count(&records).Error; err != nil {
		t.Fatal(err)
	}
	if records != 0 {
		t.Fatalf("client records written before protocol validation: %d", records)
	}
	for _, inbound := range []*model.Inbound{awg, wireguard} {
		saved, err := inboundSvc.GetInbound(inbound.Id)
		if err != nil {
			t.Fatal(err)
		}
		var settings struct {
			Clients []model.Client `json:"clients"`
		}
		if err := json.Unmarshal([]byte(saved.Settings), &settings); err != nil {
			t.Fatal(err)
		}
		if len(settings.Clients) != 0 {
			t.Fatalf("client partially written to %s inbound: %#v", inbound.Protocol, settings.Clients)
		}
	}
}

func TestClientUpdateRejectsRangeWhenSharedWithWireGuard(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	awg := keepAliveTestAWGInbound(t, 51874)
	wireguard := mkInbound(t, 51875, model.WireGuard, `{"secretKey":"server","clients":[]}`)
	seed := newKeepAliveRangeClient(t, "shared@tunnels", "25")
	if _, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{
		Client: seed, InboundIds: []int{awg.Id, wireguard.Id},
	}); err != nil {
		t.Fatalf("create scalar client on both protocols: %v", err)
	}
	record, err := clientSvc.GetRecordByEmail(nil, seed.Email)
	if err != nil {
		t.Fatal(err)
	}
	update := model.Client{Email: seed.Email, Enable: true, KeepAlive: seed.KeepAlive}
	*update.KeepAlive = "15-25"
	if _, err := clientSvc.Update(inboundSvc, record.Id, update, 0); err == nil || !strings.Contains(err.Error(), "keepAlive ranges are supported only for AmneziaWG clients") {
		t.Fatalf("shared client update error = %v, want WireGuard range rejection", err)
	}
	var unchanged model.ClientRecord
	if err := database.GetDB().Where("id = ?", record.Id).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.KeepAlive != "25" {
		t.Fatalf("shared record keepAlive after rejected update = %q, want 25", unchanged.KeepAlive)
	}
	for _, inbound := range []*model.Inbound{awg, wireguard} {
		saved, err := inboundSvc.GetInbound(inbound.Id)
		if err != nil {
			t.Fatal(err)
		}
		clients, err := inboundSvc.GetClients(saved)
		if err != nil {
			t.Fatal(err)
		}
		if len(clients) != 1 || clients[0].KeepAliveText() != "25" {
			t.Fatalf("%s inbound keepAlive after rejected update = %#v, want 25", inbound.Protocol, clients)
		}
	}
}

func TestClientCreateRejectsKeepAliveOutsideNativeRangeBeforeWrite(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	inbound := keepAliveTestAWGInbound(t, 51876)
	client := newKeepAliveRangeClient(t, "too-large@awg", "65536")
	if _, err := clientSvc.CreateOne(inboundSvc, inbound.Id, client); err == nil || !strings.Contains(err.Error(), "keepAlive must be an integer from 0 to 65535") {
		t.Fatalf("out-of-range create error = %v, want keepAlive bound error", err)
	}
	var records int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", client.Email).Count(&records).Error; err != nil {
		t.Fatal(err)
	}
	if records != 0 {
		t.Fatalf("out-of-range client was written: %d records", records)
	}
	saved, err := inboundSvc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := inboundSvc.GetClients(saved)
	if err != nil || len(clients) != 0 {
		t.Fatalf("out-of-range client settings = %#v, %v; want no clients", clients, err)
	}
}

func TestClientCreateAllowsRangeForSharedAmneziaWGAndVLESSClient(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	awg := keepAliveTestAWGInbound(t, 51877)
	vless := mkInbound(t, 51878, model.VLESS, `{"clients":[]}`)
	client := newKeepAliveRangeClient(t, "range@awg-vless", "15-25")
	if _, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{Client: client, InboundIds: []int{awg.Id, vless.Id}}); err != nil {
		t.Fatalf("create client shared by AmneziaWG and VLESS: %v", err)
	}
	var record model.ClientRecord
	if err := database.GetDB().Where("email = ?", client.Email).First(&record).Error; err != nil {
		t.Fatalf("read shared client: %v", err)
	}
	if record.KeepAlive != "15-25" {
		t.Fatalf("shared client keepAlive = %q, want 15-25", record.KeepAlive)
	}
}

func TestClientUpdateAllowsRangeForSharedAmneziaWGAndVLESSClient(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	awg := keepAliveTestAWGInbound(t, 51879)
	vless := mkInbound(t, 51880, model.VLESS, `{"clients":[]}`)
	client := newKeepAliveRangeClient(t, "update@awg-vless", "25")
	if _, err := clientSvc.Create(inboundSvc, &ClientCreatePayload{Client: client, InboundIds: []int{awg.Id, vless.Id}}); err != nil {
		t.Fatalf("create scalar client shared by AmneziaWG and VLESS: %v", err)
	}
	record, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatal(err)
	}
	updated := model.KeepAliveValue("15-25")
	client.KeepAlive = &updated
	if _, err := clientSvc.Update(inboundSvc, record.Id, client, 0); err != nil {
		t.Fatalf("update shared client to an AmneziaWG range: %v", err)
	}
	var saved model.ClientRecord
	if err := database.GetDB().Where("id = ?", record.Id).First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.KeepAlive != "15-25" {
		t.Fatalf("updated shared client keepAlive = %q, want 15-25", saved.KeepAlive)
	}
	client.KeepAlive = model.KeepAlivePtr(25)
	if _, err := clientSvc.Update(inboundSvc, record.Id, client, 0); err != nil {
		t.Fatalf("change shared client range to scalar: %v", err)
	}
	client.KeepAlive = model.KeepAlivePtr(0)
	if _, err := clientSvc.Update(inboundSvc, record.Id, client, 0); err != nil {
		t.Fatalf("set explicit zero after range: %v", err)
	}
	client.KeepAlive = nil
	if _, err := clientSvc.Update(inboundSvc, record.Id, client, 0); err != nil {
		t.Fatalf("update with omitted keepAlive after explicit zero: %v", err)
	}
	if err := database.GetDB().Where("id = ?", record.Id).First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.KeepAlive != "0" {
		t.Fatalf("omitted update changed explicit zero to %q", saved.KeepAlive)
	}
}

func TestSetClientEnableByEmailPreservesLegacyAmneziaWGKeepAlive(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	legacy := newKeepAliveRangeClient(t, "legacy-enable@awg", "25")
	inbound := keepAliveTestAWGInbound(t, 51881)
	if _, err := clientSvc.CreateOne(inboundSvc, inbound.Id, legacy); err != nil {
		t.Fatalf("create client: %v", err)
	}
	legacy.KeepAlive = model.KeepAlivePtr(70000)
	if err := saveKeepAliveTestClientSettings(t, inboundSvc, inbound.Id, legacy); err != nil {
		t.Fatalf("seed legacy client settings: %v", err)
	}

	changed, _, err := clientSvc.SetClientEnableByEmail(inboundSvc, legacy.Email, false)
	if err != nil {
		t.Fatalf("disable existing client with legacy keepAlive: %v", err)
	}
	if !changed {
		t.Fatal("SetClientEnableByEmail reported no change")
	}
	saved, err := inboundSvc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := inboundSvc.GetClients(saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 1 || clients[0].Enable || clients[0].KeepAliveText() != "70000" {
		t.Fatalf("saved legacy client = %#v, want disabled with keepAlive 70000", clients)
	}
}

func TestUpdateInboundPreservesUnchangedLegacyAmneziaWGKeepAlive(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	legacy := newKeepAliveRangeClient(t, "legacy-remark@awg", "-5")
	legacy.AllowedIPs = []string{"10.8.1.3/32"}
	inbound := mkInbound(t, 51882, model.AmneziaWG, clientsSettings(t, []model.Client{legacy}))
	loaded, err := inboundSvc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Remark = "updated remark"
	if _, _, err := inboundSvc.UpdateInbound(loaded); err != nil {
		t.Fatalf("save unrelated inbound remark with legacy keepAlive: %v", err)
	}
	saved, err := inboundSvc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := inboundSvc.GetClients(saved)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Remark != "updated remark" {
		t.Fatalf("saved inbound remark = %q, want updated remark", saved.Remark)
	}
	if len(clients) != 1 || clients[0].KeepAliveText() != "-5" {
		t.Fatalf("saved client keepAlive = %#v, want -5", clients)
	}
}

func saveKeepAliveTestClientSettings(t *testing.T, inboundSvc *InboundService, inboundID int, client model.Client) error {
	t.Helper()
	inbound, err := inboundSvc.GetInbound(inboundID)
	if err != nil {
		return err
	}
	clients, err := inboundSvc.GetClients(inbound)
	if err != nil {
		return err
	}
	for i := range clients {
		if clients[i].Email == client.Email {
			clients[i].KeepAlive = client.KeepAlive
			clients[i].AllowedIPs = client.AllowedIPs
			clients[i].PrivateKey = client.PrivateKey
			clients[i].PublicKey = client.PublicKey
		}
	}
	return database.GetDB().Model(&model.Inbound{}).Where("id = ?", inboundID).Update("settings", clientsSettings(t, clients)).Error
}

func TestClientUpdateGrandfathersOnlyUnchangedLegacyKeepAlive(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	clientSvc := &ClientService{}
	client := newKeepAliveRangeClient(t, "legacy-update@awg", "25")
	inbound := keepAliveTestAWGInbound(t, 51883)
	if _, err := clientSvc.CreateOne(inboundSvc, inbound.Id, client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.KeepAlive = model.KeepAlivePtr(70000)
	if err := saveKeepAliveTestClientSettings(t, inboundSvc, inbound.Id, client); err != nil {
		t.Fatalf("seed legacy client settings: %v", err)
	}
	fresh, err := inboundSvc.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := inboundSvc.GetClients(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if err := clientSvc.SyncInbound(nil, inbound.Id, clients); err != nil {
		t.Fatalf("sync legacy record: %v", err)
	}
	record, err := clientSvc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatal(err)
	}
	unchanged := model.Client{Email: client.Email, Enable: true, KeepAlive: model.KeepAlivePtr(70000), Comment: "updated comment"}
	if _, err := clientSvc.Update(inboundSvc, record.Id, unchanged, 0); err != nil {
		t.Fatalf("update unrelated field with unchanged legacy keepAlive: %v", err)
	}
	renamed := unchanged
	renamed.Email = "legacy-renamed@awg"
	if _, err := clientSvc.Update(inboundSvc, record.Id, renamed, 0); err != nil {
		t.Fatalf("rename client with unchanged legacy keepAlive: %v", err)
	}
	record, err = clientSvc.GetByID(record.Id)
	if err != nil {
		t.Fatal(err)
	}
	if record.Email != "legacy-renamed@awg" || record.Comment != "updated comment" || record.KeepAlive != "70000" {
		t.Fatalf("saved renamed record = %#v, want renamed/comment updated with keepAlive 70000", record)
	}
	changed := renamed
	changed.KeepAlive = model.KeepAlivePtr(70001)
	if _, err := clientSvc.Update(inboundSvc, record.Id, changed, 0); err == nil || !strings.Contains(err.Error(), "keepAlive must be an integer from 0 to 65535") {
		t.Fatalf("changed invalid legacy value error = %v, want keepAlive bound error", err)
	}
	stored, err := clientSvc.GetByID(record.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.KeepAlive != "70000" {
		t.Fatalf("record after rejected changed legacy value = %q, want 70000", stored.KeepAlive)
	}
}
