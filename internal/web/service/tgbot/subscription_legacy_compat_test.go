package tgbot

import (
	"context"
	"html"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestLegacySubscriptionURIsRemainAvailableWithIndependentJSONExports(t *testing.T) {
	tb, email := seedExportClient(t)
	exportRecorder(t)
	db := database.GetDB()
	set := func(key, value string) {
		t.Helper()
		if err := db.Model(&model.Setting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"sub.example.test/custom", "custom://subscription", "https://user:pass@example.test/sub", "https://example.test/sub path"} {
		set("subURI", raw)
		primary, _, err := tb.buildSubscriptionURLs(email)
		if err != nil || !strings.HasPrefix(primary, raw+"/") {
			t.Errorf("legacy URI %q stopped working: %q %v", raw, primary, err)
		}
		if err := tb.sendClientExport(context.Background(), adminTgID, email, "url"); err == nil {
			t.Errorf("new export accepted unsafe URI %q", raw)
		}
	}
	set("subURI", "https://sub.example.test/sub")
	set("subJsonURI", "https://")
	for _, enabled := range []string{"true", "false"} {
		set("subJsonEnable", enabled)
		primary, jsonURL, err := tb.buildSubscriptionURLs(email)
		if err != nil || !strings.HasPrefix(primary, "https://sub.example.test/sub/") {
			t.Fatalf("optional JSON broke primary URL: %q %v", primary, err)
		}
		if enabled == "false" && jsonURL != "" {
			t.Fatalf("disabled JSON URL leaked: %q", jsonURL)
		}
		if err := tb.sendClientExport(context.Background(), adminTgID, email, "url"); err != nil {
			t.Fatal("valid primary export failed:", err)
		}
		if err := tb.sendClientExport(context.Background(), adminTgID, email, "json"); err == nil {
			t.Fatal("invalid/disabled optional JSON exported")
		}
	}
}

func TestLegacyLinksIndividualAndQRScreensAcceptCustomPrimaryURI(t *testing.T) {
	_, email := seedExportClient(t)
	tb, _ := newScreenTgbot(t, false)
	localizeWithRealBundle(t)
	db := database.GetDB()
	if err := db.Model(&model.Setting{}).Where("key = ?", "subURI").Update("value", "sub.example.test/custom").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Setting{}).Where("key = ?", "subJsonURI").Update("value", "not-an-http-uri").Error; err != nil {
		t.Fatal(err)
	}
	for _, chatID := range []int64{adminTgID, 123456} {
		for _, kind := range []string{"links", "individual", "qr"} {
			switch kind {
			case "links":
				tb.screenClientLinks(chatID, email)
			case "individual":
				tb.screenIndividualLinks(chatID, email)
			case "qr":
				tb.screenClientQR(chatID, email)
			}
			screen, ok := tb.screens().get(chatID)
			if !ok || !strings.Contains(screen.body, html.EscapeString(email)) {
				t.Fatalf("legacy %s screen failed for chat %d: %+v", kind, chatID, screen)
			}
			if kind != "individual" && !strings.Contains(screen.body, "sub.example.test/custom/") {
				t.Fatalf("legacy primary URI missing from %s: %s", kind, screen.body)
			}
		}
	}
}
