package tgbot

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// seedLinkedClient binds email to tgID on one VLESS inbound per remark, with the
// real link provider, so the subscription yields one share link per inbound.
func seedLinkedClient(t *testing.T, tgID int64, email string, remarks ...string) {
	t.Helper()
	seedLinkedClientOn(t, model.VLESS, tgID, email, remarks...)
}

func seedLinkedClientOn(t *testing.T, protocol model.Protocol, tgID int64, email string, remarks ...string) {
	t.Helper()
	service.RegisterSubLinkProvider(sub.NewLinkProvider())
	t.Cleanup(func() { service.RegisterSubLinkProvider(nil) })
	origHost := hostname
	t.Cleanup(func() { hostname = origHost })
	hostname = "panel.invalid"

	const uuid = "11111111-2222-4333-8444-555555555555"
	db := database.GetDB()
	rec := &model.ClientRecord{Email: email, SubID: "sub-" + email, UUID: uuid, TgID: tgID, Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	for i, remark := range remarks {
		ib := &model.Inbound{
			UserId: 1, Tag: fmt.Sprintf("one-%d", i), Enable: true, Listen: "203.0.113.5", Port: 4430 + i,
			Protocol: protocol, Remark: remark,
			Settings: fmt.Sprintf(`{"clients":[{"id":%q,"email":%q,"tgId":%d,"subId":%q,"enable":true}],"decryption":"none"}`,
				uuid, email, tgID, rec.SubID),
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("seed client_inbound: %v", err)
		}
		if i == 0 {
			if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: email, Enable: true}).Error; err != nil {
				t.Fatalf("seed traffic: %v", err)
			}
		}
	}
}

func screenMarkup(t *testing.T, tb *Tgbot, chatID int64) string {
	t.Helper()
	sc, ok := tb.screens().get(chatID)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	markup, _ := json.Marshal(sc.markup)
	return string(markup)
}

// The label is what a customer picks from, so it comes from the link's own
// remark — the same name their client app will show for that server.
func TestLinkLabel(t *testing.T) {
	tests := []struct {
		name  string
		link  string
		index int
		want  string
	}{
		{"remark fragment", "vless://uuid@host:443?type=tcp#Germany-Reality", 0, "1. Germany-Reality"},
		{"percent-encoded remark", "vless://uuid@host:443#Tokyo%20Node", 1, "2. Tokyo Node"},
		{"no fragment falls back to protocol", "trojan://pass@host:443", 2, "3. TROJAN"},
		{"empty fragment falls back", "vmess://payload#", 3, "4. VMESS"},
		{"blank fragment falls back", "ss://payload#%20", 4, "5. SS"},
		{"not a url at all", "garbage", 5, "Link 6"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := linkLabel(tc.link, tc.index); got != tc.want {
				t.Fatalf("linkLabel(%q, %d) = %q, want %q", tc.link, tc.index, got, tc.want)
			}
		})
	}
}

// The index is callback data, so a malformed one must not reach a screen, and
// it must never be read as part of the client the ownership check verifies.
func TestSplitEmailIndex(t *testing.T) {
	tests := []struct {
		arg       string
		wantEmail string
		wantIndex int
		wantOK    bool
	}{
		{"alice@x 0", "alice@x", 0, true},
		{"alice@x 12", "alice@x", 12, true},
		{"  alice@x 3  ", "alice@x", 3, true},
		{"two words 3", "two words", 3, true},
		{"alice@x", "", 0, false},
		{"alice@x abc", "", 0, false},
		{"alice@x -1", "", 0, false},
		{" 4", "", 0, false},
		{"", "", 0, false},
	}
	for _, tc := range tests {
		email, index, ok := splitEmailIndex(tc.arg)
		if email != tc.wantEmail || index != tc.wantIndex || ok != tc.wantOK {
			t.Errorf("splitEmailIndex(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.arg, email, index, ok, tc.wantEmail, tc.wantIndex, tc.wantOK)
		}
	}
}

// "Get one" on a config attached to two inbounds offers each by the name its
// app will show, and the chosen one comes back with its own QR as the picture.
func TestGetOneOffersEachLinkAndShowsTheChosenOne(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedLinkedClient(t, 4242, "pick@x", "Alpha", "Beta")

	tapClientLinks(t, tb, 4242, "client_one_link pick@x")
	sc, ok := tb.screens().get(4242)
	if !ok {
		t.Fatal("the picker was not drawn")
	}
	var picked []string
	for _, row := range sc.markup.InlineKeyboard {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, "link_one pick@x ") {
				picked = append(picked, button.Text+" → "+button.CallbackData)
			}
		}
	}
	if len(picked) != 2 || !strings.Contains(picked[0], "Alpha") || !strings.Contains(picked[1], "Beta") {
		t.Fatalf("picker offers %v, want Alpha then Beta", picked)
	}

	tapClientLinks(t, tb, 4242, "link_one pick@x 1")
	sc, _ = tb.screens().get(4242)
	if !strings.Contains(sc.qrSource, "vless://") || !strings.Contains(sc.qrSource, ":4431") {
		t.Fatalf("the QR encodes %q, want Beta's link on port 4431", sc.qrSource)
	}
	if sc.qrPicture == nil || !strings.Contains(sc.body, "Beta") {
		t.Errorf("the chosen link is not on the screen: body %q", sc.body)
	}
}

// VMess keeps its remark inside the base64 body, not after "#", so a picker that
// read only fragments listed every VMess inbound as the same "VMESS".
func TestGetOneNamesVMessLinksByTheirRemark(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedLinkedClientOn(t, model.VMESS, 4242, "vm@x", "Alpha", "Beta")

	tapClientLinks(t, tb, 4242, "client_one_link vm@x")
	sc, ok := tb.screens().get(4242)
	if !ok {
		t.Fatal("the picker was not drawn")
	}
	var labels []string
	for _, row := range sc.markup.InlineKeyboard {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, "link_one vm@x ") {
				labels = append(labels, button.Text)
			}
		}
	}
	if len(labels) != 2 || !strings.Contains(labels[0], "Alpha") || !strings.Contains(labels[1], "Beta") {
		t.Fatalf("picker offers %q, want Alpha then Beta", labels)
	}
}

// One link is nothing to choose from, so the picker opens it at once; a stale
// index from an inbound since removed is refused rather than shown.
func TestGetOneOpensASingleLinkAndRefusesAStaleIndex(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedLinkedClient(t, 4242, "solo@x", "Only")

	tapClientLinks(t, tb, 4242, "client_one_link")
	sc, ok := tb.screens().get(4242)
	if !ok || !strings.Contains(sc.qrSource, ":4430") {
		t.Fatalf("the single link did not open directly: %v", methods(calls()))
	}

	before := len(calls())
	tapClientLinks(t, tb, 4242, "link_one solo@x 3")
	if got := countMethod(calls()[before:], "sendMessage"); got != 1 {
		t.Errorf("a stale index sent %v, want one notice", methods(calls()[before:]))
	}
	if sc, _ := tb.screens().get(4242); !strings.Contains(sc.qrSource, ":4430") {
		t.Errorf("a stale index replaced the screen with %q", sc.qrSource)
	}
}
