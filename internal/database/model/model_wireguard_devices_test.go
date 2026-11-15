package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWireguardDevicesRoundTripAndClone(t *testing.T) {
	client := Client{Email: "owner", WgDeviceLimit: 3, WireguardDevices: []WireguardDevice{{Id: "device", Name: "Phone", Enable: true, PrivateKey: "private", PublicKey: "public", AllowedIPs: []string{"10.0.0.3/32"}}}}
	record := client.ToRecord()
	if got := record.ToClient(); !reflect.DeepEqual(got.WireguardDevices, client.WireguardDevices) || got.WgDeviceLimit != 3 {
		t.Fatalf("round trip lost devices: %+v", got)
	}
	record.WireguardDevices[0].AllowedIPs[0] = "10.0.0.4/32"
	if client.WireguardDevices[0].AllowedIPs[0] != "10.0.0.3/32" {
		t.Fatal("ToRecord shares device addresses")
	}
	if CloneWireguardDevices(nil) != nil || CloneWireguardDevices([]WireguardDevice{}) == nil {
		t.Fatal("clone lost nil versus explicit empty distinction")
	}
}

func TestWireguardDevicesExpandCorePeers(t *testing.T) {
	client := Client{Email: "owner", Enable: true, PublicKey: "primary", AllowedIPs: []string{"10.0.0.2/32"}, WireguardDevices: []WireguardDevice{
		{Id: "phone", Name: "Phone", Enable: true, PrivateKey: "secret", PublicKey: "phone-public", AllowedIPs: []string{"10.0.0.3/32"}},
		{Id: "laptop", Name: "Laptop", Enable: false, PublicKey: "laptop-public", AllowedIPs: []string{"10.0.0.4/32"}},
	}}
	encoded, err := json.Marshal(map[string]any{"clients": []Client{client}})
	if err != nil {
		t.Fatal(err)
	}
	converted, ok := WireguardClientsToPeers(string(encoded))
	if !ok {
		t.Fatal("settings did not convert")
	}
	var settings struct {
		Peers []map[string]any `json:"peers"`
	}
	if err := json.Unmarshal([]byte(converted), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Peers) != 2 || settings.Peers[1]["publicKey"] != "phone-public" {
		t.Fatalf("unexpected peers: %+v", settings.Peers)
	}
	for _, peer := range settings.Peers {
		if peer["email"] != client.Email || peer["privateKey"] != nil || peer["wireguardDevices"] != nil {
			t.Fatalf("peer leaked credentials or lost parent attribution: %+v", peer)
		}
	}
	if got := WireguardAllowedIPs(client); len(got) != 3 {
		t.Fatalf("disabled device address was not reserved: %v", got)
	}
}

func TestWireguardDeviceMergePreservesOmittedAndClearsExplicitEmpty(t *testing.T) {
	existing := ClientRecord{Email: "owner", UpdatedAt: 10, WgDeviceLimit: 3, WireguardDevices: []WireguardDevice{{Id: "phone"}}}
	MergeClientRecord(&existing, &ClientRecord{Email: "owner", UpdatedAt: 20})
	if len(existing.WireguardDevices) != 1 || existing.WgDeviceLimit != 3 {
		t.Fatal("older snapshot erased devices")
	}
	MergeClientRecord(&existing, &ClientRecord{Email: "owner", UpdatedAt: 30, WgDeviceLimit: 2, WireguardDevices: []WireguardDevice{}})
	if existing.WireguardDevices == nil || len(existing.WireguardDevices) != 0 || existing.WgDeviceLimit != 2 {
		t.Fatal("explicit empty snapshot failed to clear devices")
	}
}
