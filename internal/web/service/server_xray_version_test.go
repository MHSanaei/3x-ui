package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func setupXrayVersionTest(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test executables use /bin/sh")
	}
	root := t.TempDir()
	dbtest.InitDB(t, filepath.Join(root, "x-ui.db"))
	t.Setenv("XUI_MAIN_FOLDER", root)
	t.Setenv("XUI_BIN_FOLDER", filepath.Join(root, "bin"))
	t.Setenv("XUI_LOG_FOLDER", filepath.Join(root, "log"))
	t.Setenv("XUI_IN_DOCKER", "true")
	t.Setenv("XRAY_TEST_STARTS", filepath.Join(root, "starts"))
	restore := SetXrayProcessForTest(nil)
	t.Cleanup(func() {
		_ = (&XrayService{}).StopXray()
		isManuallyStopped.Store(false)
		isNeedXrayRestart.Store(false)
		restore()
	})
	return root
}

func xrayVersionTestBinary(version string) []byte {
	return fmt.Appendf(nil, "#!/bin/sh\nif [ \"$1\" = '-version' ]; then\n echo 'Xray %s test'\nelse\n echo '%s' >> \"$XRAY_TEST_STARTS\"\n exec sleep 600\nfi\n", version, version)
}

func writeXrayVersionBinary(t *testing.T, path, version string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, xrayVersionTestBinary(version), 0o755); err != nil {
		t.Fatal(err)
	}
}

func serveXrayVersion(t *testing.T, version string) *atomic.Int32 {
	t.Helper()
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create("xray")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(xrayVersionTestBinary(version)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch {
		case r.URL.Path == "/repos/XTLS/Xray-core/releases":
			fmt.Fprintf(w, `[{"tag_name":"v%s"},{"tag_name":"v26.6.27"},{"tag_name":"v26.6.26"}]`, version)
		case strings.HasSuffix(r.URL.Path, ".dgst"):
			fmt.Fprintf(w, "SHA2-256= %x\n", sha256.Sum256(archive.Bytes()))
		case strings.Contains(r.URL.Path, "/download/v"+version+"/"):
			_, _ = w.Write(archive.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	previous := http.DefaultTransport
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	http.DefaultTransport = transport
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		http.DefaultTransport = previous
	})
	return &requests
}

func assertStoredXrayVersion(t *testing.T, want string) {
	t.Helper()
	var rows []model.Setting
	if err := database.GetDB().Where("key = ?", "xrayVersion").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if want == "" {
		if len(rows) != 0 {
			t.Fatalf("latest must leave no version setting, got %#v", rows)
		}
	} else if len(rows) != 1 || rows[0].Value != want {
		t.Fatalf("stored selection = %#v, want %q", rows, want)
	}
}

func assertXrayVersionStarts(t *testing.T, root, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, _ := os.ReadFile(filepath.Join(root, "starts"))
		if string(data) == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("started versions = %q, want %q", data, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUpdateXrayPersistsOnlyContainerSelections(t *testing.T) {
	for _, container := range []string{"true", "false"} {
		t.Run(container, func(t *testing.T) {
			root := setupXrayVersionTest(t)
			t.Setenv("XUI_IN_DOCKER", container)
			serveXrayVersion(t, "26.10.10")
			if err := (&ServerService{}).UpdateXray("v26.10.10"); err != nil {
				t.Fatal(err)
			}
			assertXrayVersionStarts(t, root, "26.10.10\n")
			want := ""
			if container == "true" {
				want = "v26.10.10"
			}
			assertStoredXrayVersion(t, want)
			err := (&ServerService{}).UpdateXray("v26.6.26")
			wantError := `xray version "v26.6.26" is not in the fetched release list`
			if err == nil || err.Error() != wantError {
				t.Fatalf("rejected update error = %v, want %q", err, wantError)
			}
			assertStoredXrayVersion(t, want)
		})
	}
}

func TestXrayVersionListOffersLatestOnlyInContainers(t *testing.T) {
	for _, container := range []string{"true", "false"} {
		t.Run(container, func(t *testing.T) {
			setupXrayVersionTest(t)
			t.Setenv("XUI_IN_DOCKER", container)
			serveXrayVersion(t, "26.10.10")
			got, err := (&ServerService{}).GetXrayVersions()
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"v26.10.10", "v26.6.27"}
			if container == "true" {
				want = append([]string{"latest"}, want...)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("versions = %q, want %q", got, want)
			}
		})
	}
}

func TestXrayVersionListKeepsImageChoiceWhenGitHubIsOffline(t *testing.T) {
	setupXrayVersionTest(t)
	previous := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("GitHub is offline")
		},
	}
	t.Cleanup(func() { http.DefaultTransport = previous })
	got, err := (&ServerService{}).GetXrayVersionsCached()
	if err != nil || !slices.Equal(got, []string{"latest"}) {
		t.Fatalf("offline versions = %q, %v; want [latest]", got, err)
	}
}

func TestXrayLatestRestoresImageOfflineAndDeletesPin(t *testing.T) {
	root := setupXrayVersionTest(t)
	requests := serveXrayVersion(t, "26.10.10")
	writeXrayVersionBinary(t, filepath.Join(root, "xray-image", xray.GetBinaryName()), "26.10.11")
	svc := &ServerService{}
	if err := svc.UpdateXray("v26.10.10"); err != nil {
		t.Fatal(err)
	}
	assertXrayVersionStarts(t, root, "26.10.10\n")
	requests.Store(0)
	if err := svc.UpdateXray("latest"); err != nil {
		t.Fatal(err)
	}
	assertStoredXrayVersion(t, "")
	assertXrayVersionStarts(t, root, "26.10.10\n26.10.11\n")
	if got := requests.Load(); got != 0 {
		t.Fatalf("latest made %d network requests, want none", got)
	}
}

func TestContainerXrayPinSurvivesRecreationAndRestartBeforeFirstLaunch(t *testing.T) {
	root := setupXrayVersionTest(t)
	requests := serveXrayVersion(t, "26.10.10")
	svc := &ServerService{}
	if err := svc.UpdateXray("v26.10.10"); err != nil {
		t.Fatal(err)
	}
	assertXrayVersionStarts(t, root, "26.10.10\n")
	if err := svc.StopXrayService(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "starts")); err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(filepath.Join(root, "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	writeXrayVersionBinary(t, xray.GetBinaryPath(), "26.11.1")
	writeXrayVersionBinary(t, filepath.Join(root, "xray-image", xray.GetBinaryName()), "26.11.1")
	requests.Store(0)

	for _, phase := range []string{"recreated with new image", "restarted offline"} {
		t.Run(phase, func(t *testing.T) {
			if err := svc.RestoreXrayVersion(); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(filepath.Join(root, "starts")); !os.IsNotExist(err) {
				t.Fatalf("restoration started Xray before preparation completed: %q, %v", data, err)
			}
			if err := svc.RestartXrayService(); err != nil {
				t.Fatal(err)
			}
			assertXrayVersionStarts(t, root, "26.10.10\n")
			assertStoredXrayVersion(t, "v26.10.10")
			wantRequests := int32(2)
			if phase == "restarted offline" {
				wantRequests = 0
			}
			if got := requests.Load(); got != wantRequests {
				t.Fatalf("startup requests = %d, want %d (archive and checksum only on recreation)", got, wantRequests)
			}
			if err := svc.StopXrayService(); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, "starts")); err != nil {
				t.Fatal(err)
			}
			requests.Store(0)
		})
	}
}

func TestContainerXrayStartupFallbackPreservesPreferenceAndNativeBinary(t *testing.T) {
	for _, tc := range []struct {
		name, pin, container, wantVersion string
		wantRequests                      int32
	}{
		{"latest replaces stale binary", "", "true", "26.10.11", 0},
		{"unsupported pin", "v26.6.26", "true", "26.10.11", 0},
		{"invalid pin", "../bad-version", "true", "26.10.11", 0},
		{"unavailable release", "v26.10.9", "true", "26.10.11", 1},
		{"native installation", "v26.10.9", "false", "26.10.8", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := setupXrayVersionTest(t)
			t.Setenv("XUI_IN_DOCKER", tc.container)
			requests := serveXrayVersion(t, "26.10.10")
			writeXrayVersionBinary(t, xray.GetBinaryPath(), "26.10.8")
			writeXrayVersionBinary(t, filepath.Join(root, "xray-image", xray.GetBinaryName()), "26.10.11")
			if tc.pin != "" {
				if err := (&SettingService{}).setString("xrayVersion", tc.pin); err != nil {
					t.Fatal(err)
				}
			}
			svc := &ServerService{}
			if err := svc.RestoreXrayVersion(); err != nil {
				t.Fatal(err)
			}
			if err := svc.RestartXrayService(); err != nil {
				t.Fatal(err)
			}
			assertXrayVersionStarts(t, root, tc.wantVersion+"\n")
			assertStoredXrayVersion(t, tc.pin)
			if got := requests.Load(); got != tc.wantRequests {
				t.Fatalf("startup requests = %d, want %d", got, tc.wantRequests)
			}
		})
	}
}
