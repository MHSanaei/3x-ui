package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestWireguardReEnableRetainsAllDeviceCredentials(t *testing.T) {
	for _, bulk := range []bool{true, false} {
		t.Run(map[bool]string{true: "bulk", false: "traffic-reset"}[bulk], func(t *testing.T) {
			setupConflictDB(t)
			manager := useTestRuntimeManager(t)
			probe := new(reverseUserProbe)
			manager.SetLocalRuntimeOverride(probe)
			client := model.Client{
				Email: "wireguard-devices", Enable: !bulk, PublicKey: "primary", AllowedIPs: []string{"10.0.0.2/32"},
				PreSharedKey: "psk", KeepAlive: model.KeepAlivePtr(25),
				WireguardDevices: []model.WireguardDevice{{Id: "phone", Enable: true, PublicKey: "phone-key", AllowedIPs: []string{"10.0.0.3/32"}}},
			}
			seedInboundConflict(t, "wg-readd", "0.0.0.0", 50079, model.WireGuard, "", clientsSettings(t, []model.Client{client}))
			inbound := loadInboundByTag(t, "wg-readd")
			if err := (&ClientService{}).SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
				t.Fatal(err)
			}
			if bulk {
				if _, _, err := (&ClientService{}).BulkSetEnable(&InboundService{}, []string{client.Email}, true); err != nil {
					t.Fatal(err)
				}
			} else {
				depleteClientTraffic(t, inbound.Id, client.Email)
				if _, err := (&InboundService{}).ResetClientTraffic(inbound.Id, client.Email); err != nil {
					t.Fatal(err)
				}
			}
			users := probe.recorded()
			if len(users) != 1 {
				t.Fatalf("runtime received %d users, want one parent with devices", len(users))
			}
			user := users[0]
			devices, ok := user["wireguardDevices"].([]model.WireguardDevice)
			if !ok || len(devices) != 1 || devices[0].PublicKey != "phone-key" {
				t.Fatalf("lost device credentials: %v", user)
			}
			if user["publicKey"] != "primary" || user["preSharedKey"] != "psk" || user["keepAlive"] != "25" {
				t.Fatalf("lost primary credentials: %v", user)
			}
		})
	}
}
