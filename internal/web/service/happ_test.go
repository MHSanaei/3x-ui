package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func initHappTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	t.Setenv("XUI_BIN_FOLDER", dbDir)
	if err := os.WriteFile(filepath.Join(dbDir, "config.json"), []byte(`{"log":{}}`), 0o600); err != nil {
		t.Fatalf("write Xray config: %v", err)
	}
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func seedHappClient(t *testing.T, subID string) *model.ClientRecord {
	t.Helper()
	client := &model.ClientRecord{Email: "happ@test", SubID: subID, Enable: true}
	if err := database.GetDB().Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return client
}

func configureHappSubscription(t *testing.T, enabled bool, subURI string) {
	t.Helper()
	settings := &SettingService{}
	for key, value := range map[string]string{
		"subEnable": "false",
		"subURI":    subURI,
		"subPath":   "/sub/",
		"subPort":   "80",
		"subDomain": "",
	} {
		if key == "subEnable" && enabled {
			value = "true"
		}
		if err := settings.saveSetting(key, value); err != nil {
			t.Fatalf("save %s: %v", key, err)
		}
	}
}

func configureHappLinkGate(t *testing.T, enabled bool) {
	t.Helper()
	if err := (&SettingService{}).saveSetting("happLinkEnable", strconv.FormatBool(enabled)); err != nil {
		t.Fatalf("save happLinkEnable: %v", err)
	}
}

type happRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f happRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newHappTestService(server *httptest.Server, timeout time.Duration) *HappService {
	return &HappService{
		clientService:  &ClientService{},
		settingService: &SettingService{},
		endpoint:       server.URL,
		newHTTPClient: func(time.Duration) *http.Client {
			return &http.Client{Timeout: timeout}
		},
	}
}

func TestHappGenerateRejectsDisabledGateBeforeProviderSetup(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*testing.T)
	}{
		{name: "missing setting", configure: func(*testing.T) {}},
		{name: "explicit false", configure: func(t *testing.T) { configureHappLinkGate(t, false) }},
		{name: "invalid setting", configure: func(t *testing.T) {
			if err := (&SettingService{}).saveSetting("happLinkEnable", "not-a-bool"); err != nil {
				t.Fatalf("save invalid happLinkEnable: %v", err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, "current-sub-id")
			configureHappSubscription(t, true, "https://sub.example/sub/")
			tt.configure(t)

			var providerCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				providerCalls.Add(1)
				_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
			}))
			defer server.Close()
			svc := newHappTestService(server, time.Second)
			baseFactory := svc.newHTTPClient
			var clientSetups atomic.Int32
			svc.newHTTPClient = func(timeout time.Duration) *http.Client {
				clientSetups.Add(1)
				return baseFactory(timeout)
			}

			result, err := svc.Generate(context.Background(), client.Id, "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) {
				t.Fatalf("Generate error = %v, want ErrHappLinkUnavailable", err)
			}
			if result != (HappLinkResult{}) {
				t.Fatalf("Generate result = %#v, want empty", result)
			}
			if got := clientSetups.Load(); got != 0 {
				t.Fatalf("HTTP client setups = %d, want 0", got)
			}
			if got := providerCalls.Load(); got != 0 {
				t.Fatalf("provider calls = %d, want 0", got)
			}
		})
	}
}

func TestHappGenerateDiscardsResultWhenGateDisabledInFlight(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "current-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)

	entered := make(chan struct{})
	release := make(chan struct{})
	var providerCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		providerCalls.Add(1)
		close(entered)
		<-release
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()

	type generateOutcome struct {
		result HappLinkResult
		err    error
	}
	outcome := make(chan generateOutcome, 1)
	svc := newHappTestService(server, time.Second)
	go func() {
		result, err := svc.Generate(context.Background(), client.Id, "panel.example")
		outcome <- generateOutcome{result: result, err: err}
	}()

	<-entered
	configureHappLinkGate(t, false)
	close(release)
	got := <-outcome
	if !errors.Is(got.err, ErrHappLinkUnavailable) {
		t.Fatalf("Generate error = %v, want ErrHappLinkUnavailable", got.err)
	}
	if got.result != (HappLinkResult{}) {
		t.Fatalf("Generate result = %#v, want empty", got.result)
	}
	if calls := providerCalls.Load(); calls != 1 {
		t.Fatalf("provider calls = %d, want 1", calls)
	}
}

func TestHappGenerateSendsExplicitCurrentSource(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "current-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q", got)
		}
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.URL != "https://sub.example/sub/current-sub-id" {
			t.Fatalf("url = %q", body.URL)
		}
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()

	result, err := newHappTestService(server, time.Second).Generate(context.Background(), client.Id, "panel.example")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.EncryptedLink != "happ://crypt5/example" {
		t.Fatalf("EncryptedLink = %q", result.EncryptedLink)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestHappGenerateUsesDefaultSubscriptionSource(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "fallback-sub-id")
	configureHappSubscription(t, true, "")
	configureHappLinkGate(t, true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.URL != "http://panel.example/sub/fallback-sub-id" {
			t.Fatalf("url = %q", body.URL)
		}
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()

	if _, err := newHappTestService(server, time.Second).Generate(context.Background(), client.Id, "panel.example"); err != nil {
		t.Fatalf("Generate: %v", err)
	}
}

func TestHappGenerateUsesCurrentSubIDForEachAction(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "before")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)

	var gotURLs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotURLs = append(gotURLs, body.URL)
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()
	svc := newHappTestService(server, time.Second)
	if _, err := svc.Generate(context.Background(), client.Id, "panel.example"); err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	if err := database.GetDB().Model(client).Update("sub_id", "after").Error; err != nil {
		t.Fatalf("update SubID: %v", err)
	}
	if _, err := svc.Generate(context.Background(), client.Id, "panel.example"); err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if got := strings.Join(gotURLs, ","); got != "https://sub.example/sub/before,https://sub.example/sub/after" {
		t.Fatalf("sent URLs = %q", got)
	}
}

func TestHappGenerateDiscardsResultWhenSourceChangesInFlight(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "before")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)
	entered := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := newHappTestService(server, time.Second).Generate(context.Background(), client.Id, "panel.example")
		errCh <- err
	}()
	<-entered
	if err := database.GetDB().Model(client).Update("sub_id", "after").Error; err != nil {
		t.Fatalf("update SubID: %v", err)
	}
	close(release)
	if err := <-errCh; !errors.Is(err, ErrHappLinkUnavailable) {
		t.Fatalf("Generate error = %v, want ErrHappLinkUnavailable", err)
	}
}

func TestHappGenerateSkipsUnavailableSources(t *testing.T) {
	tests := []struct {
		name     string
		clientID func(*model.ClientRecord) int
		enabled  bool
		subID    string
	}{
		{name: "disabled subscription", clientID: func(c *model.ClientRecord) int { return c.Id }, enabled: false, subID: "current-sub-id"},
		{name: "missing client", clientID: func(c *model.ClientRecord) int { return c.Id + 1 }, enabled: true, subID: "current-sub-id"},
		{name: "empty subscription ID", clientID: func(c *model.ClientRecord) int { return c.Id }, enabled: true, subID: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, tt.subID)
			configureHappSubscription(t, tt.enabled, "https://sub.example/sub/")
			configureHappLinkGate(t, true)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
			defer server.Close()
			_, err := newHappTestService(server, time.Second).Generate(context.Background(), tt.clientID(client), "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) {
				t.Fatalf("Generate error = %v, want ErrHappLinkUnavailable", err)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("provider calls = %d, want 0", got)
			}
		})
	}
}

func TestHappGenerateRejectsInvalidProviderResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "provider error", statusCode: http.StatusOK, body: `{"error":"nope"}`},
		{name: "both fields", statusCode: http.StatusOK, body: `{"encrypted_link":"happ://crypt5/example","error":"nope"}`},
		{name: "duplicate encrypted link", statusCode: http.StatusOK, body: `{"encrypted_link":"happ://crypt5/first","encrypted_link":"happ://crypt5/second"}`},
		{name: "duplicate error", statusCode: http.StatusOK, body: `{"error":"first","error":"second"}`},
		{name: "missing fields", statusCode: http.StatusOK, body: `{}`},
		{name: "null field", statusCode: http.StatusOK, body: `{"encrypted_link":null}`},
		{name: "non-string field", statusCode: http.StatusOK, body: `{"encrypted_link":7}`},
		{name: "null error", statusCode: http.StatusOK, body: `{"error":null}`},
		{name: "non-string error", statusCode: http.StatusOK, body: `{"error":7}`},
		{name: "empty payload", statusCode: http.StatusOK, body: `{"encrypted_link":""}`},
		{name: "wrong scheme", statusCode: http.StatusOK, body: `{"encrypted_link":"https://provider.example/link"}`},
		{name: "whitespace", statusCode: http.StatusOK, body: `{"encrypted_link":"happ://crypt5/has space"}`},
		{name: "control character", statusCode: http.StatusOK, body: "{\"encrypted_link\":\"happ://crypt5/example\\n\"}"},
		{name: "malformed JSON", statusCode: http.StatusOK, body: `{"encrypted_link":`},
		{name: "trailing JSON", statusCode: http.StatusOK, body: `{"encrypted_link":"happ://crypt5/example"} {}`},
		{name: "non-success response", statusCode: http.StatusBadGateway, body: `{"encrypted_link":"happ://crypt5/example"}`},
		{name: "oversized response", statusCode: http.StatusOK, body: `{"encrypted_link":"happ://crypt5/` + strings.Repeat("a", (64<<10)+1) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, "current-sub-id")
			configureHappSubscription(t, true, "https://sub.example/sub/")
			configureHappLinkGate(t, true)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			_, err := newHappTestService(server, time.Second).Generate(context.Background(), client.Id, "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) {
				t.Fatalf("Generate error = %v, want ErrHappLinkUnavailable", err)
			}
		})
	}
}

func TestParseHappResponseRejectsDuplicateSupportedFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "encrypted link", body: `{"encrypted_link":"happ://crypt5/first","encrypted_link":"happ://crypt5/second"}`},
		{name: "error", body: `{"error":"first","error":"second"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, reason := parseHappResponse([]byte(tt.body)); reason != "response_shape" {
				t.Fatalf("parseHappResponse reason = %q, want response_shape", reason)
			}
		})
	}
}

func TestHappGenerateRejectsRedirectAndTimeout(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "current-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)

	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var originCalls atomic.Int32
			var targetCalls atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				targetCalls.Add(1)
				_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/followed"}`)
			}))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				originCalls.Add(1)
				http.Redirect(w, r, target.URL, status)
			}))
			defer origin.Close()

			if _, err := newHappTestService(origin, time.Second).Generate(context.Background(), client.Id, "panel.example"); !errors.Is(err, ErrHappLinkUnavailable) {
				t.Fatalf("redirect error = %v, want ErrHappLinkUnavailable", err)
			}
			if got := originCalls.Load(); got != 1 {
				t.Fatalf("origin calls = %d, want 1", got)
			}
			if got := targetCalls.Load(); got != 0 {
				t.Fatalf("target calls = %d, want 0", got)
			}
		})
	}

	timeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer timeout.Close()
	if _, err := newHappTestService(timeout, time.Millisecond).Generate(context.Background(), client.Id, "panel.example"); !errors.Is(err, ErrHappLinkUnavailable) {
		t.Fatalf("timeout error = %v, want ErrHappLinkUnavailable", err)
	}
}

func TestHappGenerateUsesProductionTimeout(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "current-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()

	var gotTimeout time.Duration
	svc := &HappService{
		clientService:  &ClientService{},
		settingService: &SettingService{},
		endpoint:       server.URL,
		newHTTPClient: func(timeout time.Duration) *http.Client {
			gotTimeout = timeout
			return &http.Client{Timeout: time.Second}
		},
	}
	if _, err := svc.Generate(context.Background(), client.Id, "panel.example"); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotTimeout != 10*time.Second {
		t.Fatalf("HTTP client timeout = %s, want 10s", gotTimeout)
	}
}

func TestHappGenerateLogsSanitizedFailure(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "current-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/seeded-source-and-secret/")
	configureHappLinkGate(t, true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"error":"https://secret.example/?token=leak"}`)
	}))
	defer server.Close()

	if _, err := newHappTestService(server, time.Second).Generate(context.Background(), client.Id, "panel.example"); !errors.Is(err, ErrHappLinkUnavailable) {
		t.Fatalf("Generate error = %v, want ErrHappLinkUnavailable", err)
	}
	logs := logger.GetLogs(1, "WARNING")
	if len(logs) != 1 {
		t.Fatalf("warning logs = %d, want 1", len(logs))
	}
	logLine := logs[0]
	for _, want := range []string{"component=happ_link", "client_id=" + strconv.Itoa(client.Id), "reason=http_status", "status=502", "elapsed_ms=", "correlation_id="} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log %q does not contain %q", logLine, want)
		}
	}
	for _, secret := range []string{"seeded-source-and-secret", "current-sub-id", "happ://", "secret.example", "token=leak"} {
		if strings.Contains(logLine, secret) {
			t.Fatalf("log leaked %q: %q", secret, logLine)
		}
	}
}

func TestHappGenerateDoesNotExposeTransportErrors(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "transport-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/transport-source/")
	configureHappLinkGate(t, true)

	const reflectedError = "request https://crypto.happ.su/?source=https://sub.example/sub/transport-source/transport-sub-id token=secret cookie=session authorization=Bearer-secret happ://crypt5/leak"
	var transportCalls atomic.Int32
	svc := &HappService{
		clientService:  &ClientService{},
		settingService: &SettingService{},
		endpoint:       happCryptoEndpoint,
		newHTTPClient: func(time.Duration) *http.Client {
			return &http.Client{Transport: happRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				transportCalls.Add(1)
				return nil, errors.New(reflectedError)
			})}
		},
	}

	result, err := svc.Generate(context.Background(), client.Id, "panel.example")
	if !errors.Is(err, ErrHappLinkUnavailable) || err.Error() != "happ link unavailable" {
		t.Fatalf("Generate error = %v, want generic ErrHappLinkUnavailable", err)
	}
	if result != (HappLinkResult{}) {
		t.Fatalf("Generate result = %#v, want empty", result)
	}
	if got := transportCalls.Load(); got != 1 {
		t.Fatalf("transport calls = %d, want 1", got)
	}
	logs := logger.GetLogs(1, "WARNING")
	if len(logs) != 1 || !strings.Contains(logs[0], "reason=transport") {
		t.Fatalf("transport warning logs = %#v, want one sanitized transport failure", logs)
	}
	for _, secret := range []string{reflectedError, "transport-source", "transport-sub-id", "token=secret", "cookie=session", "authorization=Bearer-secret", "happ://"} {
		if strings.Contains(logs[0], secret) || strings.Contains(err.Error(), secret) {
			t.Fatalf("transport failure leaked %q: error=%q log=%q", secret, err, logs[0])
		}
	}
}

func TestHappGenerateDoesNotCacheResults(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "current-sub-id")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"encrypted_link":"happ://crypt5/example"}`)
	}))
	defer server.Close()
	svc := newHappTestService(server, time.Second)
	for range 2 {
		if _, err := svc.Generate(context.Background(), client.Id, "panel.example"); err != nil {
			t.Fatalf("Generate: %v", err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
}

func TestSanitizeHappDetailRedactsSensitiveTokens(t *testing.T) {
	detail := sanitizeHappDetail("provider said https://provider.example/path?token=secret password=hunter2\nsource=https://sub.example/sub/current-sub-id", "https://sub.example/sub/current-sub-id", "current-sub-id")
	for _, secret := range []string{"provider.example", "token=secret", "hunter2", "current-sub-id", "\n"} {
		if strings.Contains(detail, secret) {
			t.Fatalf("sanitized detail leaked %q: %q", secret, detail)
		}
	}
}
