package service

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

// A box with no IPv6 route spends 3s per lookup service, and a status sample
// that waits for that is a panel reporting nothing for the first ~15s.
func TestStatusSampleDoesNotWaitOnPublicIPLookup(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte("203.0.113.7"))
	}))
	t.Cleanup(srv.Close)

	savedV4, savedV6 := publicIPv4Services, publicIPv6Services
	publicIPv4Services = []string{srv.URL}
	publicIPv6Services = []string{srv.URL}
	t.Cleanup(func() { publicIPv4Services, publicIPv6Services = savedV4, savedV6 })

	svc := &ServerService{}
	status := svc.CurrentStatus()
	if status == nil {
		t.Fatal("CurrentStatus returned nil while the IP lookup was in flight")
	}
	if status.PublicIP.IPv4 != "" {
		t.Fatalf("the sample waited for the lookup: PublicIP.IPv4 = %q, want it still unresolved", status.PublicIP.IPv4)
	}

	close(release)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if ipv4, _ := svc.publicIPs(); ipv4 == "203.0.113.7" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the background lookup never cached the public IP")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
