package frontproxy

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// loginMock describes the fake authentication endpoint a login-style decoy answers.
// It always rejects -- there is no real account -- but in the mimicked product's own API shape.
type loginMock struct {
	// Path is the exact request path the real product's login page posts to.
	Path string
	// Threshold and Ban are this template's lockout policy. Zero means
	// loginDefaultThreshold/loginDefaultBan (see registerLoginMock).
	Threshold int
	Ban       time.Duration
	// Reject writes the response for a rejected, not-yet-locked-out attempt.
	Reject func(w http.ResponseWriter)
	// Lockout writes the response once the threshold is hit, for the
	// remaining time until the ban lifts.
	Lockout func(w http.ResponseWriter, retryAfter time.Duration)
}

// loginDefaultThreshold/loginDefaultBan apply to every mocked login except AdGuard Home.
// AGH's own real 5/15-minute policy (internal/home/authratelimiter.go) is used as-is instead.
const (
	loginDefaultThreshold = 5
	loginDefaultBan       = 10 * time.Minute
)

// loginMocks maps a decoy template name to the fake endpoint it answers.
// A template absent here gets no interactive behaviour: newTemplateDecoy's plain page for every path.
var loginMocks = map[string]loginMock{}

func registerLoginMock(template string, m loginMock) {
	if m.Threshold == 0 {
		m.Threshold = loginDefaultThreshold
	}
	if m.Ban == 0 {
		m.Ban = loginDefaultBan
	}
	loginMocks[template] = m
}

// withLoginMock wraps a page handler with template's login mock, if one is registered.
// Everything except a POST to the mock's own path passes through untouched, ETag/Cache-Control/HEAD included.
func withLoginMock(template string, page http.Handler) http.Handler {
	mock, ok := loginMocks[template]
	if !ok {
		return page
	}
	tracker := newLoginAttempts()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != mock.Path {
			page.ServeHTTP(w, r)
			return
		}
		key := clientKey(r)
		if locked, until := tracker.check(key); locked {
			mock.Lockout(w, time.Until(until))
			return
		}
		if locked, until := tracker.fail(key, mock.Threshold, mock.Ban); locked {
			mock.Lockout(w, time.Until(until))
			return
		}
		mock.Reject(w)
	})
}

func init() {
	// AdGuard Home: POST /control/login, {"name","password"} JSON.
	// Reject/lockout shapes copied verbatim from internal/home/authhttp.go + authratelimiter.go.
	registerLoginMock("adguardhome", loginMock{
		Path:      "/control/login",
		Threshold: 5,
		Ban:       15 * time.Minute,
		Reject: func(w http.ResponseWriter) {
			http.Error(w, "invalid username or password", http.StatusForbidden)
		},
		Lockout: func(w http.ResponseWriter, retryAfter time.Duration) {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			http.Error(w, fmt.Sprintf("auth: blocked for %s", retryAfter), http.StatusTooManyRequests)
		},
	})

	// Portainer: POST /api/auth, status/message confirmed from api/http/handler/auth/authenticate.go.
	// That handler has no visible rate limiting, so the lockout here is this mock's own hardening.
	registerLoginMock("portainer", loginMock{
		Path: "/api/auth",
		Reject: func(w http.ResponseWriter) {
			writeJSON(w, http.StatusUnprocessableEntity, `{"message":"Invalid credentials","details":"Unauthorized"}`)
		},
		Lockout: func(w http.ResponseWriter, retryAfter time.Duration) {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeJSON(w, http.StatusTooManyRequests, `{"message":"Too many attempts","details":"Please wait before trying again"}`)
		},
	})

	// Pi-hole (v6 API): POST /api/auth, {"password"} JSON, no username field.
	// Error envelope shape follows FTL's API conventions; not source-verified this session.
	registerLoginMock("pihole", loginMock{
		Path: "/api/auth",
		Reject: func(w http.ResponseWriter) {
			writeJSON(w, http.StatusUnauthorized, `{"error":{"key":"unauthorized","message":"Invalid password","hint":null}}`)
		},
		Lockout: func(w http.ResponseWriter, retryAfter time.Duration) {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeJSON(w, http.StatusTooManyRequests, `{"error":{"key":"rate_limited","message":"Too many login attempts, please try again later","hint":null}}`)
		},
	})

	// OpenMediaVault: POST /rpc.php, a Session.login RPC envelope.
	// Real OMV answers 200 with the error nested in the body, not the HTTP status; not source-verified.
	registerLoginMock("openmediavault", loginMock{
		Path: "/rpc.php",
		Reject: func(w http.ResponseWriter) {
			writeJSON(w, http.StatusOK, `{"response":null,"error":{"code":5001,"message":"Invalid username or password"}}`)
		},
		Lockout: func(w http.ResponseWriter, retryAfter time.Duration) {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeJSON(w, http.StatusOK, `{"response":null,"error":{"code":5002,"message":"Too many failed login attempts, please try again later"}}`)
		},
	})

	// Jellyfin: POST /Users/AuthenticateByName, {"Username","Pw"} JSON; not
	// source-verified this session.
	registerLoginMock("jellyfin", loginMock{
		Path: "/Users/AuthenticateByName",
		Reject: func(w http.ResponseWriter) {
			writeJSON(w, http.StatusUnauthorized, `{"Message":"Invalid username or password."}`)
		},
		Lockout: func(w http.ResponseWriter, retryAfter time.Duration) {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeJSON(w, http.StatusTooManyRequests, `{"Message":"Too many attempts. Please wait before trying again."}`)
		},
	})
}

// writeJSON writes a fixed JSON literal with the right content type. Every
// caller above passes a compile-time constant, never request-derived data.
func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
