package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func setupTestDB(t *testing.T) service.SettingService {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return service.SettingService{}
}

func TestSendMessage_Success(t *testing.T) {
	settingService := setupTestDB(t)
	if err := settingService.SetDiscordBotToken("test-bot-token"); err != nil {
		t.Fatal(err)
	}
	if err := settingService.SetDiscordChannelId("123456789012345678"); err != nil {
		t.Fatal(err)
	}

	var reqMethod, reqPath, reqAuth, reqUA, reqCT string
	var reqPayload MessagePayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqMethod = r.Method
		reqPath = r.URL.Path
		reqAuth = r.Header.Get("Authorization")
		reqUA = r.Header.Get("User-Agent")
		reqCT = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &reqPayload)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "msg-123"}`))
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	payload := MessagePayload{
		Content: "Hello Discord!",
		Embeds: []Embed{
			{
				Title:       "Test Embed",
				Description: "Desc",
				Color:       ColorGreen,
			},
		},
	}

	if err := svc.SendMessage(context.Background(), payload); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if reqMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", reqMethod)
	}
	expectedPath := "/channels/123456789012345678/messages"
	if reqPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, reqPath)
	}
	if reqAuth != "Bot test-bot-token" {
		t.Errorf("expected auth 'Bot test-bot-token', got %s", reqAuth)
	}
	if reqUA != discordUserAgent {
		t.Errorf("expected User-Agent %s, got %s", discordUserAgent, reqUA)
	}
	if reqCT != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", reqCT)
	}
	if reqPayload.Content != "Hello Discord!" || len(reqPayload.Embeds) != 1 {
		t.Errorf("payload mismatch: %+v", reqPayload)
	}
}

func TestSendMessage_CreatedStatus(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("token")
	_ = settingService.SetDiscordChannelId("ch-1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	if err := svc.SendEmbed(context.Background(), Embed{Title: "Title"}); err != nil {
		t.Fatalf("SendEmbed failed: %v", err)
	}
}

func TestSendMessage_StatusCodes(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		respBody   string
		wantErrSub string
	}{
		{"Bad Request", http.StatusBadRequest, `{"message": "Invalid Form Body"}`, "discord bad request (400)"},
		{"Unauthorized", http.StatusUnauthorized, `{"message": "401: Unauthorized"}`, "discord unauthorized (401)"},
		{"Forbidden", http.StatusForbidden, `{"message": "Missing Permissions"}`, "discord forbidden (403)"},
		{"NotFound", http.StatusNotFound, `{"message": "Unknown Channel"}`, "discord not found (404)"},
		{"RateLimited", http.StatusTooManyRequests, `{"retry_after": 1.5}`, "discord rate limited (429)"},
		{"InternalError", http.StatusInternalServerError, `{"message": "Server Error"}`, "discord API error (500)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settingService := setupTestDB(t)
			_ = settingService.SetDiscordBotToken("token")
			_ = settingService.SetDiscordChannelId("ch-1")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.respBody))
			}))
			defer server.Close()

			svc := NewDiscordService(settingService)
			svc.SetBaseURL(server.URL)
			svc.SetHTTPClient(server.Client())

			err := svc.SendEmbed(context.Background(), Embed{Title: "Test"})
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tc.statusCode)
			}
			if !strings.Contains(err.Error(), tc.wantErrSub) {
				t.Errorf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
			}
		})
	}
}

func TestSendMessage_MissingConfig(t *testing.T) {
	settingService := setupTestDB(t)
	svc := NewDiscordService(settingService)

	// Both empty
	err := svc.SendMessage(context.Background(), MessagePayload{Content: "Hi"})
	if err == nil || !strings.Contains(err.Error(), "token is not configured") {
		t.Fatalf("expected token not configured error, got %v", err)
	}

	// Token set, channel empty
	_ = settingService.SetDiscordBotToken("some-token")
	err = svc.SendMessage(context.Background(), MessagePayload{Content: "Hi"})
	if err == nil || !strings.Contains(err.Error(), "channel id is not configured") {
		t.Fatalf("expected channel id not configured error, got %v", err)
	}
}

func TestSendTest(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("999888777")

	var receivedPayload MessagePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	if err := svc.SendTest(context.Background()); err != nil {
		t.Fatalf("SendTest failed: %v", err)
	}

	if len(receivedPayload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(receivedPayload.Embeds))
	}

	embed := receivedPayload.Embeds[0]
	if embed.Color != ColorGreen {
		t.Errorf("expected ColorGreen (0x%X), got 0x%X", ColorGreen, embed.Color)
	}
	if embed.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	} else {
		parsed, err := time.Parse(time.RFC3339, embed.Timestamp)
		if err != nil {
			t.Errorf("timestamp is not RFC3339: %v", err)
		}
		if parsed.Location() != time.UTC {
			t.Errorf("expected UTC timestamp location, got %v", parsed.Location())
		}
	}
	if len(embed.Fields) == 0 {
		t.Error("expected test embed to have fields")
	}
}

func TestSendMessage_BotPrefixHandling(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("Bot prefixed-token")
	_ = settingService.SetDiscordChannelId("ch-100")

	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	if err := svc.SendEmbed(context.Background(), Embed{Title: "Prefix Test"}); err != nil {
		t.Fatalf("SendEmbed failed: %v", err)
	}
	if receivedAuth != "Bot prefixed-token" {
		t.Errorf("expected 'Bot prefixed-token', got %q", receivedAuth)
	}
}

func TestSendMessage_NoContentStatus(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("token")
	_ = settingService.SetDiscordChannelId("ch-204")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	if err := svc.SendEmbed(context.Background(), Embed{Title: "204 Test"}); err != nil {
		t.Fatalf("SendEmbed failed for 204: %v", err)
	}
}

func TestSendMessage_ContextCancelled(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("token")
	_ = settingService.SetDiscordChannelId("ch-1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.SendMessage(ctx, MessagePayload{Content: "Cancelled"})
	if err == nil {
		t.Fatal("expected error with cancelled context, got nil")
	}
}

func TestSendMessageWithFiles_Success(t *testing.T) {
	settingService := setupTestDB(t)
	_ = settingService.SetDiscordBotToken("test-bot-token")
	_ = settingService.SetDiscordChannelId("ch-multipart")

	var receivedCT string
	var receivedPayload MessagePayload
	receivedFiles := make(map[string][]byte)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCT = r.Header.Get("Content-Type")

		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader error: %v", err)
		}
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart error: %v", err)
			}
			data, _ := io.ReadAll(part)
			formName := part.FormName()
			if formName == "payload_json" {
				_ = json.Unmarshal(data, &receivedPayload)
			} else {
				receivedFiles[part.FileName()] = data
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "msg-files"}`))
	}))
	defer server.Close()

	svc := NewDiscordService(settingService)
	svc.SetBaseURL(server.URL)
	svc.SetHTTPClient(server.Client())

	payload := MessagePayload{
		Content: "Report message",
		Embeds:  []Embed{{Title: "Report Embed"}},
	}
	files := []FileAttachment{
		{Filename: "x-ui.db", Data: []byte("sqlite-db-binary")},
		{Filename: "config.json", Data: []byte(`{"log":{}}`)},
	}

	if err := svc.SendMessageWithFiles(context.Background(), payload, files...); err != nil {
		t.Fatalf("SendMessageWithFiles failed: %v", err)
	}

	if !strings.HasPrefix(receivedCT, "multipart/form-data; boundary=") {
		t.Errorf("expected multipart/form-data content type, got %s", receivedCT)
	}
	if receivedPayload.Content != "Report message" || len(receivedPayload.Embeds) != 1 {
		t.Errorf("payload mismatch: %+v", receivedPayload)
	}
	if string(receivedFiles["x-ui.db"]) != "sqlite-db-binary" {
		t.Errorf("x-ui.db mismatch: %s", string(receivedFiles["x-ui.db"]))
	}
	if string(receivedFiles["config.json"]) != `{"log":{}}` {
		t.Errorf("config.json mismatch: %s", string(receivedFiles["config.json"]))
	}
}
