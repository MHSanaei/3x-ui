package frontproxy

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// ukSocketIOPath is Socket.IO's own default HTTP path; Uptime Kuma
// (louislam/uptime-kuma, server/server.js) never changes it.
const ukSocketIOPath = "/socket.io/"

// ukLoginEventPattern matches a Socket.IO EVENT packet carrying a "login"
// call: engine.io message type 4 is stripped by the transport already, so
// what a POST body carries is "42<ackId>[\"login\",<data>]" -- socket.io
// packet type 2 (EVENT) followed by the ack id socket.io-client assigns
// when it wants a callback.
var ukLoginEventPattern = regexp.MustCompile(`^42(\d*)\["login"`)

func ukRandSid() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// uptimeKumaHandler speaks just enough Engine.IO v4 / Socket.IO v4 wire
// framing for one login round-trip: a polling handshake, the CONNECT
// packet, and a "login" event with an ack -- verified from server.js's
// `socket.on("login", ...)` (event name, {ok,msg,msgi18n} reject shape) and
// rate-limiter.js's loginRateLimiter (a 20-token-per-minute bucket that
// refills continuously, not a hard N-attempts-then-locked-for-M-minutes
// ban like the other mocks here, hence its own token bucket below instead
// of loginAttempts).
//
// Real socket.io only ever delivers an ack via the separate long-polling
// GET side, never in the POST's own response. This mock answers directly
// in the POST response instead -- a deliberate simplification: both sides
// of this exchange are this mock's own hand-written client script, not the
// real socket.io-client, so nothing downstream depends on matching that
// ordering.
func uptimeKumaHandler(page http.Handler, _ *loginAttempts, _ int, _ time.Duration) http.Handler {
	buckets := newUKBuckets()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ukSocketIOPath {
			page.ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			ukWriteHandshake(w)
		case http.MethodPost:
			ukHandlePost(w, r, buckets)
		default:
			http.Error(w, "", http.StatusMethodNotAllowed)
		}
	})
}

// ukWriteHandshake answers the Engine.IO handshake GET with an OPEN packet
// (type "0") advertising no upgrades, so a real client never attempts the
// WebSocket transport this mock doesn't implement and stays on polling.
func ukWriteHandshake(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	_, _ = io.WriteString(w, `0{"sid":"`+ukRandSid()+`","upgrades":[],"pingInterval":25000,"pingTimeout":20000,"maxPayload":1000000}`)
}

func ukHandlePost(w http.ResponseWriter, r *http.Request, buckets *ukBuckets) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")

	if string(body) == "40" {
		// Socket.IO CONNECT for the default namespace; the socket.io-level
		// session id here is separate from the Engine.IO one above.
		_, _ = io.WriteString(w, `40{"sid":"`+ukRandSid()+`"}`)
		return
	}

	m := ukLoginEventPattern.FindStringSubmatch(string(body))
	if m == nil {
		_, _ = io.WriteString(w, "ok")
		return
	}
	payload, _ := json.Marshal([]any{ukLoginResult(buckets.take(clientKey(r)))})
	// Socket.IO ACK (type "3") for the same ack id the EVENT packet carried.
	_, _ = io.WriteString(w, "43"+m[1]+string(payload))
}

// ukLoginResult mirrors server.js's socket.on("login", ...) reply shapes
// verbatim: msgi18n:true marks "authIncorrectCreds" as an i18n key the real
// frontend looks up and translates, not a literal displayed string.
func ukLoginResult(allowed bool) map[string]any {
	if !allowed {
		return map[string]any{"ok": false, "msg": "Too frequently, try again later."}
	}
	return map[string]any{"ok": false, "msg": "authIncorrectCreds", "msgi18n": true}
}

// ukBucketCapacity/ukBucketRefillPerSecond mirror rate-limiter.js's
// loginRateLimiter exactly: 20 tokens, refilling continuously at 20/minute,
// starting full (fireImmediately: true) rather than empty.
const (
	ukBucketCapacity        = 20.0
	ukBucketRefillPerSecond = ukBucketCapacity / 60.0
)

type ukBucket struct {
	tokens     float64
	lastRefill time.Time
}

// ukBuckets tracks one token bucket per source IP, the same clientKey used
// by loginAttempts elsewhere in this package.
type ukBuckets struct {
	mu    sync.Mutex
	state map[string]*ukBucket
	now   func() time.Time
}

func newUKBuckets() *ukBuckets {
	return &ukBuckets{state: make(map[string]*ukBucket), now: time.Now}
}

// take consumes one token for key, reporting whether the request may
// proceed. false means the bucket ran dry: rate-limited, matching
// KumaRateLimiter.pass's "remainingRequests < 0" check.
func (b *ukBuckets) take(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	s := b.state[key]
	if s == nil {
		s = &ukBucket{tokens: ukBucketCapacity, lastRefill: now}
		b.state[key] = s
	}
	s.tokens = min(ukBucketCapacity, s.tokens+now.Sub(s.lastRefill).Seconds()*ukBucketRefillPerSecond)
	s.lastRefill = now
	s.tokens--
	return s.tokens >= 0
}
