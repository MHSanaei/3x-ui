package tgbot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mymmrac/telego"
)

const (
	adminTgID      = int64(31337)
	adminCardEmail = "admin-card@example.com"
)

// seedAdminClient stores one client the way the panel keeps it: an inbound row
// with its settings blob, a traffic row and a client record.
func seedAdminClient(t *testing.T, email string) {
	t.Helper()
	inbound := &model.Inbound{
		UserId: 1, Tag: "in-" + email, Remark: "in-" + email, Port: 8443,
		Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[{"email":"` + email + `","enable":true,"subId":"sub-` + email + `"}]}`,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: inbound.Id, Email: email, Enable: true,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	record := (&model.Client{Email: email, Enable: true, SubID: "sub-" + email}).ToRecord()
	if err := database.GetDB().Create(record).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := database.GetDB().Create(&model.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("seed client_inbounds: %v", err)
	}
}

// cardCallback finds the callback data of the card's button for one action.
func cardCallback(t *testing.T, sc *screen, action string) string {
	t.Helper()
	for _, row := range sc.rows {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, action+" ") {
				return btn.CallbackData
			}
		}
	}
	t.Fatalf("the client card offers no %q button: %+v", action, sc.rows)
	return ""
}

// The v3.11.0 bot rebuild left the admin's client card without a way to the
// client's subscription link or QR code, so an admin could not hand a config to
// a client who cannot reach Telegram.
func TestAdminClientCardOffersTheLinksAndQRCode(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t, adminTgID)
	seedAdminClient(t, adminCardEmail)

	tb.searchClientScreen(adminTgID, adminCardEmail)

	sc, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	markup, _ := json.Marshal(sc.markup)
	for _, want := range []string{"client_sub_links " + adminCardEmail, "client_qr_links " + adminCardEmail} {
		if !strings.Contains(string(markup), want) {
			t.Errorf("the admin's client card lacks %q: %s", want, markup)
		}
	}
}

// The button has to work, not just exist: tapping the card's own button must
// draw the subscription screen, because a URL the admin can read is the whole
// point of the flow.
func TestAdminTappingTheClientCardLinksDrawsTheSubscription(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	localizeWithRealBundle(t)
	setTestAdmins(t, adminTgID)
	seedAdminClient(t, adminCardEmail)

	tb.searchClientScreen(adminTgID, adminCardEmail)
	sc, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	data := cardCallback(t, sc, "client_sub_links")

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: adminTgID},
		Data:    data,
		Message: &telego.Message{Chat: telego.Chat{ID: adminTgID}},
	}, true)

	links, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	if links.kind != "links" {
		t.Fatalf("screen kind = %q, want the links screen", links.kind)
	}
	if !strings.Contains(links.text, "sub-") {
		t.Errorf("the links screen carries no subscription url: %q", links.text)
	}
	if countMethod(calls(), "editMessageMedia")+countMethod(calls(), "editMessageText") == 0 {
		t.Error("the links screen was never drawn")
	}
}

// The QR code is the second half of the request: an admin sends a picture
// straight to a client, so the card must reach it too.
func TestAdminTappingTheClientCardQRCodeDrawsTheCode(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t, adminTgID)
	seedAdminClient(t, adminCardEmail)

	tb.searchClientScreen(adminTgID, adminCardEmail)
	sc, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	data := cardCallback(t, sc, "client_qr_links")

	tb.answerCallback(&telego.CallbackQuery{
		ID:      "q1",
		From:    telego.User{ID: adminTgID},
		Data:    data,
		Message: &telego.Message{Chat: telego.Chat{ID: adminTgID}},
	}, true)

	qr, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("no screen was tracked")
	}
	if qr.kind != "qr" {
		t.Fatalf("screen kind = %q, want the qr screen", qr.kind)
	}
	if qr.qrPicture == nil {
		t.Error("the qr screen carries no sheet of codes")
	}
	if countMethod(calls(), "sendPhoto") == 0 && countMethod(calls(), "editMessageMedia") == 0 {
		t.Error("the qr screen was never drawn")
	}
}
