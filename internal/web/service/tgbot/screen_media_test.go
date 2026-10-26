package tgbot

import (
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type screenMediaRequest struct {
	Method    string
	ChatID    int64  `json:"chat_id"`
	MessageID int    `json:"message_id"`
	Caption   string `json:"caption"`
	Photo     string `json:"photo"`
	Media     struct {
		Media   string `json:"media"`
		Caption string `json:"caption"`
	} `json:"media"`
	ReplyMarkup telego.InlineKeyboardMarkup `json:"reply_markup"`
	Uploaded    bool
	UploadName  string
}

type screenMediaAPI struct {
	mu            sync.Mutex
	avatar        string
	code          int
	failUpload    bool
	failEdit      bool
	badResponse   bool
	uploadStarted chan struct{}
	releaseUpload <-chan struct{}
	requests      []screenMediaRequest
}

func newScreenMediaAPI(t *testing.T) (*Tgbot, *screenMediaAPI) {
	t.Helper()
	api := &screenMediaAPI{avatar: "rejected-avatar", code: 400}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		w.Header().Set("Content-Type", "application/json")
		api.mu.Lock()
		avatar := api.avatar
		api.mu.Unlock()
		switch method {
		case "getMe":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"id": 42, "is_bot": true, "first_name": "test", "username": "test_bot", "supports_inline_queries": true,
			}})
			return
		case "getUserProfilePhotos":
			if avatar == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"total_count": 0, "photos": [][]telego.PhotoSize{}}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
				"total_count": 1, "photos": [][]telego.PhotoSize{{{FileID: avatar, FileUniqueID: "avatar", Width: 640, Height: 640}}},
			}})
			return
		}

		request := screenMediaRequest{Method: method}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse upload: %v", err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			request.ChatID = parseInt64(r.FormValue("chat_id"))
			request.MessageID = int(parseInt64(r.FormValue("message_id")))
			request.Caption, request.Photo = r.FormValue("caption"), r.FormValue("photo")
			_ = json.Unmarshal([]byte(r.FormValue("reply_markup")), &request.ReplyMarkup)
			_ = json.Unmarshal([]byte(r.FormValue("media")), &request.Media)
			for _, files := range r.MultipartForm.File {
				for _, file := range files {
					reader, err := file.Open()
					if err != nil {
						t.Errorf("open uploaded photo: %v", err)
						return
					}
					config, err := png.DecodeConfig(reader)
					_ = reader.Close()
					if err != nil || (file.Filename == "screen.png" && (config.Width != 512 || config.Height != 512)) {
						t.Errorf("fallback is not the generated 512x512 PNG: %+v, %v", config, err)
					}
					request.Uploaded = true
					request.UploadName = file.Filename
				}
			}
		} else if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode %s: %v", method, err)
			return
		}
		if method == "editMessageMedia" {
			request.Caption = request.Media.Caption
			request.Photo = request.Media.Media
		}
		api.mu.Lock()
		api.requests = append(api.requests, request)
		code, failUpload, failEdit := api.code, api.failUpload, api.failEdit
		badResponse, started, release := api.badResponse, api.uploadStarted, api.releaseUpload
		id := 100 + len(api.requests)
		api.mu.Unlock()
		if request.Uploaded && started != nil {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		if badResponse {
			_, _ = w.Write([]byte("{"))
			return
		}
		if (request.Photo == "rejected-avatar" && !request.Uploaded) || (failUpload && request.Uploaded) || (failEdit && method == "editMessageMedia") {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": code, "description": "Bad Request: wrong file identifier/HTTP URL specified"})
			return
		}
		if method == "deleteMessage" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": true})
			return
		}
		if request.MessageID != 0 {
			id = request.MessageID
		}
		photoID := request.Photo
		if request.Uploaded {
			photoID = "fallback-photo"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{
			"message_id": id, "date": 0, "chat": map[string]any{"id": request.ChatID, "type": "private"},
			"photo": []telego.PhotoSize{{FileID: photoID, FileUniqueID: photoID, Width: 512, Height: 512}},
		}})
	}))
	t.Cleanup(srv.Close)
	swapTestBot(t, srv.URL)
	tb := &Tgbot{}
	tb.resolveScreenArt()
	return tb, api
}

func (api *screenMediaAPI) snapshot() []screenMediaRequest {
	api.mu.Lock()
	defer api.mu.Unlock()
	return append([]screenMediaRequest(nil), api.requests...)
}

func TestStartRecoversWhenTelegramRejectsAvatar(t *testing.T) {
	tb, api := newScreenMediaAPI(t)
	initReportDB(t)
	setTestAdmins(t, 1)
	draftLocalizer(t,
		&i18n.Message{ID: "tgbot.buttons.catServer", Other: "Server"},
		&i18n.Message{ID: "tgbot.buttons.catClients", Other: "Clients"},
		&i18n.Message{ID: "tgbot.buttons.catTraffic", Other: "Traffic"},
		&i18n.Message{ID: "tgbot.buttons.catMaintenance", Other: "Maintenance"},
		&i18n.Message{ID: "tgbot.buttons.refresh", Other: "Refresh"},
		&i18n.Message{ID: "tgbot.messages.hostname", Other: "Server: {{.Hostname}}"},
	)
	tb.answerCommand(&telego.Message{Chat: telego.Chat{ID: 500, Type: "private"}, From: &telego.User{ID: 1}, Text: "/start"}, 500, true)
	screen, ok := tb.screens().get(500)
	if !ok || screen.msgID == 0 {
		t.Fatal("/start left the chat without a menu after Telegram rejected its avatar")
	}
	requests := api.snapshot()
	if len(requests) != 2 || !requests[1].Uploaded {
		t.Fatalf("requests = %+v, want one failed file-id send and one fallback upload", requests)
	}
	if requests[0].Caption != requests[1].Caption || !reflect.DeepEqual(requests[0].ReplyMarkup, requests[1].ReplyMarkup) || requests[1].ChatID != 500 {
		t.Fatalf("fallback changed the menu content or destination: %+v", requests)
	}
	if len(requests[1].ReplyMarkup.InlineKeyboard) != 3 || requests[1].ReplyMarkup.InlineKeyboard[0][0].CallbackData != cbCatServer {
		t.Fatalf("fallback did not carry the admin menu: %+v", requests[1].ReplyMarkup)
	}
	if screen.photo != "fallback-photo" {
		t.Errorf("tracked photo = %q, want reusable fallback-photo", screen.photo)
	}

	tb.resolveScreenArt()
	tb.renderScreen(500, tb.newScreen("main", "next", tb.homeRows()...))
	tb.renderScreen(501, tb.newScreen("status", "another screen", tb.homeRows()...))
	for _, request := range api.snapshot()[2:] {
		if request.Photo == "rejected-avatar" && !request.Uploaded {
			t.Errorf("the unchanged rejected avatar was reused: %+v", request)
		}
	}

	api.mu.Lock()
	api.avatar = "new-avatar"
	api.mu.Unlock()
	tb.resolveScreenArt()
	tb.renderScreen(500, tb.newScreen("main", "new avatar", tb.homeRows()...))
	last := api.snapshot()
	if last[len(last)-1].Photo != "new-avatar" || last[len(last)-1].Uploaded {
		t.Errorf("a changed avatar was not adopted: %+v", last[len(last)-1])
	}
}

func TestScreenMediaEditRecoversWithoutReplacingMessage(t *testing.T) {
	tb, api := newScreenMediaAPI(t)
	tb.screens().put(1, &screen{msgID: 9, kind: "main", hasPhoto: true})
	tb.renderScreen(1, tb.newScreen("main", "updated", tb.homeRows()...))
	requests := api.snapshot()
	if len(requests) != 2 || requests[0].Method != "editMessageMedia" || requests[1].Method != "editMessageMedia" || !requests[1].Uploaded {
		t.Fatalf("requests = %+v, want a bounded media-edit fallback on the existing message", requests)
	}
	tracked, _ := tb.screens().get(1)
	if tracked.msgID != 9 || tracked.photo != "fallback-photo" {
		t.Errorf("recovered screen = %+v, want message 9 with fallback-photo", tracked)
	}
}

func TestScreenMediaFailureKeepsThePreviousMenuAndCache(t *testing.T) {
	tb, api := newScreenMediaAPI(t)
	api.mu.Lock()
	api.failUpload, api.failEdit = true, true
	api.mu.Unlock()
	previous := &screen{msgID: 9, kind: "main", hasPhoto: true, photo: "rejected-avatar"}
	tb.screens().put(1, previous)
	tb.renderScreen(1, tb.newScreen("main", "update", tb.homeRows()...))
	tracked, _ := tb.screens().get(1)
	if tracked != previous || tracked.msgID != 9 {
		t.Errorf("failed recovery replaced the previous menu: %+v", tracked)
	}
	if id, _ := artFileID("main"); id != "rejected-avatar" {
		t.Errorf("failed upload poisoned the artwork cache: %q", id)
	}
	requests := api.snapshot()
	if len(requests) > 4 {
		t.Fatalf("requests = %+v, want at most one upload retry per edit/send operation", requests)
	}
	for _, request := range requests {
		if request.Method == "deleteMessage" {
			t.Fatal("the previous menu was deleted before a replacement succeeded")
		}
	}
}

func TestScreenMediaHealthyBotsKeepTheirExistingRendering(t *testing.T) {
	for _, avatar := range []string{"valid-avatar", ""} {
		t.Run(avatar, func(t *testing.T) {
			tb, api := newScreenMediaAPI(t)
			api.mu.Lock()
			api.avatar = avatar
			api.mu.Unlock()
			resetScreenArt()
			tb.resolveScreenArt()
			tb.renderScreen(1, tb.newScreen("main", "first", tb.homeRows()...))
			tb.renderScreen(1, tb.newScreen("main", "second", tb.homeRows()...))
			requests := api.snapshot()
			if len(requests) != 2 || requests[0].Method != "sendPhoto" || requests[1].Method != "editMessageMedia" {
				t.Fatalf("healthy bot changed its send/edit behavior: %+v", requests)
			}
			wantPhoto := avatar
			if avatar == "" {
				wantPhoto = "fallback-photo"
				if !requests[0].Uploaded {
					t.Error("an avatar-less bot did not upload its usual fallback tile")
				}
			} else if requests[0].Uploaded {
				t.Error("a valid avatar was unnecessarily replaced")
			}
			if requests[1].Photo != wantPhoto || requests[1].Uploaded || requests[1].Caption != "second" {
				t.Errorf("healthy menu was not edited normally: %+v", requests[1])
			}
		})
	}
}

func TestScreenMediaDoesNotRetryUncertainOrForbiddenFailures(t *testing.T) {
	for _, code := range []int{403, 429, 500, 0} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			tb, api := newScreenMediaAPI(t)
			api.mu.Lock()
			api.code, api.badResponse = code, code == 0
			api.mu.Unlock()
			tb.renderScreen(1, tb.newScreen("main", "menu", tb.homeRows()...))
			requests := api.snapshot()
			if len(requests) != 1 || requests[0].Uploaded {
				t.Errorf("non-recoverable failure retried or uploaded art: %+v", requests)
			}
			if _, ok := tb.screens().get(1); ok {
				t.Error("an unconfirmed send was recorded as a live screen")
			}
			if id, _ := artFileID("main"); id != "rejected-avatar" {
				t.Errorf("non-media failure invalidated the avatar: %q", id)
			}
		})
	}
}

func TestRejectedQRIsNeverReplacedByDecorativeArt(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprint(cached), func(t *testing.T) {
			tb, api := newScreenMediaAPI(t)
			api.mu.Lock()
			api.failUpload = true
			api.mu.Unlock()
			sc := tb.newScreen("qr", "QR", tb.backRow())
			sc.qrPicture = qrSheetPicture([]subscriptionQRCode{{kind: "sub", content: "https://example.invalid/sub"}})
			if cached {
				sc.photo = "rejected-avatar"
			}
			tb.renderScreen(1, sc)
			requests := api.snapshot()
			if len(requests) != 1 || requests[0].UploadName == "screen.png" {
				t.Errorf("QR rejection substituted decorative artwork: %+v", requests)
			}
			if id, _ := artFileID("main"); id != "rejected-avatar" {
				t.Errorf("QR failure invalidated unrelated menu artwork: %q", id)
			}
		})
	}
}

func TestScreenMediaRecoveryPreservesAConcurrentlyRefreshedAvatar(t *testing.T) {
	tb, api := newScreenMediaAPI(t)
	started, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	releaseUpload := func() { once.Do(func() { close(release) }) }
	t.Cleanup(releaseUpload)
	api.mu.Lock()
	api.uploadStarted, api.releaseUpload = started, release
	api.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		tb.renderScreen(1, tb.newScreen("main", "menu", tb.homeRows()...))
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("fallback upload never started")
	}
	api.mu.Lock()
	api.avatar = "new-avatar"
	api.mu.Unlock()
	tb.resolveScreenArt()
	releaseUpload()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("screen recovery never completed")
	}
	if id, _ := artFileID("main"); id != "new-avatar" {
		t.Errorf("late recovery discarded the newly refreshed avatar: %q", id)
	}
}
