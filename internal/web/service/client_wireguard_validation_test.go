package service

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func TestValidateWireguardDevicesAllocatesDistinctReservedAddresses(t *testing.T) {
	client := model.Client{Email: "owner", AllowedIPs: []string{"10.0.0.2/32"}, WireguardDevices: []model.WireguardDevice{
		{Name: "Phone", Enable: true}, {Name: "Laptop", Enable: false},
	}}
	if err := validateWireguardDevices("", nil, &client, map[string]string{"10.0.0.3/32": "other inbound"}); err != nil {
		t.Fatal(err)
	}
	if client.WireguardDevices[0].AllowedIPs[0] != "10.0.0.4/32" || client.WireguardDevices[1].AllowedIPs[0] != "10.0.0.5/32" {
		t.Fatalf("unexpected allocation: %+v", client.WireguardDevices)
	}
	if client.WireguardDevices[0].PublicKey == client.WireguardDevices[1].PublicKey {
		t.Fatal("device keys collided")
	}
	before := model.CloneWireguardDevices(client.WireguardDevices)
	if err := validateWireguardDevices("", nil, &client, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, client.WireguardDevices) {
		t.Fatal("validation rotated device identity")
	}
}

func TestWireguardDevicesGenericCreateKeepsIdentityAcrossInbounds(t *testing.T) {
	for _, bulk := range []bool{false, true} {
		t.Run(fmt.Sprintf("bulk=%v", bulk), func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			ids := []int{}
			for i := range 2 {
				settings := fmt.Sprintf(`{"secretKey":%q,"subnetIp":"10.100.%d.0","subnetCidr":24,"clients":[]}`, wgTestSecretKey(), i+1)
				ids = append(ids, mkInbound(t, 51926+i, model.WireGuard, settings).Id)
			}
			payload := ClientCreatePayload{Client: model.Client{Email: "generic@wg", Enable: true, WireguardDevices: []model.WireguardDevice{{Name: "Phone", Enable: true}}}, InboundIds: ids}
			if bulk {
				result, _, err := svc.BulkCreate(&InboundService{}, []ClientCreatePayload{payload})
				if err != nil || result.Created != 1 {
					t.Fatalf("BulkCreate: %+v %v", result, err)
				}
			} else if _, err := svc.Create(&InboundService{}, &payload); err != nil {
				t.Fatal(err)
			}
			devices, err := svc.WireguardDevices("generic@wg")
			if err != nil || len(devices.Inbounds) != 2 {
				t.Fatalf("devices: %+v %v", devices, err)
			}
			first := devices.Inbounds[0].Devices[1]
			second := devices.Inbounds[1].Devices[1]
			if first.Id != second.Id || first.PrivateKey != second.PrivateKey || first.PublicKey != second.PublicKey {
				t.Fatal("generic create generated independent device identities")
			}
			if first.AllowedIPs[0] != "10.100.1.3/32" || second.AllowedIPs[0] != "10.100.2.3/32" {
				t.Fatalf("device allocation lost inbound subnet: %v / %v", first.AllowedIPs, second.AllowedIPs)
			}
		})
	}
}

func TestWireguardDeviceLimitIndependentOfAmneziaPeerOwner(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	awg := mkInbound(t, 51928, model.AmneziaWG, `{"server":{"subnetIp":"10.8.1.0","subnetCidr":24},"clients":[]}`)
	wg := mkInbound(t, 51929, model.WireGuard, wgServerSettings())
	if _, err := svc.Create(&InboundService{}, &ClientCreatePayload{Client: model.Client{Email: "mixed@wg", Enable: true}, InboundIds: []int{awg.Id, wg.Id}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetWireguardDeviceLimit("mixed@wg", 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddWireguardDevice("mixed@wg", "Phone"); err == nil {
		t.Fatal("AmneziaWG peer ownership bypassed WireGuard device limit")
	}
	record := lookupClientRecord(t, "mixed@wg")
	if record.WgDeviceLimit != 1 {
		t.Fatalf("WireGuard limit did not persist: %d", record.WgDeviceLimit)
	}
}

func TestValidateWireguardDevicesRejectsConflictsAndLimit(t *testing.T) {
	privateKey, publicKey, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	base := model.Client{Email: "owner", PublicKey: publicKey, AllowedIPs: []string{"10.0.0.2/32"}}
	for _, tc := range []struct {
		name   string
		limit  int
		device model.WireguardDevice
	}{
		{name: "duplicate key", device: model.WireguardDevice{Name: "Phone", PrivateKey: privateKey}},
		{name: "duplicate address", device: model.WireguardDevice{Name: "Phone", AllowedIPs: []string{"10.0.0.2/32"}}},
		{name: "limit", limit: 1, device: model.WireguardDevice{Name: "Phone"}},
		{name: "invalid limit", limit: 33},
		{name: "invalid id", device: model.WireguardDevice{Id: "primary", Name: "Phone"}},
		{name: "invalid name", device: model.WireguardDevice{Name: "Phone\nLaptop"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := base
			client.WgDeviceLimit = tc.limit
			client.WireguardDevices = []model.WireguardDevice{tc.device}
			if err := validateWireguardDevices("", nil, &client, nil); err == nil {
				t.Fatal("invalid device was accepted")
			}
		})
	}
}

func TestDefaultWireguardClientsReservesDisabledDevice(t *testing.T) {
	existing := []model.Client{{Email: "other", AllowedIPs: []string{"10.0.0.2/32"}, WireguardDevices: []model.WireguardDevice{{Id: uuid.NewString(), Enable: false, AllowedIPs: []string{"10.0.0.3/32"}}}}}
	clients := []model.Client{{Email: "new", Enable: true}}
	if err := defaultWireguardClients("", existing, clients, nil, nil); err != nil {
		t.Fatal(err)
	}
	if clients[0].AllowedIPs[0] != "10.0.0.4/32" {
		t.Fatalf("disabled device address reused: %v", clients[0].AllowedIPs)
	}
}

func TestWireguardMetadataUpdatePreservesDevices(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	ib := mkInbound(t, 51925, model.WireGuard, wgServerSettings())
	client := model.Client{Email: "metadata@wg", Enable: true, WgDeviceLimit: 3, WireguardDevices: []model.WireguardDevice{{Name: "Phone", Enable: true}}}
	if _, err := svc.AddInboundClient(&InboundService{}, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, []model.Client{client})}); err != nil {
		t.Fatal(err)
	}
	before := lookupClientRecord(t, client.Email)
	update := model.Client{Email: client.Email, Enable: true, Comment: "updated"}
	if _, err := svc.UpdateInboundClient(&InboundService{}, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, []model.Client{update})}, client.Email); err != nil {
		t.Fatal(err)
	}
	after := lookupClientRecord(t, client.Email)
	if !reflect.DeepEqual(before.WireguardDevices, after.WireguardDevices) || after.WgDeviceLimit != 3 {
		t.Fatal("metadata edit lost devices")
	}
}
