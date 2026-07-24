package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// EmailService sends email notifications via SMTP.
type EmailService struct {
	settingService service.SettingService
}

// SMTPTestResult holds the result of an SMTP connection test.
type SMTPTestResult struct {
	Success bool   `json:"success"`
	Stage   string `json:"stage"`   // "connect" | "auth" | "send"
	Message string `json:"message"` // classified error message
}

// NewEmailService creates a new EmailService.
func NewEmailService(settingService service.SettingService) *EmailService {
	return &EmailService{settingService: settingService}
}

// smtpConnectTimeout bounds the TCP dial. smtpDeadline bounds every SMTP
// protocol step after the connection is up, so a server that accepts the socket
// but then stalls cannot block the sender goroutine (and leak its socket) long
// after the caller's own timeout has already fired. smtpDeadline is a var only
// so tests can shorten it.
const smtpConnectTimeout = 10 * time.Second

var smtpDeadline = 30 * time.Second

// Send sends an HTML email to all configured recipients.
func (s *EmailService) Send(subject, body string) error {
	host, err := s.settingService.GetSmtpHost()
	if err != nil || host == "" {
		return fmt.Errorf("smtp host not configured")
	}
	port, err := s.settingService.GetSmtpPort()
	if err != nil || port <= 0 {
		port = 587
	}
	username, _ := s.settingService.GetSmtpUsername()
	password, _ := s.settingService.GetSmtpPassword()
	fromAddr, _ := s.settingService.GetSmtpFrom()
	fromName, _ := s.settingService.GetSmtpFromName()
	toStr, _ := s.settingService.GetSmtpTo()
	encryptionType, _ := s.settingService.GetSmtpEncryptionType()

	from := fromAddr
	if from == "" {
		from = username
	}
	if from == "" {
		return fmt.Errorf("smtp from not configured")
	}
	from, fromName = resolveFrom(from, fromName)

	recipients := parseRecipients(toStr)
	if len(recipients) == 0 {
		return fmt.Errorf("no recipients configured")
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	msg := buildMessage(from, fromName, recipients, subject, body)

	// Authenticate only when credentials are set. Go's PlainAuth refuses to run
	// over the unencrypted "none" transport, so an open relay must use nil auth.
	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	// Wrap in a channel with timeout to prevent indefinite blocking
	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		switch encryptionType {
		case "tls":
			ch <- result{s.sendWithTLS(addr, auth, from, recipients, msg, host)}
		case "starttls", "none":
			ch <- result{s.sendPlain(addr, auth, from, recipients, msg, host)}
		default:
			ch <- result{fmt.Errorf("unknown SMTP encryption type: %s", encryptionType)}
		}
	}()

	select {
	case r := <-ch:
		return r.err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("smtp connection timed out after 30s")
	}
}

// TestConnection tests SMTP connection stage by stage and sends a test email.
func (s *EmailService) TestConnection() SMTPTestResult {
	host, err := s.settingService.GetSmtpHost()
	if err != nil || host == "" {
		return SMTPTestResult{false, "connect", "smtpHostNotConfigured"}
	}
	port, err := s.settingService.GetSmtpPort()
	if err != nil || port <= 0 {
		port = 587
	}
	username, _ := s.settingService.GetSmtpUsername()
	password, _ := s.settingService.GetSmtpPassword()
	fromAddr, _ := s.settingService.GetSmtpFrom()
	fromName, _ := s.settingService.GetSmtpFromName()
	toStr, _ := s.settingService.GetSmtpTo()
	encryptionType, _ := s.settingService.GetSmtpEncryptionType()

	from := fromAddr
	if from == "" {
		from = username
	}
	if from == "" {
		return SMTPTestResult{false, "send", "smtpFromNotConfigured"}
	}
	from, fromName = resolveFrom(from, fromName)

	recipients := parseRecipients(toStr)
	if len(recipients) == 0 {
		return SMTPTestResult{false, "send", "smtpNoRecipients"}
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	// Stage 1: Connect
	var conn net.Conn
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	switch encryptionType {
	case "tls":
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}}).DialContext(context.Background(), "tcp", addr)
	default:
		conn, err = dialer.Dial("tcp", addr)
	}

	if err != nil {
		return SMTPTestResult{false, "connect", classifySMTPError(err)}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(smtpDeadline))

	// Stage 2: Handshake + Auth
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return SMTPTestResult{false, "auth", classifySMTPError(err)}
	}
	defer client.Close()

	if err = client.Hello("localhost"); err != nil {
		return SMTPTestResult{false, "auth", classifySMTPError(err)}
	}

	// STARTTLS upgrade for non-TLS connections
	if encryptionType == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return SMTPTestResult{false, "auth", classifySMTPError(err)}
			}
		}
	}

	if username != "" && password != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err = client.Auth(auth); err != nil {
			return SMTPTestResult{false, "auth", classifySMTPError(err)}
		}
	}

	// Stage 3: Send test email
	if err = client.Mail(from); err != nil {
		return SMTPTestResult{false, "send", classifySMTPError(err)}
	}
	for _, r := range recipients {
		if err = client.Rcpt(r); err != nil {
			return SMTPTestResult{false, "send", classifySMTPError(err)}
		}
	}

	msg := buildMessage(from, fromName, recipients, "[3x-ui] Test email",
		`<html><body style="font-family:monospace;font-size:14px">
<h2>Test email from 3x-ui</h2>
<p>If you received this, SMTP is configured correctly.</p>
</body></html>`)

	w, err := client.Data()
	if err != nil {
		return SMTPTestResult{false, "send", classifySMTPError(err)}
	}
	if _, err = w.Write(msg); err != nil {
		return SMTPTestResult{false, "send", classifySMTPError(err)}
	}
	if err = w.Close(); err != nil {
		return SMTPTestResult{false, "send", classifySMTPError(err)}
	}

	return SMTPTestResult{true, "send", "smtpTestSuccess"}
}

func (s *EmailService) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, host string) error {
	// Dial with explicit timeout
	dialer := &net.Dialer{Timeout: smtpConnectTimeout}
	conn, err := (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}}).DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(smtpDeadline))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Hello("localhost"); err != nil {
		return err
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	for _, r := range to {
		if err = client.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}

// sendPlain delivers over a plain TCP connection, opportunistically upgrading
// via STARTTLS when the server advertises it (the behavior net/smtp.SendMail
// gives the "starttls" and "none" transports). Unlike SendMail it dials with a
// timeout and arms a connection deadline, so a server that never speaks or
// stalls mid-protocol cannot block the sender goroutine past smtpDeadline.
func (s *EmailService) sendPlain(addr string, auth smtp.Auth, from string, to []string, msg []byte, host string) error {
	conn, err := (&net.Dialer{Timeout: smtpConnectTimeout}).Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(smtpDeadline))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Hello("localhost"); err != nil {
		return err
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return err
		}
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	for _, r := range to {
		if err = client.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}

// SendTest sends a test email and returns any error with detail.
func (s *EmailService) SendTest() error {
	return s.Send(
		"[3x-ui] Test email",
		`<html><body style="font-family:monospace;font-size:14px">
<h2>Test email from 3x-ui</h2>
<p>If you received this, SMTP is configured correctly.</p>
</body></html>`,
	)
}

// classifySMTPError maps raw SMTP errors to human-readable messages.
func classifySMTPError(err error) string {
	msg := err.Error()
	msgLower := strings.ToLower(msg)

	switch {
	case strings.Contains(msg, "535") || strings.Contains(msgLower, "authentication"):
		return "pages.settings.smtpErrorAuth"
	case strings.Contains(msg, "534") || strings.Contains(msgLower, "starttls"):
		return "pages.settings.smtpErrorStarttls"
	case strings.Contains(msg, "465") || strings.Contains(msgLower, "tls"):
		return "pages.settings.smtpErrorTls"
	case strings.Contains(msgLower, "connection refused") || strings.Contains(msgLower, "dial"):
		return "pages.settings.smtpErrorRefused"
	case strings.Contains(msgLower, "timeout"):
		return "pages.settings.smtpErrorTimeout"
	case strings.Contains(msg, "550") || strings.Contains(msgLower, "relay"):
		return "pages.settings.smtpErrorRelay"
	case strings.Contains(msgLower, "eof"):
		return "pages.settings.smtpErrorEof"
	default:
		return fmt.Sprintf("pages.settings.smtpErrorUnknown: %s", msg)
	}
}

func parseRecipients(toStr string) []string {
	if toStr == "" {
		return nil
	}
	var out []string
	for s := range strings.SplitSeq(toStr, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// buildMessage assembles an RFC 5322 message. It emits the two mandatory
// header fields (Date, From) plus Message-ID, so strict receivers such as Gmail
// accept it and spam filters do not penalize a missing date or message id. The
// From header is a proper name-addr ("Name" <addr>) via net/mail, and a
// non-ASCII subject is RFC 2047 encoded.
// headerSanitizer drops CR/LF so a crafted address or name cannot inject extra
// header lines. Configured addresses are already validated at save time
// (entity.AllSetting.CheckValid), this is defense in depth for buildMessage.
var headerSanitizer = strings.NewReplacer("\r", "", "\n", "")

func resolveFrom(from, fromName string) (string, string) {
	parsed, err := mail.ParseAddress(from)
	if err != nil {
		return from, fromName
	}
	if fromName == "" {
		fromName = parsed.Name
	}
	return parsed.Address, fromName
}

func buildMessage(fromAddr, fromName string, to []string, subject, body string) []byte {
	fromAddr = headerSanitizer.Replace(fromAddr)
	fromName = headerSanitizer.Replace(fromName)
	from := (&mail.Address{Name: fromName, Address: fromAddr}).String()

	domain := "localhost"
	if at := strings.LastIndex(fromAddr, "@"); at >= 0 && at+1 < len(fromAddr) {
		domain = fromAddr[at+1:]
	}
	var token [16]byte
	_, _ = rand.Read(token[:])
	messageID := fmt.Sprintf("<%s@%s>", hex.EncodeToString(token[:]), domain)

	var msg strings.Builder
	fmt.Fprintf(&msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&msg, "From: %s\r\n", from)
	fmt.Fprintf(&msg, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&msg, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	return []byte(msg.String())
}
