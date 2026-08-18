package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// seedWarp stores warp credentials (with a Warp Plus license key) in the DB.
func seedWarp(t *testing.T, license string) {
	t.Helper()
	oldData := fmt.Sprintf(
		`{"access_token":"old-token","device_id":"old-device","license_key":%q,"private_key":"old-priv"}`,
		license,
	)
	if err := database.GetDB().Create(&model.Setting{Key: "warp", Value: oldData}).Error; err != nil {
		t.Fatalf("seed warp: %v", err)
	}
}

// mockWarpAPI emulates the Cloudflare WARP registration API. When reapplyFails
// is true, the PUT /reg/{id}/account endpoint returns 500 (license rejected).
func mockWarpAPI(t *testing.T, reapplyFails bool) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	regCalls := &atomic.Int32{}
	licCalls := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/reg":
			regCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"new-device","token":"new-token","account":{"license":""},"config":{"client_id":"YWJj"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/reg/new-device/account":
			licCalls.Add(1)
			if reapplyFails {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"license already in use"}`))
				return
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["license"] != "WARPPLLUS-KEY-0123456789abcdefgh" {
				t.Errorf("re-apply license: got %q, want the saved Warp Plus key", body["license"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"new-device"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, regCalls, licCalls
}

func withWarpAPIBase(t *testing.T, base string) {
	t.Helper()
	orig := warpAPIBase
	warpAPIBase = base
	t.Cleanup(func() { warpAPIBase = orig })
}

func TestChangeWarpIPPreservesLicenseKey(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const license = "WARPPLLUS-KEY-0123456789abcdefgh" // 32 chars, >= 26 gate
	seedWarp(t, license)
	srv, regCalls, licCalls := mockWarpAPI(t, false)
	withWarpAPIBase(t, srv.URL)

	s := &WarpService{}
	resp, err := s.ChangeWarpIP()
	if err != nil {
		t.Fatalf("ChangeWarpIP: %v", err)
	}

	// Storage must keep the license key and the new device id.
	stored, err := s.GetWarp()
	if err != nil {
		t.Fatalf("GetWarp: %v", err)
	}
	var storedData map[string]string
	if err := json.Unmarshal([]byte(stored), &storedData); err != nil {
		t.Fatalf("unmarshal stored warp: %v", err)
	}
	if storedData["license_key"] != license {
		t.Errorf("stored license_key = %q, want %q (key must survive changeIp)", storedData["license_key"], license)
	}
	if storedData["device_id"] != "new-device" {
		t.Errorf("stored device_id = %q, want %q (IP must still rotate)", storedData["device_id"], "new-device")
	}

	// The response must carry the license key so the UI shows it.
	var parsed struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if parsed.Data["license_key"] != license {
		t.Errorf("response license_key = %q, want %q", parsed.Data["license_key"], license)
	}
	if strings.Contains(resp, "warning") {
		t.Errorf("response unexpectedly contains a warning: %s", resp)
	}

	if regCalls.Load() != 1 {
		t.Errorf("reg calls = %d, want 1", regCalls.Load())
	}
	if licCalls.Load() != 1 {
		t.Errorf("license re-apply calls = %d, want 1", licCalls.Load())
	}
}

func TestChangeWarpIPKeepsLicenseWhenReapplyFails(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const license = "WARPPLLUS-KEY-0123456789abcdefgh"
	seedWarp(t, license)
	srv, _, licCalls := mockWarpAPI(t, true)
	withWarpAPIBase(t, srv.URL)

	s := &WarpService{}
	resp, err := s.ChangeWarpIP()
	if err != nil {
		t.Fatalf("ChangeWarpIP: %v", err)
	}

	// Even when Cloudflare rejects the re-apply, the saved key must stay.
	stored, err := s.GetWarp()
	if err != nil {
		t.Fatalf("GetWarp: %v", err)
	}
	var storedData map[string]string
	if err := json.Unmarshal([]byte(stored), &storedData); err != nil {
		t.Fatalf("unmarshal stored warp: %v", err)
	}
	if storedData["license_key"] != license {
		t.Errorf("stored license_key = %q, want %q (re-apply failure must not delete the key)", storedData["license_key"], license)
	}

	// The response must warn the user instead of silently succeeding.
	var parsed struct {
		Data    map[string]string `json:"data"`
		Warning string            `json:"warning"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if parsed.Warning == "" {
		t.Error("response missing warning about failed license re-apply")
	}
	if parsed.Data["license_key"] != license {
		t.Errorf("response license_key = %q, want %q", parsed.Data["license_key"], license)
	}
	if licCalls.Load() != 1 {
		t.Errorf("license re-apply calls = %d, want 1", licCalls.Load())
	}
}
