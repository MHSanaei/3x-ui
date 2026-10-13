package tgbot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The customer keyboard is the only surface every linked account already has,
// so the menu has to be reachable from it or nothing below it can be opened.
func TestCustomerMenuIsReachableFromTheCustomerKeyboard(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	markup := tb.clientKeyboard()

	var found bool
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "client_menu_open" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the customer keyboard has no button that opens the customer menu")
	}
	if !isClientSelfCallback("client_menu_open") {
		t.Error("client_menu_open does not pass the customer gate")
	}
}

// recordingCustomerServer stands in for Telegram and keeps every send as
// "<method> <text>", so a test can tell a served verb from the error fallback.
func recordingCustomerServer(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		sent = append(sent, method+" "+body.Text)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1, "date": 0, "chat": map[string]any{"id": ownerTgID, "type": "private"},
		}})
	}))
	t.Cleanup(srv.Close)
	swapTestBot(t, srv.URL)
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), sent...)
	}
}

// seedLinkableCustomer binds one config to ownerTgID with everything the link
// builders read, so each verb can succeed rather than fail into a reply.
func seedLinkableCustomer(t *testing.T, uuid string) {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	service.RegisterSubLinkProvider(sub.NewLinkProvider())

	db := database.GetDB()
	ib := &model.Inbound{
		UserId: 1, Tag: "in-443", Enable: true, Listen: "203.0.113.5", Port: 443,
		Protocol: model.VLESS, Remark: "in",
		Settings: `{"clients":[{"id":"` + uuid + `","email":"` + ownerMail + `","tgId":4242,"subId":"sub-owned","enable":true}],"decryption":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	rec := &model.ClientRecord{Email: ownerMail, SubID: "sub-owned", UUID: uuid, TgID: ownerTgID, Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: ownerMail, Enable: true}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
}

// A one-config customer once got the error fallback for main's three verbs, so
// each verb must produce the output only its own handler sends.
func TestSingleConfigCustomerGetsMainVerbsServed(t *testing.T) {
	const uuid = "11111111-2222-4333-8444-555555555555"
	vlessLink := "vless://" + uuid + "@203.0.113.5:443"
	cases := []struct {
		verb string
		want string
	}{
		{verb: "client_sub_links", want: "/sub-owned</code>"},
		{verb: "client_individual_links", want: vlessLink},
		{verb: "client_qr_links", want: "sendDocument "},
		{verb: "client_one_link", want: "EMAIL=" + ownerMail},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			sent := recordingCustomerServer(t)
			seedLinkableCustomer(t, uuid)
			customerLocalizer(t)
			runningBot(t)

			(&Tgbot{}).clientEmailPicker(7001, ownerTgID, tc.verb, false)

			got := strings.Join(sent(), "\n")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("%s for a one-config customer sent %q, want it to contain %q", tc.verb, got, tc.want)
			}
			if tc.verb == "client_one_link" && !strings.Contains(got, vlessLink) {
				t.Fatalf("client_one_link sent %q, want the header followed by the client's only link", got)
			}
		})
	}
}

// A verb no handler claims would leave the button spinning, so it is answered.
func TestUnhandledSelfVerbIsAnsweredNotSwallowed(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)
	runningBot(t)

	tb.runClientSelfAction(7001, "client_not_a_verb", ownerMail)

	if n := calls("sendMessage"); n == 0 {
		t.Fatal("an unhandled verb produced no reply at all")
	}
}

// Every menu button must pass the customer gate, or a customer's tap falls
// through to the "error" answer and nothing happens.
func TestEveryMenuButtonIsServedAndGated(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	markup := tb.clientMenuKeyboard()
	seen := 0
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			seen++
			if btn.CallbackData == "" {
				t.Error("a menu button carries no callback data")
				continue
			}
			data := btn.CallbackData
			if data == "client_menu" {
				continue
			}
			if !isClientSelfCallback(data) {
				t.Errorf("menu button %q does not pass the customer gate", data)
			}
		}
	}
	if seen == 0 {
		t.Fatal("the menu keyboard has no buttons")
	}
}

// The menu and the docs both promise a Subscription URL, so the button has to
// exist and carry the verb that serves it.
func TestMenuOffersSubscriptionUrl(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	var found bool
	for _, row := range tb.clientMenuKeyboard().InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == "client_sub_links" {
				found = true
			}
		}
	}
	if !found {
		t.Error("the menu offers no Subscription URL, which the PR description and docs both list")
	}
}

// The menu's "All links" button must send the all-links verb. It once carried
// the one-link verb instead, so the label did not describe the action.
func TestMenuAllLinksButtonCarriesTheRightVerb(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	verbs := map[string]string{}
	for _, row := range tb.clientMenuKeyboard().InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != "" {
				verbs[btn.Text] = btn.CallbackData
			}
		}
	}
	allLinks, ok := verbs[tb.I18nBot("tgbot.buttons.getAllLinks")]
	if !ok {
		t.Fatal("the menu has no All links button")
	}
	if allLinks != "client_individual_links" {
		t.Errorf("All links sends %q, want client_individual_links", allLinks)
	}
}
