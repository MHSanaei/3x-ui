package sub

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// inboundLinks (the "Export all inbound links" path) must render the remark
// template's whole Client token group per client, name-only — the same engine
// the client/QR pages use.
func TestInboundLinks_RemarkTemplateClientTokens(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	settings := `{"clients":[{"id":"11111111-2222-4333-8444-000000000001","email":"john@e","subId":"subABC","comment":"vip","tgId":777,"enable":true}],"decryption":"none"}`
	ib := &model.Inbound{
		UserId: 1, Tag: "t", Enable: true, Listen: "203.0.113.5", Port: 4431,
		Protocol: model.VLESS, Remark: "Germany", Settings: settings,
		StreamSettings: `{"network":"ws","security":"tls","wsSettings":{"path":"/","host":""},"tlsSettings":{"serverName":"sni"}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{
		Email: "john@e", SubID: "subABC", UUID: "11111111-2222-4333-8444-000000000001",
		Enable: true, Comment: "vip", TgID: 777,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	svc := NewSubService("{{INBOUND}}-{{EMAIL}}-{{COMMENT}}-{{SUB_ID}}-{{TELEGRAM_ID}}-{{SHORT_ID}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D")
	svc.PrepareForRequest("req.example.com")
	links := svc.inboundLinks(ib)

	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(links), links)
	}
	frag := links[0]
	for _, want := range []string{"Germany-john", "vip", "subABC", "777", "11111111"} {
		if !strings.Contains(frag, want) {
			t.Fatalf("remark missing client token %q: %s", want, frag)
		}
	}
	if strings.Contains(frag, "GB") || strings.ContainsRune(frag, '⏳') {
		t.Fatalf("display mode must drop the traffic/expiry segments: %s", frag)
	}
}

// inboundLinks must use the clients-table UUID when the inbound settings JSON
// still embeds a stale id (#6436).
func TestInboundLinks_UsesClientsTableUUIDWhenSettingsStale(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	stale := "11111111-1111-1111-1111-111111111111"
	fresh := "22222222-2222-2222-2222-222222222222"
	settings := `{"clients":[{"id":"` + stale + `","email":"stale@e","subId":"subStale","enable":true}],"decryption":"none"}`
	ib := &model.Inbound{
		UserId: 1, Tag: "stale-uuid", Enable: true, Listen: "203.0.113.5", Port: 4432,
		Protocol: model.VLESS, Remark: "Stale", Settings: settings,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: "stale@e", SubID: "subStale", UUID: fresh, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}

	svc := NewSubService("{{EMAIL}}")
	svc.PrepareForRequest("req.example.com")
	links := svc.inboundLinks(ib)
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1: %v", len(links), links)
	}
	if !strings.Contains(links[0], fresh) {
		t.Fatalf("link missing fresh UUID %q: %s", fresh, links[0])
	}
	if strings.Contains(links[0], stale) {
		t.Fatalf("link still carries stale settings UUID %q: %s", stale, links[0])
	}
}

// inboundLinks must keep each WireGuard/AmneziaWG inbound's own tunnel address
// and private key when the same email is attached to both — those fields live
// only in the per-inbound settings JSON, not the shared clients.wg_* columns.
func TestInboundLinks_PreservesPerInboundWireGuardIdentity(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()

	serverPriv, serverPub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	wgPriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("wg client keypair: %v", err)
	}
	awgPriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("awg client keypair: %v", err)
	}
	// Shared clients row deliberately holds the *other* tunnel's key/address
	// (last sync wins) — the failure mode ListClientsForInbound alone would export.
	mergedPriv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("merged keypair: %v", err)
	}

	email := "dual@e"
	wgSettings := `{"secretKey":"` + serverPriv + `","clients":[{"email":"` + email + `","privateKey":"` + wgPriv + `","allowedIPs":["10.0.0.5/32"],"enable":true}]}`
	awgSettings := `{"server":{"privateKey":"` + serverPriv + `","publicKey":"` + serverPub + `","mtu":1420},` +
		`"clients":[{"email":"` + email + `","privateKey":"` + awgPriv + `","allowedIPs":["10.8.1.5/32"],"enable":true}]}`

	wgIb := &model.Inbound{
		UserId: 1, Tag: "wg-dual", Enable: true, Listen: "203.0.113.7", Port: 51820,
		Protocol: model.WireGuard, Remark: "WG", Settings: wgSettings,
	}
	awgIb := &model.Inbound{
		UserId: 1, Tag: "awg-dual", Enable: true, Listen: "203.0.113.8", Port: 443,
		Protocol: model.AmneziaWG, Remark: "AWG", Settings: awgSettings,
	}
	for _, ib := range []*model.Inbound{wgIb, awgIb} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %s: %v", ib.Tag, err)
		}
	}
	rec := &model.ClientRecord{
		Email: email, SubID: "subDual", Enable: true,
		PrivateKey: mergedPriv, AllowedIPs: "10.9.9.9/32",
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	for _, ib := range []*model.Inbound{wgIb, awgIb} {
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("create client_inbound %s: %v", ib.Tag, err)
		}
	}

	svc := NewSubService("{{EMAIL}}")
	svc.PrepareForRequest("req.example.com")

	wgLinks := svc.inboundLinks(wgIb)
	if len(wgLinks) != 1 {
		t.Fatalf("wg links = %d, want 1: %v", len(wgLinks), wgLinks)
	}
	wu, err := url.Parse(wgLinks[0])
	if err != nil {
		t.Fatalf("wg link parse: %v (%s)", err, wgLinks[0])
	}
	if wu.User.Username() != wgPriv {
		t.Fatalf("wg private key = %q, want inbound settings key %q (not merged %q)", wu.User.Username(), wgPriv, mergedPriv)
	}
	if got := wu.Query().Get("address"); got != "10.0.0.5/32" {
		t.Fatalf("wg address = %q, want 10.0.0.5/32 (not merged 10.9.9.9/32)", got)
	}

	awgLinks := svc.inboundLinks(awgIb)
	if len(awgLinks) != 1 {
		t.Fatalf("awg links = %d, want 1: %v", len(awgLinks), awgLinks)
	}
	if !strings.HasPrefix(awgLinks[0], "vpn://") {
		t.Fatalf("awg link = %q, want vpn:// prefix", awgLinks[0])
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(awgLinks[0], "vpn://"))
	if err != nil {
		t.Fatalf("awg link decode: %v (%s)", err, awgLinks[0])
	}
	textCfg := string(raw)
	if !strings.Contains(textCfg, "PrivateKey = "+awgPriv) {
		t.Fatalf("awg config missing inbound private key %q:\n%s", awgPriv, textCfg)
	}
	if !strings.Contains(textCfg, "Address = 10.8.1.5/32") {
		t.Fatalf("awg config missing inbound address 10.8.1.5/32:\n%s", textCfg)
	}
	if strings.Contains(textCfg, mergedPriv) || strings.Contains(textCfg, "10.9.9.9/32") {
		t.Fatalf("awg config leaked merged clients-table tunnel identity:\n%s", textCfg)
	}
}
