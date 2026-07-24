package email

import (
	"bufio"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestBuildMessageIsRFC5322(t *testing.T) {
	raw := buildMessage("panel@example.com", "3x-ui", []string{"a@example.com", "b@example.com"}, "Тест", "<b>hi</b>")

	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("message does not parse as RFC 5322: %v", err)
	}

	from, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		t.Fatalf("From header does not parse: %v", err)
	}
	if from.Name != "3x-ui" || from.Address != "panel@example.com" {
		t.Errorf("From = %q <%q>, want name %q addr %q", from.Name, from.Address, "3x-ui", "panel@example.com")
	}

	if _, err := msg.Header.Date(); err != nil {
		t.Errorf("Date header missing or unparseable: %v", err)
	}

	id := msg.Header.Get("Message-ID")
	if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@example.com>") {
		t.Errorf("Message-ID = %q, want <token@example.com>", id)
	}

	subject, err := (&mime.WordDecoder{}).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("Subject does not decode: %v", err)
	}
	if subject != "Тест" {
		t.Errorf("Subject = %q, want %q", subject, "Тест")
	}

	body, _ := io.ReadAll(msg.Body)
	if string(body) != "<b>hi</b>" {
		t.Errorf("body = %q, want %q", body, "<b>hi</b>")
	}
}

func TestBuildMessageFromWithoutName(t *testing.T) {
	raw := buildMessage("panel@example.com", "", []string{"a@example.com"}, "s", "b")
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	from, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		t.Fatalf("From header does not parse: %v", err)
	}
	if from.Name != "" || from.Address != "panel@example.com" {
		t.Errorf("From = %q <%q>, want bare addr", from.Name, from.Address)
	}
}

func startFakeSMTPServer(t *testing.T) (string, func() []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	var mu sync.Mutex
	var lines []string
	record := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, line)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 fake ready\r\n")
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			record(line)
			if inData {
				if line == "." {
					inData = false
					fmt.Fprint(conn, "250 ok\r\n")
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "DATA"):
				inData = true
				fmt.Fprint(conn, "354 send\r\n")
			case strings.HasPrefix(line, "QUIT"):
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()

	return ln.Addr().String(), func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), lines...)
	}
}

func TestSendUsesBareAddressFromNameAddrSmtpFrom(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	addr, recordedLines := startFakeSMTPServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	settingService := service.SettingService{}
	mustSet := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	mustSet("host", settingService.SetSmtpHost(host))
	mustSet("port", settingService.SetSmtpPort(port))
	mustSet("from", settingService.SetSmtpFrom("3x-ui Panel <panel@example.com>"))
	mustSet("to", settingService.SetSmtpTo("admin@example.com"))
	mustSet("encryption", settingService.SetSmtpEncryptionType("none"))

	if err := NewEmailService(settingService).Send("subject", "<b>hi</b>"); err != nil {
		t.Fatalf("send: %v", err)
	}

	var mailFrom, fromHeader string
	for _, line := range recordedLines() {
		if strings.HasPrefix(line, "MAIL FROM:") {
			mailFrom = line
		}
		if strings.HasPrefix(line, "From: ") {
			fromHeader = line
		}
	}
	if want := "MAIL FROM:<panel@example.com>"; mailFrom != want {
		t.Errorf("envelope sender = %q, want %q", mailFrom, want)
	}
	if want := `From: "3x-ui Panel" <panel@example.com>`; fromHeader != want {
		t.Errorf("from header = %q, want %q", fromHeader, want)
	}
}

func TestConnectionReportsMissingFrom(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	settingService := service.SettingService{}
	mustSet := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	mustSet("host", settingService.SetSmtpHost("127.0.0.1"))
	mustSet("port", settingService.SetSmtpPort(1))
	mustSet("to", settingService.SetSmtpTo("admin@example.com"))
	mustSet("encryption", settingService.SetSmtpEncryptionType("none"))

	got := NewEmailService(settingService).TestConnection()
	want := SMTPTestResult{Success: false, Stage: "send", Message: "smtpFromNotConfigured"}
	if got != want {
		t.Errorf("TestConnection() = %+v, want %+v", got, want)
	}
}

func TestBuildMessageStripsHeaderInjection(t *testing.T) {
	raw := buildMessage(
		"panel@example.com\r\nBcc: evil@example.com",
		"Name\r\nX-Evil: 1",
		[]string{"a@example.com"}, "s", "b",
	)
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := msg.Header.Get("Bcc"); got != "" {
		t.Errorf("injected Bcc header leaked: %q", got)
	}
	if got := msg.Header.Get("X-Evil"); got != "" {
		t.Errorf("injected X-Evil header leaked: %q", got)
	}
}
