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
		for range 3 {
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

// A panelGuid is copied into every inbound the node reports and into indexed columns, so
// one oversized value multiplies into gigabytes of stored copies.
func TestProbeRefusesAnOversizedPanelGuid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"obj":{"cpuPct":1,"panelGuid":"` + strings.Repeat("g", maxPanelGuidLen+1) + `"}}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	n := &model.Node{Id: 1, Scheme: "http", Address: u.Hostname(), Port: port, BasePath: "/", Enable: true, AllowPrivateAddress: true}
	patch, err := (&NodeService{}).Probe(context.Background(), n)
	if err == nil || patch.Guid != "" || !strings.Contains(patch.LastError, "panelGuid") {
		t.Fatalf("Probe = guid %d bytes, %v (%q); want the heartbeat refused", len(patch.Guid), err, patch.LastError)
	}
}
