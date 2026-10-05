package tgbot

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/mymmrac/telego"
)

// seedPickerInbounds inserts one enabled inbound per protocol.
func seedPickerInbounds(t *testing.T, protocols ...model.Protocol) {
	t.Helper()
	for _, protocol := range protocols {
		tag := "pick-" + string(protocol)
		if err := database.GetDB().Create(&model.Inbound{
			UserId: 1, Tag: tag, Remark: string(protocol), Port: 8443,
			Protocol: protocol, Enable: true,
		}).Error; err != nil {
			t.Fatalf("seed %s inbound: %v", protocol, err)
		}
	}
}

// attachPickerRows builds the attach picker for a draft and flattens it.
func attachPickerRows(t *testing.T, tb *Tgbot, draft *clientDraft) []telego.InlineKeyboardButton {
	t.Helper()
	picker, err := tb.getInboundsAttachPicker(draft)
	if err != nil {
		t.Fatalf("getInboundsAttachPicker: %v", err)
	}
	var rows []telego.InlineKeyboardButton
	for _, row := range picker.InlineKeyboard {
		rows = append(rows, row...)
	}
	return rows
}

// WireGuard and AmneziaWG clients get a generated keypair and address on Create,
// so the attach picker offers them; Mixed authenticates per inbound and has none.
func TestAttachPickerOffersWireGuardAndAmneziaWG(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	seedPickerInbounds(t, model.VLESS, model.WireGuard, model.AmneziaWG, model.Mixed)

	draft := &clientDraft{}
	// The picker marks selection, so its labels carry the checkbox and protocol.
	rows := attachPickerRows(t, tb, draft)
	var labels []string
	for _, row := range rows {
		labels = append(labels, row.Text)
	}
	slices.Sort(labels)
	for _, want := range []string{"amneziawg", "vless", "wireguard"} {
		found := false
		for _, label := range labels {
			if strings.Contains(label, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("the attach picker does not offer %s: %q", want, labels)
		}
	}
	for _, label := range labels {
		if strings.Contains(label, "mixed") {
			t.Errorf("the attach picker offers Mixed, which has no clients: %q", label)
		}
	}
}

// The wizard must start on a DRAFT, not on an inbound picker: the client is
// created first and attached afterwards. This is the bug that made "Add client"
// ask for an inbound and then offer existing clients.
func TestAddClientStartsOnADraftWithNoInboundChosen(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)

	tb.screenAddClientStart(777, 1)

	sc, ok := tb.screens().get(777)
	if !ok {
		t.Fatal("the add-client flow drew no screen")
	}
	// The draft card is shown, and nothing is attached yet.
	if !strings.Contains(sc.pages[0], "newclient") && !strings.Contains(sc.pages[0], "Email") {
		t.Errorf("the first screen is not the draft card: %s", sc.pages[0])
	}
	if strings.Contains(sc.pages[0], "pickInbound") || strings.Contains(sc.pages[0], "Pick the inbound") {
		t.Errorf("the flow still opens on an inbound picker: %s", sc.pages[0])
	}
	markup, _ := json.Marshal(sc.markup)
	if !strings.Contains(string(markup), "add_client_attach_more") {
		t.Errorf("the draft offers no way to attach an inbound: %s", markup)
	}
	if strings.Contains(string(markup), "add_client_ch_default_email") == false {
		t.Errorf("the draft offers no way to set the client's fields: %s", markup)
	}
}

// Attaching is optional while filling the card: the inbounds are chosen from the
// draft's own button, and submitting without one is refused rather than creating
// a client attached to nothing.
func TestDraftCanBeFilledBeforeAnyInboundIsAttached(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)
	draft := &clientDraft{email: "draft-only"}

	rows := tb.getCommonClientButtons(draft)
	markup, _ := json.Marshal(rows)
	for _, want := range []string{"add_client_ch_default_email", "add_client_ch_default_comment",
		"add_client_ch_default_traffic", "add_client_ch_default_exp",
		"add_client_ch_default_ip_limit", "add_client_ch_default_tg_id",
		"add_client_attach_more", "add_client_submit_enable", "add_client_cancel"} {
		if !strings.Contains(string(markup), want) {
			t.Errorf("the draft card lacks %s", want)
		}
	}
	// The attach button says how many are attached, and it is localized.
	for _, row := range rows {
		for _, button := range row {
			if button.CallbackData != "add_client_attach_more" {
				continue
			}
			if strings.TrimSpace(button.Text) == "" || strings.HasPrefix(button.Text, "tgbot.") {
				t.Errorf("the attach button has no localized label: %q", button.Text)
			}
		}
	}

	if _, err := tb.SubmitAddClient(draft); err == nil {
		t.Error("submitting with no inbound attached created a client anyway")
	}
}
