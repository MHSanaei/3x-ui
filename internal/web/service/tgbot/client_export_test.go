package tgbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mymmrac/telego"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type exportCall struct {
	method, text, filename string
	file                   []byte
}

func exportRecorder(t *testing.T) (func() []exportCall, *atomic.Bool) {
	t.Helper()
	var mu sync.Mutex
	var calls []exportCall
	fail := &atomic.Bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := exportCall{method: r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			defer r.MultipartForm.RemoveAll()
			call.text = r.FormValue("caption")
			for _, files := range r.MultipartForm.File {
				for _, header := range files {
					file, err := header.Open()
					if err != nil {
						t.Error(err)
						continue
					}
					call.file, err = io.ReadAll(file)
					if err != nil {
						t.Error(err)
					}
					_ = file.Close()
					call.filename = header.Filename
				}
			}
		} else {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			call.text, _ = payload["text"].(string)
		}
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if fail.Load() {
			_, _ = io.WriteString(w, `{"ok":false,"error_code":400,"description":"test upload failed"}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":31337,"type":"private"}}}`)
	}))
	swapTestBot(t, server.URL)
	t.Cleanup(server.Close)
	return func() []exportCall { mu.Lock(); defer mu.Unlock(); return append([]exportCall(nil), calls...) }, fail
}

func seedExportClient(t *testing.T) (*Tgbot, string) {
	t.Helper()
	initReportDB(t)
	localizeWithRealBundle(t)
	setTestAdmins(t, adminTgID)
	email := "علی<&>"
	seedAdminClient(t, email)
	db := database.GetDB()
	var original model.Inbound
	if err := db.First(&original).Error; err != nil {
		t.Fatal(err)
	}
	var record model.ClientRecord
	if err := db.Where("email = ?", email).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555", Email: email, SubID: record.SubID, Enable: true}
	settings, err := json.Marshal(map[string]any{"clients": []model.Client{client}, "decryption": "none"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 12 {
		ib := original
		ib.Id = 0
		ib.Port = 8400 + i
		ib.Tag = fmt.Sprintf("export-%d", i)
		ib.Settings = string(settings)
		ib.Listen = "203.0.113.5"
		if i == 0 {
			if err := db.Model(&original).Updates(map[string]any{"settings": ib.Settings, "listen": ib.Listen}).Error; err != nil {
				t.Fatal(err)
			}
		} else {
			if err := db.Create(&ib).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: ib.Id}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for key, value := range map[string]string{"subEnable": "true", "subURI": "https://sub.example.test/sub", "subJsonEnable": "true", "subJsonURI": "https://json.example.test/sub"} {
		if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	service.RegisterSubLinkProvider(sub.NewLinkProvider())
	return &Tgbot{}, email
}

func TestClientExportUploadsEveryLinkAndMatchingPrimaryQR(t *testing.T) {
	tb, email := seedExportClient(t)
	calls, fail := exportRecorder(t)
	primary, _, err := tb.buildSubscriptionURLs(email)
	if err != nil {
		t.Fatal(err)
	}
	want, err := tb.clientSubLinks(email, primary)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) < 12 {
		t.Fatalf("fixture only generated %d links", len(want))
	}
	if err := tb.sendClientExport(context.Background(), adminTgID, email, "txt"); err != nil {
		t.Fatal(err)
	}
	got := calls()[0]
	if got.method != "sendDocument" || string(got.file) != strings.Join(want, "\n")+"\n" {
		t.Fatalf("incomplete or altered TXT export: %+v", got)
	}
	if !strings.HasPrefix(got.filename, "علی") || strings.ContainsAny(got.filename, "<>/\\") || !strings.HasSuffix(got.filename, ".txt") {
		t.Fatalf("unsafe/incorrect filename: %q", got.filename)
	}
	if err := tb.sendClientExport(context.Background(), adminTgID, email, "qr"); err != nil {
		t.Fatal(err)
	}
	got = calls()[1]
	qr, err := qrPNG(primary, 512)
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "sendPhoto" || !bytes.Equal(qr, got.file) || !strings.Contains(got.text, "https://sub.example.test/sub/") || strings.Contains(got.text, "t.me/") {
		t.Fatalf("photo/URL mismatch: %+v", got)
	}
	fail.Store(true)
	if err := tb.sendClientExport(context.Background(), adminTgID, email, "txt"); err == nil {
		t.Fatal("failed Telegram upload reported success")
	}
}

func TestClientExportCallbacksRecheckPrivateAdminAuthorization(t *testing.T) {
	tb, email := seedExportClient(t)
	calls, _ := exportRecorder(t)
	for _, tc := range []struct {
		user, chat int64
		kind       string
		message    bool
	}{
		{ownerTgID, ownerTgID, "private", true},
		{adminTgID, -100, "supergroup", true},
		{adminTgID, ownerTgID, "private", true},
		{adminTgID, adminTgID, "private", false},
	} {
		query := &telego.CallbackQuery{ID: "export", From: telego.User{ID: tc.user}, Data: "client_export_txt " + email}
		if tc.message {
			query.Message = &telego.Message{Chat: telego.Chat{ID: tc.chat, Type: tc.kind}}
		}
		tb.answerCallback(query, true)
	}
	for _, call := range calls() {
		if call.method == "sendDocument" || call.method == "sendPhoto" || call.method == "sendMessage" {
			t.Fatalf("export leaked to unauthorized actor: %+v", call)
		}
	}
	query := &telego.CallbackQuery{ID: "export", From: telego.User{ID: adminTgID}, Data: "client_export_txt " + email, Message: &telego.Message{Chat: telego.Chat{ID: adminTgID, Type: "private"}}}
	tb.answerCallback(query, true)
	for _, call := range calls() {
		if call.method == "sendDocument" {
			return
		}
	}
	t.Fatal("authorized private admin received no export")
}

func TestClientExportHandlesLongAndInvalidURLs(t *testing.T) {
	tb, email := seedExportClient(t)
	calls, _ := exportRecorder(t)
	db := database.GetDB()
	longBase := "https://sub.example.test/" + strings.Repeat("a", 1050)
	if err := db.Model(&model.Setting{}).Where("key = ?", "subURI").Update("value", longBase).Error; err != nil {
		t.Fatal(err)
	}
	if err := tb.sendClientExport(context.Background(), adminTgID, email, "qr"); err != nil {
		t.Fatal(err)
	}
	got := calls()
	if len(got) != 2 || got[0].method != "sendPhoto" || got[1].method != "sendMessage" || !strings.Contains(got[1].text, longBase) {
		t.Fatalf("long QR caption lost its complete associated URL: %+v", got)
	}
	for _, invalid := range []string{"javascript:bad", "https://", "https://user:pass@example.test/", "https://sub.example.test/\n"} {
		if err := db.Model(&model.Setting{}).Where("key = ?", "subURI").Update("value", invalid).Error; err != nil {
			t.Fatal(err)
		}
		if err := tb.sendClientExport(context.Background(), adminTgID, email, "url"); err == nil {
			t.Errorf("invalid URL %q accepted", invalid)
		}
	}
	if err := tb.sendClientExport(context.Background(), adminTgID, "deleted", "txt"); err == nil {
		t.Fatal("deleted client exported")
	}
}

func TestClientExportSummaryUsesGlobalTrafficAndEscapesLabels(t *testing.T) {
	tb, email := seedExportClient(t)
	calls, _ := exportRecorder(t)
	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Updates(map[string]any{"total": int64(10 << 30), "up": int64(1 << 30)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{Email: email, MasterGuid: "export-master", Up: 4 << 30, Down: 2 << 30, UpdatedAt: db.NowFunc().UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tb.sendClientExport(context.Background(), adminTgID, email, "summary"); err != nil {
		t.Fatal(err)
	}
	got := calls()[0].text
	for _, want := range []string{"علی&lt;&amp;&gt;", "6.00GB", "4.00GB", "Attached"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary lacks %q: %s", want, got)
		}
	}
	if strings.Contains(got, "علی<&>") {
		t.Fatal("unescaped summary label")
	}
}

func TestClientExportMenuOffersFourFormatsWithoutReplacingExistingLinks(t *testing.T) {
	tb, email := seedExportClient(t)
	_, _ = exportRecorder(t)
	tb.searchClientScreen(adminTgID, email)
	for _, action := range []string{"client_sub_links", "client_qr_links", "client_export_menu"} {
		_ = workflowCallback(t, tb, action+" ")
	}
	adminWorkflowTap(tb, tb.encodeQuery("client_export_menu "+email), adminTgID)
	for _, kind := range []string{"summary", "url", "txt", "qr", "json"} {
		_ = workflowCallback(t, tb, "client_export_"+kind+" ")
	}
	writeBotScreenEvidence(t, tb, adminTgID, "share-export")
}

func TestClientCardEscapesExportEntryPoint(t *testing.T) {
	tb, email := seedExportClient(t)
	_, _ = exportRecorder(t)
	tb.searchClientScreen(adminTgID, email)
	screen, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("client card missing")
	}
	if strings.Contains(screen.body, email) || !strings.Contains(screen.body, "علی&lt;&amp;&gt;") {
		t.Fatalf("export entry point contains unsafe client/attachment labels: %s", screen.body)
	}
}

func TestOwnClientCardDoesNotOfferAdminWorkflows(t *testing.T) {
	tb, _ := newLinksCallbackTgbot(t, ownerMail)
	localizeWithRealBundle(t)
	setTestAdmins(t, adminTgID)
	tb.searchClientScreen(ownerTgID, ownerMail)
	screen, ok := tb.screens().get(ownerTgID)
	if !ok {
		t.Fatal("own client card missing")
	}
	links := false
	for _, row := range screen.rows {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, "client_export_") || strings.HasPrefix(button.CallbackData, "admin_") {
				t.Fatalf("regular self-service card offers an admin workflow: %s", button.CallbackData)
			}
			if strings.HasPrefix(button.CallbackData, "client_sub_links ") {
				links = true
			}
		}
	}
	if !links {
		t.Fatal("self-service subscription links were removed")
	}
}

func TestAdminWorkflowErrorsEscapeAndBoundDiagnosticDetails(t *testing.T) {
	localizeWithRealBundle(t)
	calls, _ := exportRecorder(t)
	(&Tgbot{}).workflowError(adminTgID, errors.New("<node>& "+strings.Repeat("x", 1000)))
	got := calls()
	if len(got) == 0 {
		t.Fatal("workflow error was not delivered")
	}
	text := got[len(got)-1].text
	if !strings.Contains(text, "&lt;node&gt;&amp;") || strings.Contains(text, "<node>") || len([]rune(text)) > 800 {
		t.Fatalf("unhelpful, unsafe or oversized diagnostic: %q", text)
	}
}
