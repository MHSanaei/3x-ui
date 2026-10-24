package tgbot

import (
	"io"
	"net"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
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
	client := tg.createRobustFastHTTPClient("http://"+addr, nil)
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
	client := tg.createRobustFastHTTPClient("socks5://"+addr, nil)
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

func TestTgbotPanelEgressBridgeAppearingAfterStartIsUsed(t *testing.T) {
	addr, got := recordingDialTarget(t, 1)
	bridge := ""
	tg := &Tgbot{}
	client := tg.createRobustFastHTTPClient("", func() string { return bridge })
	bridge = "socks5://" + addr
	go func() { _, _ = client.Dial("example.com:443") }()
	select {
	case b := <-got:
		if len(b) != 1 || b[0] != 0x05 {
			t.Fatalf("expected SOCKS5 greeting (0x05) on the late bridge, got %v", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bridge that came up after bot start never received a connection")
	}
}

func TestTgbotPanelEgressDialsDirectWithoutBridge(t *testing.T) {
	addr, got := recordingDialTarget(t, 1)
	tg := &Tgbot{}
	client := tg.createRobustFastHTTPClient("", func() string { return "" })
	conn, err := client.Dial(addr)
	if err != nil {
		t.Fatalf("direct dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte{0x42}); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case b := <-got:
		if len(b) != 1 || b[0] != 0x42 {
			t.Fatalf("expected the payload byte on a direct connection, got %v", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("target never received the direct connection")
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

func TestParseAdminChats(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		ids    []int64
		topics map[int64]int
	}{
		{name: "empty", raw: "", ids: []int64{}, topics: map[int64]int{}},
		{name: "plain ids", raw: "123,-1001", ids: []int64{123, -1001}, topics: map[int64]int{}},
		{name: "topic", raw: "-1003855200872:42", ids: []int64{-1003855200872}, topics: map[int64]int{-1003855200872: 42}},
		{name: "mixed with spaces", raw: " 123 , -1001 : 7 ", ids: []int64{123, -1001}, topics: map[int64]int{-1001: 7}},
		{name: "general topic means no thread", raw: "-1001:1,123", ids: []int64{-1001, 123}, topics: map[int64]int{}},
		{name: "duplicate chat collapses", raw: "-1001:7,-1001:7,-1001", ids: []int64{-1001}, topics: map[int64]int{-1001: 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids, topics, err := parseAdminChats(tc.raw)
			if err != nil {
				t.Fatalf("parseAdminChats(%q) error: %v", tc.raw, err)
			}
			if !slices.Equal(ids, tc.ids) {
				t.Fatalf("ids = %v, want %v", ids, tc.ids)
			}
			if len(topics) != len(tc.topics) {
				t.Fatalf("topics = %v, want %v", topics, tc.topics)
			}
			for chat, topic := range tc.topics {
				if topics[chat] != topic {
					t.Fatalf("topics[%d] = %d, want %d", chat, topics[chat], topic)
				}
			}
		})
	}
}

func TestParseAdminChatsRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"abc", "123:", "123:abc", "123:0", "123:-5", "-1001:7,-1001:8", "123,"} {
		if _, _, err := parseAdminChats(raw); err == nil {
			t.Errorf("parseAdminChats(%q) accepted invalid input", raw)
		}
	}
}

func TestInAdminTopicRoutesConfiguredChat(t *testing.T) {
	tgBotMutex.Lock()
	orig := adminTopics
	adminTopics = map[int64]int{-1001: 42}
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		adminTopics = orig
		tgBotMutex.Unlock()
	})

	doc := inAdminTopic(tu.Document(tu.ID(-1001), tu.FileFromBytes([]byte("x"), "x.db")))
	if doc.MessageThreadID != 42 {
		t.Fatalf("MessageThreadID = %d, want 42", doc.MessageThreadID)
	}
	doc = inAdminTopic(&telego.SendDocumentParams{ChatID: tu.ID(123)})
	if doc.MessageThreadID != 0 {
		t.Fatalf("MessageThreadID = %d for chat without topic, want 0", doc.MessageThreadID)
	}
}
