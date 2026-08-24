package service

import (
	"crypto/tls"
	"net"
	"strings"
	"testing"
)

func TestTLSVersionName(t *testing.T) {
	cases := map[uint16]string{
		tls.VersionTLS13: "1.3",
		tls.VersionTLS12: "1.2",
		tls.VersionTLS11: "1.1",
		tls.VersionTLS10: "1.0",
		0:                "unknown",
	}
	for in, want := range cases {
		if got := tlsVersionName(in); got != want {
			t.Errorf("tlsVersionName(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestRealityCurveName(t *testing.T) {
	cases := map[tls.CurveID]string{
		tls.X25519:         "X25519",
		tls.X25519MLKEM768: "X25519MLKEM768",
		tls.CurveP256:      "P-256",
		0:                  "",
	}
	for in, want := range cases {
		if got := realityCurveName(in); got != want {
			t.Errorf("realityCurveName(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFilterUsableSANs(t *testing.T) {
	got := filterUsableSANs([]string{"example.com", "*.example.com", "", " a.com "})
	want := []string{"example.com", "a.com"}
	if len(got) != len(want) {
		t.Fatalf("filterUsableSANs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterUsableSANs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitRealityTarget(t *testing.T) {
	okCases := []struct {
		in       string
		wantHost string
		wantPort int
	}{
		{"example.com", "example.com", 443},
		{"example.com:8443", "example.com", 8443},
		{"1.1.1.1:443", "1.1.1.1", 443},
	}
	for _, c := range okCases {
		host, port, err := splitRealityTarget(c.in)
		if err != nil {
			t.Errorf("splitRealityTarget(%q) unexpected error: %v", c.in, err)
			continue
		}
		if host != c.wantHost || port != c.wantPort {
			t.Errorf("splitRealityTarget(%q) = (%q, %d), want (%q, %d)", c.in, host, port, c.wantHost, c.wantPort)
		}
	}

	badCases := []string{"", "  ", "example.com:0", "example.com:70000", "bad host!"}
	for _, in := range badCases {
		if _, _, err := splitRealityTarget(in); err == nil {
			t.Errorf("splitRealityTarget(%q) expected error, got nil", in)
		}
	}
}

func TestScanRealityTargetInputValidation(t *testing.T) {
	if _, err := (&ServerService{}).ScanRealityTarget("", "", 0, false); err == nil {
		t.Error("ScanRealityTarget(empty) expected error, got nil")
	}
}

func TestScanRealityTargetBlocksPrivate(t *testing.T) {
	res, err := (&ServerService{}).ScanRealityTarget("10.0.0.1:443", "", 0, false)
	if err != nil {
		t.Fatalf("ScanRealityTarget(private) unexpected error: %v", err)
	}
	if res.Feasible {
		t.Error("ScanRealityTarget(private) should not be feasible")
	}
	if !strings.Contains(res.Reason, "blocked private/internal address") {
		t.Errorf("reason = %q, want the SSRF guard to refuse a private address", res.Reason)
	}
}

// Loopback is exempt so the panel's own reverse proxy can be probed before the
// REALITY fallback is pointed at it. Nothing listens on port 1, so the scan
// still fails -- but for the honest reason that nothing answered, not because
// the guard refused to dial.
func TestScanRealityTargetProbesLoopback(t *testing.T) {
	res, err := (&ServerService{}).ScanRealityTarget("127.0.0.1:1", "", 0, false)
	if err != nil {
		t.Fatalf("ScanRealityTarget(loopback) unexpected error: %v", err)
	}
	if strings.Contains(res.Reason, "blocked private/internal address") {
		t.Errorf("loopback still refused by the SSRF guard: %q", res.Reason)
	}
}

func TestScanRealityTargetsHandlesPrivateAndBadInput(t *testing.T) {
	results, err := (&ServerService{}).ScanRealityTargets("127.0.0.1:1,10.0.0.1:443,bad host!")
	if err != nil {
		t.Fatalf("ScanRealityTargets unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ScanRealityTargets returned %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.Feasible {
			t.Errorf("result %q unexpectedly feasible", r.Target)
		}
	}
}

func TestWriteProxyProtocolV1(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	local := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 51234}
	remote := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 443}

	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 128)
		n, _ := server.Read(buf)
		got <- string(buf[:n])
	}()

	if err := writeProxyProtocolV1(client, local, remote); err != nil {
		t.Fatalf("writeProxyProtocolV1: %v", err)
	}
	line := <-got
	want := "PROXY TCP4 192.0.2.10 203.0.113.5 51234 443\r\n"
	if line != want {
		t.Fatalf("v1 header = %q, want %q", line, want)
	}
}

func TestWriteProxyProtocolV2Signature(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	local := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 51234}
	remote := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 443}

	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 128)
		n, _ := server.Read(buf)
		got <- append([]byte(nil), buf[:n]...)
	}()

	if err := writeProxyProtocolV2(client, local, remote); err != nil {
		t.Fatalf("writeProxyProtocolV2: %v", err)
	}
	hdr := <-got
	sig := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	if len(hdr) < 16 || string(hdr[:12]) != string(sig) {
		t.Fatalf("v2 header missing the protocol signature: %v", hdr)
	}
	if hdr[12] != 0x21 {
		t.Fatalf("v2 version/command byte = 0x%02x, want 0x21", hdr[12])
	}
	if hdr[13] != 0x11 {
		t.Fatalf("v2 family/protocol byte = 0x%02x, want 0x11 (TCP over IPv4)", hdr[13])
	}
}
