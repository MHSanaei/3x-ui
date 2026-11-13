package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGetLinkWireGuardIncludesVKClientProfile(t *testing.T) {
	serverPrivate, serverPublic := mustWireguardKeypair(t)
	clientPrivate, _ := mustWireguardKeypair(t)
	inbound := &model.Inbound{
		Protocol: model.WireGuard, Port: 51820, Listen: "203.0.113.7",
		Settings: fmt.Sprintf(`{"secretKey":%q,"dns":"9.9.9.9","clients":[{"email":"alice","privateKey":%q,"allowedIPs":["fd00::2/128","10.77.0.2/32"],"preSharedKey":"psk"}],"vkTurn":{"enabled":true,"callLink":"https://vk.ru/call/join/example","port":56000}}`, serverPrivate, clientPrivate),
	}
	s := &SubService{}
	links := strings.Split(s.GetLink(inbound, "alice"), "\n")
	if len(links) != 2 || !strings.HasPrefix(links[0], "wireguard://") {
		t.Fatalf("links = %v, want standard WireGuard and VK TURN profiles", links)
	}
	u, err := url.Parse(links[1])
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "vkturnproxy" || u.Host != "import" {
		t.Fatalf("VK import URL = %s", links[1])
	}
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("data"))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Version  int            `json:"version"`
		Type     string         `json:"type"`
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != 1 || payload.Type != "connection" {
		t.Fatalf("invalid import envelope: %s", raw)
	}
	for field, want := range map[string]any{
		"privateKey": clientPrivate, "peerPublicKey": serverPublic, "presharedKey": "psk",
		"tunnelAddress": "10.77.0.2/32", "peerAddress": "203.0.113.7:56000",
		"vkLink": "https://vk.ru/call/join/example", "dnsServers": "9.9.9.9",
		"useSrtp": true, "useUDP": false, "numConnections": float64(30),
	} {
		if got := payload.Settings[field]; got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}
	if strings.Contains(string(raw), serverPrivate) {
		t.Fatal("VK client profile contains the server private key")
	}
}
