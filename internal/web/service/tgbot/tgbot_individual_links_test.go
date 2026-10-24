package tgbot

import (
	"encoding/json"
	"io"
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

// recordingBotServer answers sendMessage and records every text it was asked to send.
func recordingBotServer(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var texts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		switch method {
		case "sendMessage", "sendPhoto":
			// A screen is a photo message, so its caption travels as multipart
			// form data; the raw body carries it either way.
			raw, _ := io.ReadAll(r.Body)
			mu.Lock()
			texts = append(texts, string(raw))
			mu.Unlock()
		}
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
		return append([]string(nil), texts...)
	}
}

// With no sub or web domain set, the only host the bot knows is the machine
// name, which need not resolve; the links must not depend on reaching it.
func TestIndividualLinksDoNotNeedAResolvableHost(t *testing.T) {
	sent := recordingBotServer(t)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	service.RegisterSubLinkProvider(sub.NewLinkProvider())

	const uuid = "11111111-2222-4333-8444-555555555555"
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

	origHost, origRunning := hostname, isRunning
	t.Cleanup(func() { hostname, isRunning = origHost, origRunning })
	hostname, isRunning = "unresolvable-panel-host.invalid", true

	tapClientLinks(t, &Tgbot{}, ownerTgID, "client_individual_links "+ownerMail)

	got := strings.Join(sent(), "\n")
	if !strings.Contains(got, "vless://"+uuid+"@203.0.113.5:443") {
		t.Fatalf("bot sent %q, want the client's vless link", got)
	}
}
