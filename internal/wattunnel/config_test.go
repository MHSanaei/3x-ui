package wattunnel

import (
	"os"
	"strings"
	"testing"
)

func TestTokenRoundTripAndSecretIsolation(t *testing.T) {
	c := Config{Role: "server", Carrier: "tcp", Endpoint: "example.com", Port: 4443, Bind: "0.0.0.0", Ports: []Port{{Listen: 443, Target: 443, Protocol: "tcp"}}}
	token, err := EncodeToken(c, "private-test-key")
	if err != nil {
		t.Fatal(err)
	}
	got, key, err := DecodeToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if key != "private-test-key" || got.Role != "client" || got.Endpoint != "example.com" || got.Port != 4443 || got.Ports[0].Target != 443 {
		t.Fatalf("token changed connection fields: %+v", got)
	}
	rendered, err := Render(got, "/tmp/key", "127.0.0.1:19700", "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered, key) || !strings.Contains(rendered, "key-file: /tmp/key") || !strings.Contains(rendered, "peer: example.com:4443") {
		t.Fatalf("invalid rendered config: %s", rendered)
	}
}

func TestConfigRejectsInjectionAndDuplicatePorts(t *testing.T) {
	base := Config{Role: "client", Carrier: "tcp", Endpoint: "example.com", Port: 4443, Bind: "0.0.0.0", Ports: []Port{{Listen: 8443, Target: 443, Protocol: "tcp"}}}
	for _, field := range []string{"endpoint", "path", "duplicate"} {
		t.Run(field, func(t *testing.T) {
			c := base
			switch field {
			case "endpoint":
				c.Endpoint = "example.com\nkey: injected"
			case "path":
				c.Path = "/wat\nmetrics: :80"
			case "duplicate":
				c.Ports = append(append([]Port{}, c.Ports...), c.Ports[0])
			}
			if err := c.Validate(); err == nil {
				t.Fatalf("accepted %s injection/conflict", field)
			}
		})
	}
	base.Ports = append(base.Ports, Port{Listen: 8443, Target: 443, Protocol: "udp"})
	if err := base.Validate(); err != nil {
		t.Fatalf("TCP/UDP may share a number: %v", err)
	}
}

func TestTokenRejectsUnsupportedAndOversizedInput(t *testing.T) {
	for _, token := range []string{"wat1:not base64", "wat1:" + strings.Repeat("A", 20000), "wat1:Z3JlfDE5Mi4wLjIuMXxzZWNyZXR8fHx8NDQz"} {
		if _, _, err := DecodeToken(token); err == nil {
			t.Fatal("accepted invalid token")
		}
	}
}

func FuzzDecodeToken(f *testing.F) {
	f.Add("wat1:dGNwfGV4YW1wbGUuY29tOjQ0NDN8c2VjcmV0fHx8fDQ0Mw==")
	f.Fuzz(func(t *testing.T, raw string) { _, _, _ = DecodeToken(raw) })
}

func TestAutoTokenKeepsExplicitDirectionAndCandidateSet(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		c := Config{Role: "server", Carrier: "auto", Endpoint: "example.com", Reverse: reverse, ReversePeer: "client.example.com", Candidates: []CarrierEndpoint{{"tcp", 4443}, {"wss", 2054}}, Ports: []Port{{8443, 443, "tcp"}}}
		token, err := EncodeToken(c, "key")
		if err != nil {
			t.Fatal(err)
		}
		decoded, _, err := DecodeToken(token)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Reverse != reverse || len(decoded.Candidates) != 2 || decoded.Candidates[0].Carrier != "tcp" || decoded.Candidates[1].Port != 2054 {
			t.Fatalf("changed Auto plan: %+v", decoded)
		}
	}
}

func TestSignedReleaseBinary(t *testing.T) {
	if os.Getenv("WATTUN_E2E_VERIFY") != "1" {
		t.Skip("requires the signed native release")
	}
	binary := os.Getenv("WATTUN_E2E_BINARY")
	if err := VerifyBinary(binary); err != nil {
		t.Fatal(err)
	}
}
