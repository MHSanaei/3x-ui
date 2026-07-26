package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// recordingServer captures every request it receives so tests can assert on
// method, headers, and body without caring about response timing races.
type recordingServer struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []recordedRequest
	hits int32
}

type recordedRequest struct {
	Method  string
	Headers http.Header
	Body    []byte
}

func newRecordingServer(t *testing.T, status int) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&rs.hits, 1)
		body, _ := io.ReadAll(r.Body)
		rs.mu.Lock()
		rs.reqs = append(rs.reqs, recordedRequest{
			Method:  r.Method,
			Headers: r.Header.Clone(),
			Body:    body,
		})
		rs.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *recordingServer) requests() []recordedRequest {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]recordedRequest(nil), rs.reqs...)
}

func (rs *recordingServer) hitCount() int32 {
	return atomic.LoadInt32(&rs.hits)
}

func newTestSettingService(t *testing.T) service.SettingService {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return service.SettingService{}
}

func mustSet(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setting: %v", err)
	}
}

func TestSend_PostsJSONPayload(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{
		Type:   eventbus.EventXrayCrash,
		Source: "core",
		Data:   "boom",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	reqs := srv.requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if ct := req.Headers.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if ev := req.Headers.Get(eventHeader); ev != string(eventbus.EventXrayCrash) {
		t.Errorf("%s = %q, want %q", eventHeader, ev, eventbus.EventXrayCrash)
	}

	var payload Payload
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		t.Fatalf("body does not decode as Payload: %v", err)
	}
	if payload.Event != eventbus.EventXrayCrash {
		t.Errorf("payload.Event = %q, want %q", payload.Event, eventbus.EventXrayCrash)
	}
	if payload.Source != "core" {
		t.Errorf("payload.Source = %q, want %q", payload.Source, "core")
	}
	if payload.Timestamp == 0 {
		t.Error("payload.Timestamp should be non-zero")
	}
}

func TestSend_SignsWhenSecretConfigured(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookSecret("top-secret"))

	w := NewWebhookService(settingService)
	if err := w.Send(eventbus.Event{Type: eventbus.EventNodeUp, Source: "node-1"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	reqs := srv.requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	gotSig := reqs[0].Headers.Get(signatureHeader)
	if gotSig == "" {
		t.Fatal("expected a signature header when a secret is configured")
	}

	mac := hmac.New(sha256.New, []byte("top-secret"))
	mac.Write(reqs[0].Body)
	wantSig := hex.EncodeToString(mac.Sum(nil))
	if gotSig != wantSig {
		t.Errorf("signature = %q, want %q", gotSig, wantSig)
	}
}

func TestSend_NoSignatureWithoutSecret(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	// webhookSecret left empty (default)

	w := NewWebhookService(settingService)
	if err := w.Send(eventbus.Event{Type: eventbus.EventNodeUp, Source: "node-1"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	reqs := srv.requests()
	if got := reqs[0].Headers.Get(signatureHeader); got != "" {
		t.Errorf("expected no signature header, got %q", got)
	}
}

func TestSend_MissingURLReturnsError(t *testing.T) {
	settingService := newTestSettingService(t)
	// webhookURL left empty (default)

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{Type: eventbus.EventXrayCrash})
	if err == nil {
		t.Fatal("expected an error when webhook URL is not configured")
	}
}

func TestSend_NonSuccessStatusIsError(t *testing.T) {
	srv := newRecordingServer(t, http.StatusInternalServerError)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{Type: eventbus.EventXrayCrash})
	if err == nil {
		t.Fatal("expected an error on a 500 response")
	}
}

func TestSend_CPUHighBelowThreshold_NoRequest(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookCpu(80))

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{
		Type: eventbus.EventCPUHigh,
		Data: &eventbus.SystemMetricData{Percent: 50, Threshold: 80},
	})
	if err != nil {
		t.Fatalf("Send should not error when below threshold, got: %v", err)
	}
	if srv.hitCount() != 0 {
		t.Errorf("expected no request below threshold, got %d hits", srv.hitCount())
	}
}

func TestSend_CPUHighAboveThreshold_Fires(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookCpu(80))

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{
		Type: eventbus.EventCPUHigh,
		Data: &eventbus.SystemMetricData{Percent: 95, Threshold: 80},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if srv.hitCount() != 1 {
		t.Errorf("expected exactly 1 request above threshold, got %d", srv.hitCount())
	}
}

func TestSend_CPUHighThresholdDisabled_NoRequest(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookCpu(0)) // 0 = disabled, mirrors email's smtpCpu convention

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{
		Type: eventbus.EventCPUHigh,
		Data: &eventbus.SystemMetricData{Percent: 99, Threshold: 0},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if srv.hitCount() != 0 {
		t.Errorf("expected no request when threshold is 0 (disabled), got %d hits", srv.hitCount())
	}
}

func TestSend_MemoryHighAboveThreshold_Fires(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))
	mustSet(t, settingService.SetWebhookMemory(70))

	w := NewWebhookService(settingService)
	err := w.Send(eventbus.Event{
		Type: eventbus.EventMemoryHigh,
		Data: &eventbus.SystemMetricData{Percent: 85, Threshold: 70},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if srv.hitCount() != 1 {
		t.Errorf("expected exactly 1 request above threshold, got %d", srv.hitCount())
	}
}

func TestTestConnection_MissingURL(t *testing.T) {
	settingService := newTestSettingService(t)
	w := NewWebhookService(settingService)

	got := w.TestConnection()
	want := TestResult{Success: false, Stage: "config", Message: "webhookUrlNotConfigured"}
	if got != want {
		t.Errorf("TestConnection() = %+v, want %+v", got, want)
	}
}

func TestTestConnection_Success(t *testing.T) {
	srv := newRecordingServer(t, http.StatusOK)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))

	w := NewWebhookService(settingService)
	got := w.TestConnection()
	if !got.Success {
		t.Fatalf("TestConnection() = %+v, want success", got)
	}
	if got.Stage != "send" {
		t.Errorf("Stage = %q, want %q", got.Stage, "send")
	}

	reqs := srv.requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	var payload Payload
	if err := json.Unmarshal(reqs[0].Body, &payload); err != nil {
		t.Fatalf("body does not decode: %v", err)
	}
	if payload.Event != "test" {
		t.Errorf("payload.Event = %q, want %q", payload.Event, "test")
	}
}

func TestTestConnection_ServerError(t *testing.T) {
	srv := newRecordingServer(t, http.StatusBadGateway)
	settingService := newTestSettingService(t)
	mustSet(t, settingService.SetWebhookURL(srv.URL))

	w := NewWebhookService(settingService)
	got := w.TestConnection()
	if got.Success {
		t.Fatal("expected failure on a 502 response")
	}
	if got.Stage != "send" {
		t.Errorf("Stage = %q, want %q", got.Stage, "send")
	}
}
