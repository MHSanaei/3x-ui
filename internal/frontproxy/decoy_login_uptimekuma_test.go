package frontproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestUKBucketAllowsUpToCapacityThenRateLimits(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := newUKBuckets()
	b.now = func() time.Time { return clock }

	for i := 0; i < ukBucketCapacity; i++ {
		if !b.take("1.2.3.4") {
			t.Fatalf("attempt %d: rate-limited before the bucket ran dry", i+1)
		}
	}
	if b.take("1.2.3.4") {
		t.Fatal("attempt past capacity was allowed, want rate-limited")
	}
}

func TestUKBucketRefillsOverTime(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := newUKBuckets()
	b.now = func() time.Time { return clock }

	for i := 0; i < ukBucketCapacity; i++ {
		b.take("5.6.7.8")
	}
	if b.take("5.6.7.8") {
		t.Fatal("expected the bucket to be empty")
	}
	// One minute at 20/minute refills a full bucket's worth.
	clock = clock.Add(time.Minute)
	if !b.take("5.6.7.8") {
		t.Fatal("expected a refilled token one minute later")
	}
}

func TestUKBucketKeysAreIndependent(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := newUKBuckets()
	b.now = func() time.Time { return clock }

	for i := 0; i < ukBucketCapacity; i++ {
		b.take("9.9.9.9")
	}
	if !b.take("1.1.1.1") {
		t.Fatal("a different key must have its own full bucket")
	}
}

var ukHandshakeOpen = regexp.MustCompile(`^0(\{.*\})$`)

func TestUptimeKumaHandshakeAdvertisesNoUpgrades(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "uptimekuma", Seed: "test"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reqFrom(http.MethodGet, ukSocketIOPath, "203.0.113.20", ""))
	if w.Code != http.StatusOK {
		t.Fatalf("handshake: status = %d, want 200", w.Code)
	}
	m := ukHandshakeOpen.FindStringSubmatch(w.Body.String())
	if m == nil {
		t.Fatalf("handshake body = %q, want an Engine.IO OPEN packet", w.Body.String())
	}
	var open struct {
		SID      string   `json:"sid"`
		Upgrades []string `json:"upgrades"`
	}
	if err := json.Unmarshal([]byte(m[1]), &open); err != nil {
		t.Fatalf("OPEN packet payload: bad JSON: %v", err)
	}
	if open.SID == "" {
		t.Fatal("OPEN packet has no sid")
	}
	if len(open.Upgrades) != 0 {
		t.Fatalf("upgrades = %v, want none (so a real client never tries the WebSocket transport)", open.Upgrades)
	}
}

func ukEmitLogin(t *testing.T, h http.Handler, ip string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	body := `420["login",{"username":"admin","password":"wrong"}]`
	h.ServeHTTP(w, reqFrom(http.MethodPost, ukSocketIOPath, ip, body))
	if w.Code != http.StatusOK {
		t.Fatalf("login event: status = %d, want 200", w.Code)
	}
	text := w.Body.String()
	if !strings.HasPrefix(text, "430") {
		t.Fatalf("login ack = %q, want an ACK packet echoing ack id 0", text)
	}
	var args []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(text, "430")), &args); err != nil {
		t.Fatalf("ack payload: bad JSON: %v", err)
	}
	if len(args) != 1 {
		t.Fatalf("ack payload has %d elements, want 1", len(args))
	}
	return args[0]
}

func TestUptimeKumaLoginEventAlwaysRejects(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "uptimekuma", Seed: "test"})
	result := ukEmitLogin(t, h, "203.0.113.21")
	if result["ok"] != false {
		t.Fatalf("result = %v, want ok:false", result)
	}
	if result["msg"] != "authIncorrectCreds" || result["msgi18n"] != true {
		t.Fatalf("result = %v, want the real server.js authIncorrectCreds/msgi18n shape", result)
	}
}

func TestUptimeKumaLoginEventRateLimitsAfterBucketCapacity(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "uptimekuma", Seed: "test"})
	ip := "203.0.113.22"
	for i := 0; i < ukBucketCapacity; i++ {
		ukEmitLogin(t, h, ip)
	}
	result := ukEmitLogin(t, h, ip)
	if result["msg"] != "Too frequently, try again later." {
		t.Fatalf("result = %v, want the real rate-limiter.js errorMessage", result)
	}
	if _, hasI18n := result["msgi18n"]; hasI18n {
		t.Fatalf("result = %v, want no msgi18n flag (this message is plain text, not an i18n key)", result)
	}
}

func TestUptimeKumaConnectAcksWithASid(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "uptimekuma", Seed: "test"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reqFrom(http.MethodPost, ukSocketIOPath, "203.0.113.23", "40"))
	if !strings.HasPrefix(w.Body.String(), `40{"sid":"`) {
		t.Fatalf("connect ack = %q, want a Socket.IO CONNECT ack with a sid", w.Body.String())
	}
}
