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

// An older node answers success with a null obj while its status is unsampled,
// which used to surface as "success=false: " with nothing after the colon.
func TestProbeNamesANodeThatHasNoStatusYet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"msg":"","obj":null}`))
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
		Id: 1, Name: "cold", Scheme: "http", Address: u.Hostname(), Port: port,
		BasePath: "/", Enable: true, AllowPrivateAddress: true, TlsVerifyMode: "skip",
	}

	svc := &NodeService{}
	patch, err := svc.Probe(context.Background(), n)
	if err == nil {
		t.Fatal("Probe accepted a status response with no obj, want an error")
	}
	if strings.Contains(patch.LastError, "success=false") {
		t.Fatalf("LastError = %q, want the missing status named instead of a bare success=false", patch.LastError)
	}
	if !strings.Contains(patch.LastError, "no status yet") {
		t.Fatalf("LastError = %q, want it to say the remote reported no status yet", patch.LastError)
	}
}
