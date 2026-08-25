package frontproxy

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// reqFrom builds a request against handler as if it came from remoteIP.
func reqFrom(method, path, remoteIP string, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.RemoteAddr = remoteIP + ":54321"
	return r
}

func TestLoginMockTemplatesAreRegisteredAndServeTheirPage(t *testing.T) {
	for _, name := range []string{"adguardhome", "portainer", "pihole", "openmediavault", "jellyfin"} {
		t.Run(name, func(t *testing.T) {
			h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: name, Seed: "test"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, reqFrom(http.MethodGet, "/", "203.0.113.1", ""))
			if w.Code != http.StatusOK {
				t.Fatalf("GET / = %d, want 200", w.Code)
			}
			if w.Body.Len() == 0 {
				t.Fatal("GET / returned an empty body")
			}
		})
	}
}

func TestLoginMockRejectsThenLocksOutAfterThreshold(t *testing.T) {
	cases := []struct {
		template      string
		path          string
		body          string
		rejectCode    int
		lockoutCode   int
		lockoutInBody string // set only where lockout is signalled in the body, not the HTTP status
	}{
		{"adguardhome", "/control/login", `{"name":"admin","password":"wrong"}`, http.StatusForbidden, http.StatusTooManyRequests, ""},
		{"portainer", "/api/auth", `{"Username":"admin","Password":"wrong"}`, http.StatusUnprocessableEntity, http.StatusTooManyRequests, ""},
		{"pihole", "/api/auth", `{"password":"wrong"}`, http.StatusUnauthorized, http.StatusTooManyRequests, ""},
		// OMV's RPC envelope always answers 200; the error (including a
		// lockout) is signalled inside the body, never via HTTP status.
		{"openmediavault", "/rpc.php", `{"service":"Session","method":"login","params":{"username":"admin","password":"wrong"}}`, http.StatusOK, http.StatusOK, `"code":5002`},
		{"jellyfin", "/Users/AuthenticateByName", `{"Username":"admin","Pw":"wrong"}`, http.StatusUnauthorized, http.StatusTooManyRequests, ""},
	}
	for _, c := range cases {
		t.Run(c.template, func(t *testing.T) {
			mock, ok := loginMocks[c.template]
			if !ok {
				t.Fatalf("no loginMock registered for %q", c.template)
			}
			h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: c.template, Seed: "test"})
			ip := "198.51.100." + c.template // unique per subtest, avoids cross-test lockout bleed

			for i := 0; i < mock.Threshold-1; i++ {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, reqFrom(http.MethodPost, c.path, ip, c.body))
				if w.Code != c.rejectCode {
					t.Fatalf("attempt %d: status = %d, want reject code %d", i+1, w.Code, c.rejectCode)
				}
			}

			w := httptest.NewRecorder()
			h.ServeHTTP(w, reqFrom(http.MethodPost, c.path, ip, c.body))
			if w.Code != c.lockoutCode {
				t.Fatalf("threshold attempt: status = %d, want lockout code %d", w.Code, c.lockoutCode)
			}
			if c.lockoutInBody != "" && !strings.Contains(w.Body.String(), c.lockoutInBody) {
				t.Fatalf("threshold attempt body = %q, want it to contain %q", w.Body.String(), c.lockoutInBody)
			}
			retryAfter := w.Header().Get("Retry-After")
			if retryAfter == "" {
				t.Fatal("locked-out response has no Retry-After header")
			}
			if n, err := strconv.Atoi(retryAfter); err != nil || n <= 0 {
				t.Fatalf("Retry-After = %q, want a positive integer", retryAfter)
			}

			// One more attempt while still locked must stay locked, without
			// counting as a fresh failure (check(), not fail(), handles it).
			w2 := httptest.NewRecorder()
			h.ServeHTTP(w2, reqFrom(http.MethodPost, c.path, ip, c.body))
			if w2.Code != c.lockoutCode {
				t.Fatalf("still-locked attempt: status = %d, want %d", w2.Code, c.lockoutCode)
			}
		})
	}
}

func TestLoginMockTracksSourceIPsIndependently(t *testing.T) {
	mock := loginMocks["adguardhome"]
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "adguardhome", Seed: "test"})
	body := `{"name":"admin","password":"wrong"}`

	for i := 0; i < mock.Threshold; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, reqFrom(http.MethodPost, mock.Path, "203.0.113.50", body))
		_ = w
	}
	locked := httptest.NewRecorder()
	h.ServeHTTP(locked, reqFrom(http.MethodPost, mock.Path, "203.0.113.50", body))
	if locked.Code != http.StatusTooManyRequests {
		t.Fatalf("the attacking IP should be locked out, got %d", locked.Code)
	}

	fresh := httptest.NewRecorder()
	h.ServeHTTP(fresh, reqFrom(http.MethodPost, mock.Path, "203.0.113.99", body))
	if fresh.Code != http.StatusForbidden {
		t.Fatalf("a different source IP should not be locked out, got %d", fresh.Code)
	}
}

func TestLoginMockLeavesOtherPathsAndUnmockedTemplatesAlone(t *testing.T) {
	// A registered mock only intercepts its own exact POST path -- everything
	// else still reaches the underlying static page handler unchanged.
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "adguardhome", Seed: "test"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reqFrom(http.MethodGet, "/control/login", "203.0.113.1", ""))
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET (not POST) to the mock path = %d, want 404 from the static page handler", w.Code)
	}

	// A template with no registered mock must behave exactly like before:
	// no interactive behaviour, no lockout state, plain static page.
	static := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "calc", Seed: "test"})
	w2 := httptest.NewRecorder()
	static.ServeHTTP(w2, reqFrom(http.MethodPost, "/anything", "203.0.113.1", "irrelevant"))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("POST to an unmocked template = %d, want 404 (unaffected static behaviour)", w2.Code)
	}
}
