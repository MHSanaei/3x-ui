package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type botLanguage struct{}

func (botLanguage) GetTgLang() (string, error) { return "ru-RU", nil }

func TestMain(main *testing.M) {
	if err := locale.InitLocalizer(os.DirFS("../.."), botLanguage{}); err != nil {
		panic(err)
	}
	os.Exit(main.Run())
}

func setupSettings(t *testing.T, values map[string]string) service.SettingService {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	for key, value := range values {
		if err := database.GetDB().Where("key = ?", key).Delete(&model.Setting{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.GetDB().Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return service.SettingService{}
}

func localReceiver(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	receiver := httptest.NewTLSServer(handler)
	t.Cleanup(receiver.Close)
	client := receiver.Client()
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, receiver.Listener.Addr().String())
	}
	return client
}

func webhookSettings(t *testing.T) service.SettingService {
	t.Helper()
	return setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "webhook", "wechatWebhookURL": "https://receiver.example/notify",
		"wechatToken": "test-secret", "wechatLang": "en-US", "wechatReportEnable": "true",
	})
}

func TestWebhookRetriesKeepOneEnvelopeIdentityAndBearerCredential(t *testing.T) {
	settings := webhookSettings(t)
	var bodies [][]byte
	svc := NewService(settings)
	svc.retryWait = func(context.Context, time.Duration) error { return nil }
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/notify" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("unexpected request: %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	if err := svc.SendTest(); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || string(bodies[0]) != string(bodies[1]) {
		t.Fatalf("retry changed envelope or was unbounded: %q", bodies)
	}
	var payload Envelope
	if err := json.Unmarshal(bodies[0], &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != 1 || payload.Type != "test" || payload.Event != "test" || len(payload.ID) != 36 || payload.Hostname == "" || !strings.Contains(payload.Text, "3x-ui") {
		t.Fatalf("invalid webhook contract: %+v", payload)
	}
	if _, err := time.Parse(time.RFC3339, payload.OccurredAt); err != nil {
		t.Fatalf("invalid event timestamp: %q", payload.OccurredAt)
	}
}

func TestWebhookCredentialRotationKeepsDestinationAndTokenTogether(t *testing.T) {
	settings := webhookSettings(t)
	db := database.GetDB()
	var rotated atomic.Bool
	const callback = "test:rotate_wechat_destination"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if !strings.Contains(fmt.Sprint(tx.Statement.Vars), "wechatWebhookURL") || !rotated.CompareAndSwap(false, true) {
			return
		}
		if err := db.Exec("UPDATE settings SET value = CASE key WHEN 'wechatWebhookURL' THEN ? WHEN 'wechatToken' THEN ? ELSE value END WHERE key IN (?, ?)",
			"https://new-receiver.example/notify", "new-secret", "wechatWebhookURL", "wechatToken").Error; err != nil {
			t.Error(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	svc := NewService(settings)
	var destination, credential string
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		destination, credential = r.Host, r.Header.Get("Authorization")
	})
	if err := svc.SendTest(); err != nil {
		t.Fatal(err)
	}
	if !rotated.Load() {
		t.Fatal("test did not rotate the saved destination")
	}
	if destination != "receiver.example" || credential != "Bearer test-secret" {
		t.Fatalf("credential rotation mixed snapshots: destination=%q credential=%q", destination, credential)
	}
}

func TestWecomSplitsOnlyAtUTF8BoundariesAndConfirmsErrcode(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "wecom", "wechatWebhookURL": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
	})
	var pieces []string
	svc := NewService(settings)
	svc.retryWait = func(context.Context, time.Duration) error { return nil }
	svc.sendInterval = 0
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.MsgType != "text" || len(payload.Text.Content) > 2048 || !utf8.ValidString(payload.Text.Content) {
			t.Errorf("invalid WeCom segment: %q", payload.Text.Content)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("WeCom must not receive custom webhook bearer credentials")
		}
		pieces = append(pieces, payload.Text.Content)
		_, _ = io.WriteString(w, `{"errcode":0}`)
	})
	text := strings.Repeat("汉🙂", 600)
	if err := svc.send(context.Background(), "event", "node.down", text, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 3 || !strings.HasSuffix(strings.Join(pieces, ""), "\n"+text) || !strings.HasPrefix(pieces[0], "3x-ui · ") {
		t.Fatalf("segments lost text: count=%d", len(pieces))
	}
}

func TestWecomRejectedResponseIsNotReportedAsSuccessOrRetried(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "wecom", "wechatWebhookURL": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=private-key",
	})
	var calls int
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = io.WriteString(w, `{"errcode":45009,"errmsg":"private-key rejected"}`)
	})
	err := svc.SendTest()
	if !errors.Is(err, ErrDelivery) || calls != 1 || strings.Contains(err.Error(), "private-key") {
		t.Fatalf("provider rejection or privacy failure: calls=%d err=%v", calls, err)
	}
}

func TestInvalidDestinationNeverReachesTheNetwork(t *testing.T) {
	cases := []struct{ mode, url string }{
		{"webhook", "http://receiver.example/hook"},
		{"webhook", "https://127.0.0.1/hook"},
		{"webhook", "https://169.254.169.254/latest"},
		{"webhook", "https://[::1]/hook"},
		{"webhook", "https://user:pass@receiver.example/hook"},
		{"webhook", "https://receiver.example/hook#fragment"},
		{"wecom", "https://receiver.example/cgi-bin/webhook/send?key=secret"},
		{"wecom", "https://qyapi.weixin.qq.com/other?key=secret"},
		{"wecom", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=secret&other=value"},
	}
	for _, tc := range cases {
		t.Run(tc.mode+tc.url, func(t *testing.T) {
			settings := setupSettings(t, map[string]string{"wechatEnable": "true", "wechatMode": tc.mode, "wechatWebhookURL": tc.url})
			var called atomic.Bool
			svc := NewService(settings)
			svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { called.Store(true) })
			if err := svc.SendTest(); !errors.Is(err, ErrConfiguration) || called.Load() {
				t.Fatalf("invalid destination reached network: called=%v err=%v", called.Load(), err)
			}
		})
	}
}

func TestRedirectCannotForwardBearerCredentials(t *testing.T) {
	settings := webhookSettings(t)
	var calls int
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "https://other.example/stolen")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	if err := svc.SendTest(); !errors.Is(err, ErrDelivery) || calls != 1 {
		t.Fatalf("redirect followed or accepted: calls=%d err=%v", calls, err)
	}
}

func TestDNSResolutionToLoopbackIsBlockedAtDial(t *testing.T) {
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			buffer := make([]byte, 1024)
			count, peer, err := listener.ReadFrom(buffer)
			if err != nil {
				return
			}
			query := buffer[:count]
			if len(query) < 17 {
				continue
			}
			end := 12
			for end < len(query) && query[end] != 0 {
				end += int(query[end]) + 1
			}
			if end+5 > len(query) {
				continue
			}
			end += 5
			answer := append([]byte(nil), query[:end]...)
			answer[2], answer[3], answer[6], answer[7] = 0x81, 0x80, 0, 0
			answer[10], answer[11] = 0, 0
			if query[end-4] == 0 && query[end-3] == 1 {
				answer[7] = 1
				answer = append(answer, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 0, 0, 4, 127, 0, 0, 1)
			}
			_, _ = listener.WriteTo(answer, peer)
		}
	}()
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", listener.LocalAddr().String())
	}}
	t.Cleanup(func() { net.DefaultResolver = previous })
	settings := setupSettings(t, map[string]string{"wechatEnable": "true", "wechatMode": "webhook", "wechatWebhookURL": "https://receiver-dns.example/hook"})
	svc := NewService(settings)
	svc.retryWait = func(context.Context, time.Duration) error { return nil }
	dial := svc.httpClient.Transport.(*http.Transport).DialContext
	if _, err := dial(context.Background(), "tcp", "receiver-dns.example:443"); !errors.Is(err, netsafe.ErrPrivateAddressBlocked) {
		t.Fatalf("DNS rebinding bypassed guarded dial: %v", err)
	}
	if err := svc.SendTest(); !errors.Is(err, ErrDelivery) || strings.Contains(err.Error(), "receiver-dns") {
		t.Fatalf("guard or redaction failed: %v", err)
	}
}

func TestWecomPacingHonorsCancellationAndDoesNotSendAnExtraMessage(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "wecom", "wechatWebhookURL": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
	})
	var requests int
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = io.WriteString(w, `{"errcode":0}`)
	})
	if err := svc.SendTest(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	err := svc.send(ctx, "event", "node.down", "second message", time.Now())
	if !errors.Is(err, ErrDelivery) || requests != 1 {
		t.Fatalf("pacing ignored cancellation: requests=%d err=%v", requests, err)
	}
}

func TestWecomQueuedRequestCanCancelWhileAnotherReportOwnsTheSendGate(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "wecom", "wechatWebhookURL": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
	})
	var requests atomic.Int32
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1) })
	svc.sendGate <- struct{}{}
	defer func() { <-svc.sendGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := make(chan error, 1)
	go func() { result <- svc.send(ctx, "event", "node.down", "queued event", time.Now()) }()
	select {
	case err := <-result:
		if !errors.Is(err, ErrDelivery) || requests.Load() != 0 {
			t.Fatalf("cancelled request reached receiver: count=%d err=%v", requests.Load(), err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled event remained blocked behind the report")
	}
}

func TestWebhookFailuresStopAfterOneRetryAndDoNotDiscloseResponseBody(t *testing.T) {
	settings := webhookSettings(t)
	var requests int
	svc := NewService(settings)
	svc.retryWait = func(context.Context, time.Duration) error { return nil }
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "secret-response-body")
	})
	err := svc.SendTest()
	if !errors.Is(err, ErrDelivery) || requests != 2 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unbounded retries or response disclosure: requests=%d err=%v", requests, err)
	}
}

func TestOversizedWecomMessageFailsBeforePartialDelivery(t *testing.T) {
	settings := setupSettings(t, map[string]string{
		"wechatEnable": "true", "wechatMode": "wecom", "wechatWebhookURL": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
	})
	var requests int
	svc := NewService(settings)
	svc.httpClient = localReceiver(t, func(w http.ResponseWriter, r *http.Request) { requests++ })
	err := svc.send(context.Background(), "report", "report", strings.Repeat("字", 7000), time.Now())
	if !errors.Is(err, ErrTooLong) || requests != 0 {
		t.Fatalf("oversize caused partial notification: requests=%d err=%v", requests, err)
	}
}
