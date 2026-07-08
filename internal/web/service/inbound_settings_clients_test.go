package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestParseInboundSettingsClientsIgnoresProtocolScalarFields(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		want     string
	}{
		{
			name: "vless scalar settings",
			settings: `{
				"clients": [{"email": "alice@example.test", "id": "11111111-1111-1111-1111-111111111111", "limitIp": 2}],
				"decryption": "none",
				"encryption": "none",
				"fallbacks": []
			}`,
			want: "alice@example.test",
		},
		{
			name: "hysteria scalar settings",
			settings: `{
				"clients": [{"email": "bob@example.test", "password": "secret"}],
				"version": 2,
				"ignoreClientBandwidth": false
			}`,
			want: "bob@example.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients, err := ParseInboundSettingsClients(tt.settings)
			if err != nil {
				t.Fatalf("ParseInboundSettingsClients: %v", err)
			}
			if len(clients) != 1 || clients[0].Email != tt.want {
				t.Fatalf("clients = %+v, want one client with email %s", clients, tt.want)
			}
		})
	}
}

func TestParseInboundSettingsClientsRejectsEmptyOrNullSettings(t *testing.T) {
	for _, settings := range []string{"", "   ", "null", " \n null \t "} {
		t.Run(settings, func(t *testing.T) {
			clients, err := ParseInboundSettingsClients(settings)
			if err == nil {
				t.Fatalf("ParseInboundSettingsClients(%q) error = nil, want error", settings)
			}
			if clients != nil {
				t.Fatalf("clients = %+v, want nil", clients)
			}
		})
	}
}

func TestGetClientsIgnoresProtocolScalarFields(t *testing.T) {
	inbound := &model.Inbound{
		Settings: `{
			"clients": [{"email": "alice@example.test", "id": "11111111-1111-1111-1111-111111111111"}],
			"decryption": "none",
			"encryption": "none",
			"fallbacks": []
		}`,
	}

	clients, err := (&InboundService{}).GetClients(inbound)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if len(clients) != 1 || clients[0].Email != "alice@example.test" {
		t.Fatalf("clients = %+v, want alice@example.test", clients)
	}
}

func TestSearchClientTrafficIgnoresProtocolScalarFields(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()

	client := model.Client{
		Email:  "alice@example.test",
		ID:     "11111111-1111-1111-1111-111111111111",
		SubID:  "sub-alice",
		Enable: true,
	}
	inbound := &model.Inbound{
		UserId:   1,
		Tag:      "vless-scalar",
		Enable:   true,
		Port:     43001,
		Protocol: model.VLESS,
		Settings: `{
			"clients": [{"email": "alice@example.test", "id": "11111111-1111-1111-1111-111111111111", "subId": "sub-alice", "enable": true}],
			"decryption": "none",
			"encryption": "none",
			"fallbacks": []
		}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Email: client.Email, Enable: true, SubID: client.SubID}).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: client.Email, Enable: true}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}

	traffic, err := (&InboundService{}).SearchClientTraffic(client.ID)
	if err != nil {
		t.Fatalf("SearchClientTraffic: %v", err)
	}
	if traffic.Email != client.Email || traffic.InboundId != inbound.Id {
		t.Fatalf("traffic = %+v, want email %s inbound %d", traffic, client.Email, inbound.Id)
	}
}
