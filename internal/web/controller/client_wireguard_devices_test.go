package controller

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestWireguardDevicesQueryEmailRoundTrip(t *testing.T) {
	for _, email := range []string{".", "..", "alice@example.com", "legacy/name"} {
		t.Run(email, func(t *testing.T) {
			router, record, inbound := seedLegacyDotClient(t, email)
			privateKey, publicKey, err := wgutil.GenerateWireguardKeypair()
			if err != nil {
				t.Fatal(err)
			}
			client := *record.ToClient()
			client.PrivateKey, client.PublicKey = privateKey, publicKey
			client.AllowedIPs = []string{"10.0.0.2/32"}
			settings, err := json.Marshal(map[string]any{"clients": []model.Client{client}})
			if err != nil {
				t.Fatal(err)
			}
			if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", inbound.Id).Updates(map[string]any{"protocol": model.WireGuard, "settings": string(settings)}).Error; err != nil {
				t.Fatal(err)
			}
			query := "?email=" + url.QueryEscape(email)
			read := func() service.WireguardDeviceList {
				t.Helper()
				response := dotClientRequest(t, router, http.MethodGet, "wireguardDevices"+query, nil)
				assertDotClientSuccess(t, response)
				var message struct {
					Obj service.WireguardDeviceList `json:"obj"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
					t.Fatal(err)
				}
				return message.Obj
			}
			initial := read()
			if len(initial.Inbounds) != 1 || len(initial.Inbounds[0].Devices) != 1 {
				t.Fatal("primary device is missing")
			}
			response := dotClientRequest(t, router, http.MethodPut, "wireguardDevices/limit"+query, map[string]any{"limit": 2})
			assertDotClientSuccess(t, response)
			response = dotClientRequest(t, router, http.MethodPost, "wireguardDevices"+query, map[string]any{"name": "Phone"})
			assertDotClientSuccess(t, response)
			created := read()
			if created.Limit != 2 || len(created.Inbounds[0].Devices) != 2 {
				t.Fatal("device creation did not persist")
			}
			id := created.Inbounds[0].Devices[1].Id
			response = dotClientRequest(t, router, http.MethodPatch, "wireguardDevices/"+id+query, map[string]any{"name": "Laptop", "enable": false})
			assertDotClientSuccess(t, response)
			patched := read().Inbounds[0].Devices[1]
			if patched.Enable || patched.Name != "Laptop" {
				t.Fatal("device patch did not persist")
			}
			response = dotClientRequest(t, router, http.MethodDelete, "wireguardDevices/"+id+query, nil)
			assertDotClientSuccess(t, response)
			if len(read().Inbounds[0].Devices) != 1 {
				t.Fatal("device deletion did not persist")
			}
		})
	}
}
