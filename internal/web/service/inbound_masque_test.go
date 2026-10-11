package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const masqueTestStream = `{"network":"masque","security":"tls","masqueSettings":{"path":"/.well-known/masque/ip/*/*/"},
	"tlsSettings":{"alpn":["h3"],"certificates":[{"certificateFile":"/etc/ssl/certs/m.crt","keyFile":"/etc/ssl/private/m.key"}]}}`

// GetXrayConfig rebuilds every inbound's users from the clients table; a MASQUE user
// emitted without its password makes xray-core refuse the whole config at startup.
func TestGetXrayConfig_EmitsMasqueUserPasswords(t *testing.T) {
	setupConflictDB(t)
	in := &model.Inbound{
		Tag: "in-8443-masque", Enable: true, Listen: "127.0.0.1", Port: 8443, Protocol: model.MASQUE,
		Settings:       `{"clients":[{"email":"ivy@example.test","password":"masque-pass-ivy","enable":true}],"address":["10.14.0.1/24"]}`,
		StreamSettings: masqueTestStream,
	}
	if _, _, err := (&InboundService{}).AddInbound(in); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag != "in-8443-masque" {
			continue
		}
		var settings struct {
			Clients []map[string]any `json:"clients"`
		}
		if err := json.Unmarshal(cfg.InboundConfigs[i].Settings, &settings); err != nil {
			t.Fatalf("decode emitted settings: %v", err)
		}
		if len(settings.Clients) != 1 || settings.Clients[0]["pass"] != "masque-pass-ivy" {
			t.Fatalf("emitted clients = %v, want one with pass masque-pass-ivy", settings.Clients)
		}
		raw, err := json.Marshal(cfg.InboundConfigs[i])
		if err != nil {
			t.Fatalf("marshal emitted inbound: %v", err)
		}
		var emitted map[string]any
		if err := json.Unmarshal(raw, &emitted); err != nil {
			t.Fatalf("decode emitted inbound: %v", err)
		}
		if stream, _ := emitted["streamSettings"].(map[string]any); stream["network"] != "masque" {
			t.Fatalf("emitted streamSettings = %v, want the stored masque transport", emitted["streamSettings"])
		}
		assertXrayAccepts(t, "the emitted MASQUE inbound", buildGoldenInbound(t, emitted))
		return
	}
	t.Fatal("inbound in-8443-masque not found in the generated config")
}

// A client added to a MASQUE inbound without a password could never authenticate,
// and xray-core refuses the whole inbound over one empty pass.
func TestFillProtocolDefaults_MintsMasquePassword(t *testing.T) {
	client := model.Client{Email: "ivy@example.test"}
	if err := (&ClientService{}).fillProtocolDefaults(&client, &model.Inbound{Protocol: model.MASQUE}); err != nil {
		t.Fatalf("fillProtocolDefaults: %v", err)
	}
	if len(client.Password) != 32 {
		t.Fatalf("Password = %q, want a minted 32-character password", client.Password)
	}
	kept := model.Client{Email: "ivy@example.test", Password: "chosen"}
	if err := (&ClientService{}).fillProtocolDefaults(&kept, &model.Inbound{Protocol: model.MASQUE}); err != nil {
		t.Fatalf("fillProtocolDefaults: %v", err)
	}
	if kept.Password != "chosen" {
		t.Fatalf("Password = %q, want the chosen password kept", kept.Password)
	}
}

// Editing a MASQUE client must key on its password like trojan; falling back to the
// empty UUID refused every MASQUE client edit with "empty client ID".
func TestUpdateInboundClient_MasquePasswordChange(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	source := []model.Client{{Email: "ivy@x", Password: "pw-old", SubID: "sub-ivy", Enable: true}}
	ib := mkInbound(t, 22101, model.MASQUE, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, ib.Id, source); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}

	updated := []model.Client{{Email: "ivy@x", Password: "pw-new", SubID: "sub-ivy", Enable: true}}
	if _, err := svc.UpdateInboundClient(&InboundService{}, &model.Inbound{
		Id:       ib.Id,
		Settings: clientsSettings(t, updated),
	}, "ivy@x"); err != nil {
		t.Fatalf("UpdateInboundClient: %v", err)
	}

	if rec := lookupClientRecord(t, "ivy@x"); rec.Password != "pw-new" {
		t.Fatalf("stored password = %q, want pw-new", rec.Password)
	}
}
