package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type containerStartupTransport struct {
	fixture *url.URL
	base    http.Transport
}

func (c *containerStartupTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	local := request.Clone(request.Context())
	local.URL.Scheme = c.fixture.Scheme
	local.URL.Host = c.fixture.Host
	return c.base.RoundTrip(local)
}

func TestContainerStartupRestoresPinBeforeLaunchingXray(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executables use /bin/sh and Unix signals")
	}
	if fixture := os.Getenv("XUI_TEST_STARTUP_FIXTURE"); fixture != "" {
		target, err := url.Parse(fixture)
		if err != nil {
			t.Fatal(err)
		}
		http.DefaultTransport = &containerStartupTransport{fixture: target}
		runWebServer()
		return
	}

	root := t.TempDir()
	t.Setenv("XUI_DB_TYPE", "sqlite")
	t.Setenv("XUI_DB_FOLDER", root)
	t.Setenv("XUI_IN_DOCKER", "true")
	t.Setenv("XUI_MAIN_FOLDER", root)
	t.Setenv("XUI_BIN_FOLDER", filepath.Join(root, "bin"))
	t.Setenv("XUI_LOG_FOLDER", filepath.Join(root, "log"))
	t.Setenv("XUI_LOG_LEVEL", "error")
	t.Setenv("XRAY_TEST_STARTS", filepath.Join(root, "starts"))
	dbtest.InitDB(t, config.GetDBPath())
	settings := []model.Setting{
		{Key: "xrayVersion", Value: "v26.10.10"},
		{Key: "webListen", Value: "127.0.0.1"},
		{Key: "subEnable", Value: "false"},
	}
	if err := database.GetDB().Create(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_PORT", fmt.Sprint(listener.Addr().(*net.TCPAddr).Port))
	listener.Close()

	binary := func(version string) []byte {
		return fmt.Appendf(nil, "#!/bin/sh\nif [ \"$1\" = '-version' ]; then\n echo 'Xray %s test'\nelse\n echo '%s' >> \"$XRAY_TEST_STARTS\"\n exec sleep 600\nfi\n", version, version)
	}
	for _, path := range []string{xray.GetBinaryPath(), filepath.Join(root, "xray-image", xray.GetBinaryName())} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, binary("26.11.1"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	entry, err := zw.Create("xray")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(binary("26.10.10")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/releases/download/v26.10.10/") {
			http.NotFound(w, r)
		} else if strings.HasSuffix(r.URL.Path, ".dgst") {
			fmt.Fprintf(w, "SHA2-256= %x\n", sha256.Sum256(archive.Bytes()))
		} else {
			_, _ = w.Write(archive.Bytes())
		}
	}))
	defer fixture.Close()
	t.Setenv("XUI_TEST_STARTUP_FIXTURE", fixture.URL)
	logPath := filepath.Join(root, "panel.log")
	output, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestContainerStartupRestoresPinBeforeLaunchingXray$")
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-finished
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, _ := os.ReadFile(filepath.Join(root, "starts"))
		if len(data) > 0 {
			if string(data) != "26.10.10\n" {
				t.Fatalf("startup launched %q, want only the saved pin 26.10.10", data)
			}
			return
		}
		if time.Now().After(deadline) {
			logs, _ := os.ReadFile(logPath)
			t.Fatalf("Xray did not start: %s", logs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
