package tgbot

import (
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestLoginAttemptDoesNotCarryPassword(t *testing.T) {
	typ := reflect.TypeFor[LoginAttempt]()
	if _, ok := typ.FieldByName("Password"); ok {
		t.Fatal("LoginAttempt must not carry attempted passwords")
	}
}

func TestIsSupportedBotProxyScheme(t *testing.T) {
	supported := []string{
		"socks5://127.0.0.1:1080",
		"http://127.0.0.1:8080",
		"https://127.0.0.1:8080",
	}
	for _, p := range supported {
		if !isSupportedBotProxyScheme(p) {
			t.Errorf("expected %q to be supported", p)
		}
	}
	unsupported := []string{"", "ftp://x", "127.0.0.1:1080", "socks4://1.2.3.4:1080"}
	for _, p := range unsupported {
		if isSupportedBotProxyScheme(p) {
			t.Errorf("expected %q to be unsupported", p)
		}
	}
}

func recordingDialTarget(t *testing.T, n int) (addr string, got chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got = make(chan []byte, 1)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, n)
		m, _ := io.ReadFull(conn, buf)
		got <- buf[:m]
	}()
	return ln.Addr().String(), got
}

func TestTgbotProxyDialerSelectsHTTPForHTTPScheme(t *testing.T) {
	addr, got := recordingDialTarget(t, len("CONNECT "))
	tg := &Tgbot{}
	client := tg.createRobustFastHTTPClient("http://" + addr)
	if client.Dial == nil {
		t.Fatal("Dial must be set for an http:// proxy")
	}
	go func() { _, _ = client.Dial("example.com:443") }()
	select {
	case b := <-got:
		if string(b) != "CONNECT " {
			t.Fatalf("expected HTTP CONNECT to the proxy, got %q", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy never received a connection")
	}
}

func TestTgbotProxyDialerSelectsSOCKSForSocks5Scheme(t *testing.T) {
	addr, got := recordingDialTarget(t, 1)
	tg := &Tgbot{}
	client := tg.createRobustFastHTTPClient("socks5://" + addr)
	if client.Dial == nil {
		t.Fatal("Dial must be set for a socks5:// proxy")
	}
	go func() { _, _ = client.Dial("example.com:443") }()
	select {
	case b := <-got:
		if len(b) != 1 || b[0] != 0x05 {
			t.Fatalf("expected SOCKS5 greeting (0x05), got %v", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("proxy never received a connection")
	}
}

func TestTgbotProxyDialerNoneWhenEmpty(t *testing.T) {
	tg := &Tgbot{}
	client := tg.createRobustFastHTTPClient("")
	if client.Dial != nil {
		t.Fatal("Dial must be nil when no proxy is configured")
	}
}

func TestIsCommandForBotAllowsUntargetedCommand(t *testing.T) {
	if !isCommandForBot("/status", "panel_bot") {
		t.Fatal("untargeted commands must remain accepted")
	}
}

func TestIsCommandForBotAllowsMatchingUsername(t *testing.T) {
	if !isCommandForBot("/status@panel_bot", "Panel_Bot") {
		t.Fatal("commands targeted to this bot must be accepted")
	}
}

func TestIsCommandForBotRejectsOtherUsername(t *testing.T) {
	if isCommandForBot("/status@other_bot", "panel_bot") {
		t.Fatal("commands targeted to another bot must be ignored")
	}
}

func TestIsCommandForBotKeepsLegacyBehaviorWhenUsernameUnavailable(t *testing.T) {
	if !isCommandForBot("/status@panel_bot", "") {
		t.Fatal("commands must remain accepted when the current bot username is unavailable")
	}
}
