package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
)

// TestValidateOutboundConfig_RejectsUnencryptedPublicVless covers xray-core
// v26.7.11's refusal to build an unencrypted vless outbound to a public
// address — the check now runs in-process, so the panel can surface it before
// a config reaches the core and bricks startup. A private-address outbound and
// a TLS outbound stay valid.
func TestValidateOutboundConfig_RejectsUnencryptedPublicVless(t *testing.T) {
	publicPlaintext := `{
		"protocol": "vless",
		"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "security": "none"}
	}`
	if err := ValidateOutboundConfig([]byte(publicPlaintext)); err == nil {
		t.Fatal("expected a public unencrypted vless outbound to be rejected")
	} else if !strings.Contains(err.Error(), "prohibited") {
		t.Fatalf("expected a prohibition error, got: %v", err)
	}

	privatePlaintext := `{
		"protocol": "vless",
		"settings": {"address": "10.0.0.1", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "security": "none"}
	}`
	if err := ValidateOutboundConfig([]byte(privatePlaintext)); err != nil {
		t.Fatalf("a private-address plaintext vless outbound must stay valid, got: %v", err)
	}

	publicTLS := `{
		"protocol": "vless",
		"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "security": "tls", "tlsSettings": {"serverName": "example.com"}}
	}`
	if err := ValidateOutboundConfig([]byte(publicTLS)); err != nil {
		t.Fatalf("a TLS-secured public vless outbound must stay valid, got: %v", err)
	}
}

// The core feeds remoteDNS to netip.MustParseAddr when it creates the outbound, so a
// non-IP entry ("local" until 26.9.30) panics it at startup; conf.Build() lets it through.
func TestValidateOutboundConfig_RejectsWireguardRemoteDNSThatIsNotAnIP(t *testing.T) {
	outbound := func(remoteDNS string) []byte {
		return []byte(`{
			"protocol": "wireguard",
			"settings": {
				"secretKey": "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=",
				"address": ["10.0.0.2/32"],
				"peers": [{"publicKey": "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=", "endpoint": "162.159.192.1:2408"}],
				"remoteDNS": ` + remoteDNS + `
			}
		}`)
	}
	for _, rejected := range []string{`["local"]`, `["1.1.1.1", "dns.google"]`} {
		err := ValidateOutboundConfig(outbound(rejected))
		if err == nil || !strings.Contains(err.Error(), "remoteDNS") {
			t.Errorf("remoteDNS %s: want a remoteDNS refusal, got %v", rejected, err)
		}
	}
	if err := ValidateOutboundConfig(outbound(`["1.1.1.1", "2606:4700:4700::1111"]`)); err != nil {
		t.Fatalf("IP remoteDNS entries must stay valid, got: %v", err)
	}
}

// The core's XHTTP dialer dereferences downloadSettings' address and casts its transport to
// XHTTP on every dial, so a block lacking either panics the whole core; conf.Build() lets it through.
func TestValidateOutboundConfig_RejectsXhttpDownloadSettingsTheDialerPanicsOn(t *testing.T) {
	outbound := func(network, xhttpSettings string) []byte {
		return []byte(`{
			"protocol": "vless",
			"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
			"streamSettings": {"network": "` + network + `", "security": "tls", "xhttpSettings": ` + xhttpSettings + `}
		}`)
	}
	for _, rejected := range []string{
		`{"downloadSettings": {}}`,
		`{"downloadSettings": {"port": 443, "network": "xhttp"}}`,
		`{"downloadSettings": {"address": null, "port": 443, "network": "xhttp"}}`,
		`{"downloadSettings": {"address": "5.6.7.8", "port": 443}}`,
		`{"downloadSettings": {"address": "5.6.7.8", "port": 443, "network": "tcp"}}`,
		`{"downloadSettings": {"address": "5.6.7.8", "port": 443, "method": "ws"}}`,
		`{"DOWNLOADSETTINGS": {}}`,
	} {
		err := ValidateOutboundConfig(outbound("xhttp", rejected))
		if err == nil || !strings.Contains(err.Error(), "downloadSettings") {
			t.Errorf("xhttpSettings %s: want a downloadSettings refusal, got %v", rejected, err)
		}
	}
	for _, accepted := range [][]byte{
		outbound("xhttp", `{"path": "/up"}`),
		outbound("xhttp", `{"downloadSettings": {"address": "5.6.7.8", "port": 443, "network": "xhttp"}}`),
		outbound("xhttp", `{"downloadSettings": {"address": "down.example", "port": 443, "network": "splithttp", "security": "tls", "xhttpSettings": {"path": "/down"}}}`),
		outbound("tcp", `{"downloadSettings": {}}`),
	} {
		if err := ValidateOutboundConfig(accepted); err != nil {
			t.Errorf("outbound %s must stay valid, got: %v", accepted, err)
		}
	}
}

// xray-core 26.10.17 compiles a noise mask's Lua script inside Build (#7124), where the panel would
// look beside its own binary; the xray child finds a relative script in its bin folder.
func TestValidateOutboundConfig_FindsANoiseScriptBesideTheXrayBinary(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", bin)
	t.Chdir(t.TempDir())
	if err := os.WriteFile(filepath.Join(bin, "noise.lua"), []byte("function HandleNoise(plan) end\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}
	outbound := []byte(`{
		"protocol": "vless",
		"settings": {"address": "10.0.0.1", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
		"streamSettings": {"network": "tcp", "finalmask": {"udp": [{"type": "noise", "settings": {"script": "noise.lua"}}]}}
	}`)
	if err := ValidateOutboundConfig(outbound); err != nil {
		t.Fatalf("ValidateOutboundConfig = %v, want the script beside the xray binary found", err)
	}
}

// The core's loader panics on some third-party values instead of returning an error;
// a subscription refresh must count that link as rejected, not unwind the whole job.
func TestValidateOutboundConfig_ReturnsTheLoaderPanicAsAnError(t *testing.T) {
	for name, outbound := range map[string]string{
		"vless encryption": `{"protocol": "vless",
			"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "mlkem768x25519plus.native.0rtt.AAAA"},
			"streamSettings": {"network": "tcp", "security": "tls", "tlsSettings": {"serverName": "example.com"}}}`,
		"reality spiderX": `{"protocol": "vless",
			"settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"},
			"streamSettings": {"network": "tcp", "security": "reality", "realitySettings": {
				"serverName": "example.com", "fingerprint": "chrome", "spiderX": "/%zz", "publicKey": "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg"}}}`,
	} {
		if err := ValidateOutboundConfig([]byte(outbound)); err == nil || !strings.Contains(err.Error(), "panicked") {
			t.Fatalf("%s: ValidateOutboundConfig = %v, want the loader's panic as an error", name, err)
		}
	}
}

// conf.Build() lets these through, but the handler constructor parses them at core
// start, and one failure there takes every inbound down with it.
func TestValidateOutboundConfig_RejectsLinksTheHandlerConstructorRefuses(t *testing.T) {
	const wg = "wireguard://JOSLNr3Kk8BViwrrTJ2M19gRBbGttnRfApY3nQ3fdT8%3D@203.0.113.10:51820?"
	const wgPeer = "publickey=biSnuvUOX2pmtzf249FK8UGSq2PQslGSdoj4UqH4yew%3D"
	const vless = "vless://b831381d-6324-4d53-ad4f-8cda48b30811@203.0.113.10:443?security=tls&sni=a&encryption=mlkem768x25519plus.native.1rtt."
	x25519 := strings.Repeat("A", 43)
	// ss userinfo: "2022-blake3-aes-128-gcm:shortkey", "hello world" (method guessed), a 16-byte key
	const ssShort = "ss://MjAyMi1ibGFrZTMtYWVzLTEyOC1nY206c2hvcnRrZXk@203.0.113.10:8388"
	const ssGuessed = "ss://aGVsbG8gd29ybGQ@203.0.113.10:8388"
	const ssValid = "ss://MjAyMi1ibGFrZTMtYWVzLTEyOC1nY206QndjSEJ3Y0hCd2NIQndjSEJ3Y0hCdz09@203.0.113.10:8388"
	for _, tc := range []struct{ name, link, wantErr string }{
		{"wireguard address", wg + wgPeer + "&address=garbage", `address entry "garbage"`},
		{"wireguard no public key", wg + "address=10.0.0.2/32", "peer without publicKey"},
		{"wireguard valid", wg + wgPeer + "&address=10.0.0.2/32", ""},
		{"vless padding", vless + "abc." + x25519, "vless encryption"},
		{"vless mlkem key", vless + strings.Repeat("_", 1579), "vless encryption"},
		{"vless valid", vless + x25519, ""},
		{"ss2022 short key", ssShort, "shadowsocks 2022 key"},
		{"ss guessed 2022 method", ssGuessed, "shadowsocks 2022 key"},
		{"ss2022 valid", ssValid, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := link.ParseLink(tc.link)
			if err != nil {
				t.Fatalf("ParseLink: %v", err)
			}
			parsed.Outbound["tag"] = "t"
			raw, _ := json.Marshal(parsed.Outbound)
			err = ValidateOutboundConfig(raw)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOutboundConfig(%s) = %v, want nil", raw, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateOutboundConfig(%s) = %v, want an error containing %q", raw, err, tc.wantErr)
			}
		})
	}
}

// conf.Build() runs inside the panel; these values make it allocate or loop without
// bound, so they must be refused before it, whoever supplied the outbound.
func TestValidateOutboundConfig_RefusesWhatTheLoaderWouldExpandWithoutBound(t *testing.T) {
	big := filepath.Join(t.TempDir(), "big.pem")
	if err := os.WriteFile(big, make([]byte, maxCertFileBytes+1), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	vless := func(stream string) string {
		return `{"protocol": "vless", "settings": {"address": "1.2.3.4", "port": 443, "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "encryption": "none"}, "streamSettings": ` + stream + `}`
	}
	hop := `{"type": "udphop", "settings": {"mode": "intervalRemote", "remotePorts": "0-65535"}}`
	masks := strings.TrimSuffix(strings.Repeat(`{"type": "noise"},`, maxStreamMasks+1), ",")
	for name, tc := range map[string]struct{ outbound, wantErr string }{
		"xhttp sessionIDLength in extra":  {vless(`{"network": "xhttp", "security": "tls", "xhttpSettings": {"extra": {"sessionIDTable": "abcdefgh", "sessionIDLength": "2147483647"}}}`), "sessionIDLength"},
		"nested download sessionIDLength": {vless(`{"network": "xhttp", "security": "tls", "xhttpSettings": {"downloadSettings": {"network": "xhttp", "xhttpSettings": {"sessionIDTable": "ab", "sessionIDLength": "64-100000"}}}}`), "sessionIDLength"},
		"udphop ports across masks":       {vless(`{"network": "tcp", "security": "tls", "finalmask": {"udp": [` + hop + `,` + hop + `]}}`), "udphop"},
		"udphop bare port":                {vless(`{"network": "tcp", "security": "tls", "finalmask": {"UDP": [{"type": "UDPHOP", "settings": {"mode": "intervalRemote", "remotePorts": 4294967295}}]}}`), "udphop"},
		"too many masks":                  {vless(`{"network": "tcp", "security": "tls", "finalmask": {"tcp": [` + masks + `]}}`), "masks"},
		"certificate is a directory":      {vless(`{"network": "tcp", "security": "tls", "tlsSettings": {"certificates": [{"CertificateFile": "` + filepath.ToSlash(t.TempDir()) + `"}]}}`), "regular file"},
		"oversized key file":              {vless(`{"network": "tcp", "security": "tls", "tlsSettings": {"certificates": [{"keyFile": "` + filepath.ToSlash(big) + `"}]}}`), "regular file"},
	} {
		if err := ValidateOutboundConfig([]byte(tc.outbound)); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: ValidateOutboundConfig = %v, want an error naming %q", name, err, tc.wantErr)
		}
	}
}

func TestCountXMCMasksFollowsTheStreamsTheCoreBuilds(t *testing.T) {
	stream := `{"finalmask": {"tcp": [{"type": "XMC"}, {"type": "noise"}]}, "xhttpSettings": {"extra": {"downloadSettings": {"finalmask": {"tcp": [{"type": "xmc"}]}}}}}`
	if got := CountXMCMasks([]byte(stream)); got != 2 {
		t.Fatalf("CountXMCMasks = %d, want 2 (one top-level, one in the download stream inside extra)", got)
	}
}
