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
