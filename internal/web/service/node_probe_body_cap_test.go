package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// An oversized status body must be rejected, not buffered whole by encoding/json.
func TestProbeRejectsOversizedStatusBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"obj":{"cpuPct":1,"panelVersion":"`))
		pad := strings.Repeat("x", 1<<20)
		for i := 0; i < 3; i++ {
			_, _ = w.Write([]byte(pad))
		}
		_, _ = w.Write([]byte(`"}}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	n := &model.Node{
		Id: 1, Name: "big", Scheme: "http", Address: u.Hostname(), Port: port,
		BasePath: "/", Enable: true, AllowPrivateAddress: true, TlsVerifyMode: "skip",
	}

	svc := &NodeService{}
	patch, err := svc.Probe(context.Background(), n)
	if err == nil {
		t.Fatal("Probe accepted a 3 MiB status body, want an error")
	}
	// Pin the rejection to the capped decode, not a transport or envelope failure.
	if !strings.HasPrefix(patch.LastError, "decode response: ") {
		t.Fatalf("LastError = %q, want a \"decode response: \" rejection", patch.LastError)
	}
}
