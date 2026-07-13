package email

import (
	"io"
	"mime"
	"net/mail"
	"strings"
	"testing"
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
