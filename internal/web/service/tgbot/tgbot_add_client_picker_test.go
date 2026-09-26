package tgbot

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/mymmrac/telego"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func seedPickerInbounds(t *testing.T, protocols ...model.Protocol) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	for i, protocol := range protocols {
		port := 20000 + i
		ib := &model.Inbound{Remark: string(protocol), Enable: true, Port: port, Protocol: protocol, Tag: fmt.Sprintf("inbound-%d", port), Settings: `{}`}
		if err := database.GetDB().Create(ib).Error; err != nil {
			t.Fatalf("seed %s inbound: %v", protocol, err)
		}
	}
}

func pickerLabels(keyboard *telego.InlineKeyboardMarkup) []string {
	var labels []string
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			labels = append(labels, button.Text)
		}
	}
	slices.Sort(labels)
	return labels
}

// WireGuard and AmneziaWG clients get a generated keypair and address on Create,
// so the wizard offers them; Mixed authenticates per inbound and has no clients.
func TestAddClientPickerOffersWireGuardAndAmneziaWG(t *testing.T) {
	seedPickerInbounds(t, model.VLESS, model.WireGuard, model.AmneziaWG, model.Mixed)

	keyboard, err := (&Tgbot{}).getInboundsAddClient()
	if err != nil {
		t.Fatalf("getInboundsAddClient: %v", err)
	}
	want := []string{"amneziawg - ✅", "vless - ✅", "wireguard - ✅"}
	if got := pickerLabels(keyboard); !slices.Equal(got, want) {
		t.Fatalf("picker buttons = %q, want %q", got, want)
	}
}

// An empty keyboard sent the admin a "choose inbound" prompt with nothing to tap.
func TestAddClientPickerFailsWhenNoInboundTakesClients(t *testing.T) {
	draftLocalizer(t, &i18n.Message{ID: "tgbot.answers.getInboundsFailed", Other: "Failed to get inbounds."})
	seedPickerInbounds(t, model.Mixed, model.HTTP, model.Tunnel)

	keyboard, err := (&Tgbot{}).getInboundsAddClient()
	if err == nil || err.Error() != "Failed to get inbounds." {
		t.Fatalf("getInboundsAddClient = (%v, %v), want the getInboundsFailed error", keyboard, err)
	}
}
