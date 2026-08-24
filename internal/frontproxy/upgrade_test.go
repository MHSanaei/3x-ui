package frontproxy

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// echoUpgradeServer answers an Upgrade request with 101 and then echoes every
// line back, standing in for the panel's /ws live-update feed.
func echoUpgradeServer(t *testing.T) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not an upgrade", http.StatusBadRequest)
			return
		}
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		_ = buf.Flush()
		line, err := buf.ReadString('\n')
		if err != nil {
			return
		}
		_, _ = buf.WriteString("echo:" + line)
		_ = buf.Flush()
	}))
	t.Cleanup(srv.Close)
	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

// The panel pushes live updates over /ws, so a protocol upgrade has to survive
// the reverse proxy's hop in both directions.
func TestPanelUpgradeIsTunnelled(t *testing.T) {
	panelPort := echoUpgradeServer(t)
	door := httptest.NewServer(newHandler(
		Config{PanelBasePath: "/secret/", PanelPort: panelPort},
		DecoyConfig{Mode: DecoyTemplate},
	))
	t.Cleanup(door.Close)

	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(door.URL, "http://"), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	_, err = conn.Write([]byte("GET /secret/ws HTTP/1.1\r\nHost: panel.example.com\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading status line: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status = %q, want 101 Switching Protocols", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading headers: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	got, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading echoed frame: %v", err)
	}
	if strings.TrimSpace(got) != "echo:ping" {
		t.Errorf("echoed %q, want %q", strings.TrimSpace(got), "echo:ping")
	}
}
