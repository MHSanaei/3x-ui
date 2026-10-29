package mtproto

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestQuotaResetReportsHTTPFailureAndUsesCallerContext(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-token" || r.URL.EscapedPath() != "/secrets/ali%2Fone/reset-quota" {
					t.Errorf("incorrect reset request: %s %s", r.Method, r.URL.EscapedPath())
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			_, portString, err := net.SplitHostPort(u.Host)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(portString)
			if err != nil {
				t.Fatal(err)
			}
			err = resetQuotaContext(context.Background(), port, "test-token", "ali/one")
			if (err != nil) != (status >= 400) {
				t.Fatalf("status %d: %v", status, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := resetQuotaContext(ctx, port, "test-token", "ali/one"); err == nil {
				t.Fatal("cancelled reset reported success")
			}
		})
	}
}
