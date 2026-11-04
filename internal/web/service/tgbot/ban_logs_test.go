package tgbot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSendBanLogsFile(t *testing.T) {
	const (
		previous = "3xipl-banned.prev.log"
		current  = "3xipl-banned.log"
	)
	for _, tt := range []struct {
		name        string
		files       map[string]string // An absent entry means the file does not exist.
		directory   string
		failUpload  string
		wantFiles   []string
		wantNotice  bool
		wantWarning bool
	}{
		{name: "both nonempty", files: map[string]string{previous: "yesterday\n", current: "today\n"}, wantFiles: []string{previous, current}},
		{name: "only current", files: map[string]string{current: "today\n"}, wantFiles: []string{current}},
		{name: "only previous", files: map[string]string{previous: "yesterday\n"}, wantFiles: []string{previous}},
		{name: "empty previous", files: map[string]string{previous: "", current: "today\n"}, wantFiles: []string{current}},
		{name: "empty current", files: map[string]string{previous: "yesterday\n", current: ""}, wantFiles: []string{previous}},
		{name: "both empty", files: map[string]string{previous: "", current: ""}, wantNotice: true},
		{name: "both missing", wantNotice: true},
		{name: "empty previous missing current", files: map[string]string{previous: ""}, wantNotice: true},
		{name: "missing previous empty current", files: map[string]string{current: ""}, wantNotice: true},
		{name: "unreadable previous", directory: previous, files: map[string]string{current: "today\n"}, wantFiles: []string{current}},
		{name: "first upload fails", files: map[string]string{previous: "yesterday\n", current: "today\n"}, failUpload: previous, wantFiles: []string{previous, current}, wantWarning: true},
		{name: "only previous upload fails", files: map[string]string{previous: "yesterday\n"}, failUpload: previous, wantFiles: []string{previous}, wantWarning: true},
		{name: "only current upload fails", files: map[string]string{current: "today\n"}, failUpload: current, wantFiles: []string{current}, wantWarning: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XUI_LOG_FOLDER", t.TempDir())
			localizeWithRealBundle(t)
			for name, contents := range tt.files {
				if err := os.WriteFile(filepath.Join(os.Getenv("XUI_LOG_FOLDER"), name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// A directory is deterministically unreadable as a document, even as root.
			if tt.directory != "" {
				if err := os.Mkdir(filepath.Join(os.Getenv("XUI_LOG_FOLDER"), tt.directory), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			tgBotMutex.Lock()
			originalTopics := adminTopics
			adminTopics = map[int64]int{-1001: 42}
			tgBotMutex.Unlock()
			t.Cleanup(func() {
				tgBotMutex.Lock()
				adminTopics = originalTopics
				tgBotMutex.Unlock()
			})

			var mu sync.Mutex
			var filenames []string
			var notices []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var params struct {
					ChatID              int64           `json:"chat_id"`
					MessageThreadID     int             `json:"message_thread_id"`
					DisableNotification bool            `json:"disable_notification"`
					ReplyMarkup         json.RawMessage `json:"reply_markup"`
				}
				method := filepath.Base(r.URL.Path)
				filename := ""
				switch method {
				case "sendDocument":
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						http.Error(w, "invalid multipart", http.StatusBadRequest)
						return
					}
					defer r.MultipartForm.RemoveAll()
					if err := json.Unmarshal([]byte(r.FormValue("chat_id")), &params.ChatID); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal([]byte(r.FormValue("message_thread_id")), &params.MessageThreadID); err != nil {
						t.Error(err)
					}
					params.DisableNotification = r.FormValue("disable_notification") == "true"
					params.ReplyMarkup = json.RawMessage(r.FormValue("reply_markup"))
					file, header, err := r.FormFile("document")
					if err != nil {
						t.Error(err)
						http.Error(w, "missing document", http.StatusBadRequest)
						return
					}
					defer file.Close()
					filename = header.Filename
					body, err := io.ReadAll(file)
					if err != nil {
						t.Error(err)
					}
					if len(body) == 0 || string(body) != tt.files[filename] {
						t.Errorf("document %q contents = %q, want nonempty %q", filename, body, tt.files[filename])
					}
					mu.Lock()
					filenames = append(filenames, filename)
					mu.Unlock()
				case "sendMessage":
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal(body, &params); err != nil {
						t.Error(err)
					}
					var message struct {
						Text string `json:"text"`
					}
					if err := json.Unmarshal(body, &message); err != nil {
						t.Error(err)
					}
					mu.Lock()
					notices = append(notices, message.Text)
					mu.Unlock()
				default:
					t.Errorf("unexpected Bot API method %q", method)
				}
				if params.ChatID != -1001 || params.MessageThreadID != 42 || params.DisableNotification {
					t.Errorf("unexpected delivery parameters: %+v", params)
				}
				var markup struct {
					Rows [][]struct {
						CallbackData string `json:"callback_data"`
					} `json:"inline_keyboard"`
				}
				if err := json.Unmarshal(params.ReplyMarkup, &markup); err != nil {
					t.Error(err)
				}
				if len(markup.Rows) != 1 || len(markup.Rows[0]) != 1 || markup.Rows[0][0].CallbackData != "hide" {
					t.Errorf("missing hide button: %s", params.ReplyMarkup)
				}
				if filename != "" && filename == tt.failUpload {
					_, _ = io.WriteString(w, `{"ok":false,"error_code":400,"description":"controlled upload failure"}`)
					return
				}
				_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":77,"date":0,"chat":{"id":-1001,"type":"supergroup"}}}`)
			}))
			t.Cleanup(srv.Close)
			swapTestBot(t, srv.URL)

			tb := &Tgbot{}
			tb.sendBanLogsFile(-1001)

			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(filenames, tt.wantFiles) {
				t.Errorf("document requests = %v, want %v", filenames, tt.wantFiles)
			}
			var wantNotices []string
			if tt.wantNotice {
				text := tb.I18nBot("tgbot.noResult")
				if text == "" || text == "tgbot.noResult" {
					t.Fatal("noResult translation was not loaded")
				}
				wantNotices = []string{text}
			}
			if !reflect.DeepEqual(notices, wantNotices) {
				t.Errorf("notices = %q, want %q", notices, wantNotices)
			}
			if tt.wantWarning {
				logs := logger.GetLogs(10, "warning")
				path := xray.GetIPLimitBannedLogPath()
				if tt.failUpload == previous {
					path = xray.GetIPLimitBannedPrevLogPath()
				}
				if !slices.ContainsFunc(logs, func(entry string) bool {
					return strings.Contains(entry, path) && strings.Contains(entry, "controlled upload failure")
				}) {
					t.Errorf("upload failure not logged: %v", logs)
				}
			}
			for name, contents := range tt.files {
				path := filepath.Join(os.Getenv("XUI_LOG_FOLDER"), name)
				body, err := os.ReadFile(path)
				if err != nil || string(body) != contents {
					t.Errorf("log %q changed: contents = %q, error = %v", name, body, err)
				}
				// Windows refuses to remove a file whose upload handle is still open.
				if err := os.Remove(path); err != nil {
					t.Errorf("log file still open: %v", err)
				}
			}
		})
	}
}

func TestSendBanLogsFileWithoutBot(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())
	path := xray.GetIPLimitBannedLogPath()
	if err := os.WriteFile(path, []byte("today\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalBot := liveBot()
	setTestBot(nil)
	t.Cleanup(func() { setTestBot(originalBot) })
	(&Tgbot{}).sendBanLogsFile(-1001)
	if err := os.Remove(path); err != nil {
		t.Fatalf("log file still open: %v", err)
	}
}
