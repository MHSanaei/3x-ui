package sub

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestOutboundShareLinkCarriesRealitySettings(t *testing.T) {
	raw := map[string]any{
		"tag":      "remote",
		"protocol": "trojan",
		"settings": map[string]any{
			"servers": []any{map[string]any{
				"address":  "37.27.201.56",
				"port":     float64(8443),
				"password": "provider-password",
			}},
		},
		"streamSettings": map[string]any{
			"network":  "tcp",
			"security": "reality",
			"realitySettings": map[string]any{
				"serverName":    "aws.amazon.com",
				"fingerprint":   "chrome",
				"shortId":       "298b44",
				"spiderX":       "/Y3a7bZe6Zos7IBq",
				"publicKey":     "jcUMXf_ViK5nuhF6KzBVaFG6zG1qvBwXmqXR_3MYqzU",
				"mldsa65Verify": "verify-value",
			},
		},
	}

	link := NewSubService(false, "-io").outboundShareLink(raw, model.ClientRecord{
		Email:    "alice@example.com",
		Password: "client-password",
	})
	if !strings.HasPrefix(link, "trojan://provider-password@37.27.201.56:8443?") {
		t.Fatalf("link did not preserve outbound credentials/endpoint: %s", link)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	q := parsed.Query()
	want := map[string]string{
		"type":     "tcp",
		"security": "reality",
		"sni":      "aws.amazon.com",
		"fp":       "chrome",
		"sid":      "298b44",
		"spx":      "/Y3a7bZe6Zos7IBq",
		"pbk":      "jcUMXf_ViK5nuhF6KzBVaFG6zG1qvBwXmqXR_3MYqzU",
		"pqv":      "verify-value",
	}
	for key, value := range want {
		if got := q.Get(key); got != value {
			t.Fatalf("query %s = %q, want %q (link %s)", key, got, value, link)
		}
	}
}

func TestPersonalizedOutboundConfigPreservesOutboundCredentials(t *testing.T) {
	raw := map[string]any{
		"tag":                  "remote",
		"protocol":             "trojan",
		"clientExternalConfig": true,
		"_source":              "panel",
		"settings": map[string]any{
			"servers": []any{map[string]any{
				"address":  "37.27.201.56",
				"port":     float64(8443),
				"password": "provider-password",
			}},
		},
		"streamSettings": map[string]any{
			"security": "reality",
			"realitySettings": map[string]any{
				"serverName":  "aws.amazon.com",
				"fingerprint": "chrome",
			},
		},
	}

	data := personalizedOutboundConfig(raw, model.ClientRecord{
		Email:    "alice@example.com",
		Password: "client-password",
	})
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal personalized outbound: %v", err)
	}
	if got["clientExternalConfig"] != nil || got["_source"] != nil {
		t.Fatalf("internal metadata was not stripped: %#v", got)
	}
	settings, _ := got["settings"].(map[string]any)
	servers, _ := settings["servers"].([]any)
	server, _ := servers[0].(map[string]any)
	if password := server["password"]; password != "provider-password" {
		t.Fatalf("password = %q, want provider-password", password)
	}
	stream, _ := got["streamSettings"].(map[string]any)
	reality, _ := stream["realitySettings"].(map[string]any)
	if sni := reality["serverName"]; sni != "aws.amazon.com" {
		t.Fatalf("serverName = %q, want aws.amazon.com", sni)
	}
}
