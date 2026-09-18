package sub

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const clashExternalVlessLink = "vless://22222222-2222-4222-8222-222222222222@198.51.100.9:443?type=tcp&security=reality&sni=example.com&pbk=test-public-key&sid=ab12&fp=chrome&flow=xtls-rprx-vision"

func TestClashExternalVlessEncryption(t *testing.T) {
	svc := NewSubClashService(false, "", &SubService{})
	for _, tc := range []struct {
		name  string
		query string
		want  string
	}{
		{"encrypted", "&encryption=" + url.QueryEscape(testMlkemEncryption), testMlkemEncryption},
		{"trimmed", "&encryption=" + url.QueryEscape(" \t"+testMlkemEncryption+" \n"), testMlkemEncryption},
		{"none", "&encryption=none", ""},
		{"trimmed none", "&encryption=%20none%20", ""},
		{"empty", "&encryption=", ""},
		{"whitespace", "&encryption=%20%09", ""},
		{"missing", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy := svc.clashProxyFromExternal(clashExternalVlessLink+tc.query+"#external", "external")
			if proxy == nil {
				t.Fatal("expected a VLESS proxy")
			}
			if got, exists := proxy["encryption"]; tc.want == "" {
				if exists {
					t.Errorf("plain VLESS encryption should be omitted, got %v", got)
				}
			} else if got != tc.want {
				t.Errorf("encryption = %v, want %q", got, tc.want)
			}
			for key, want := range map[string]any{
				"type": "vless", "uuid": "22222222-2222-4222-8222-222222222222",
				"server": "198.51.100.9", "port": 443, "network": "tcp",
				"flow": "xtls-rprx-vision", "tls": true, "servername": "example.com",
			} {
				if got := proxy[key]; got != want {
					t.Errorf("%s = %v, want %v", key, got, want)
				}
			}
			if _, exists := proxy["packet-encoding"]; exists {
				t.Error("VLESS encryption must not be exported as packet-encoding")
			}
		})
	}
}

// Regression for #6572: both pasted links and fetched subscriptions must keep
// encryption when their nodes are merged with the client's local inbound.
func TestClashMergedExternalVlessEncryption(t *testing.T) {
	link := clashExternalVlessLink + "&encryption=" + url.QueryEscape(testMlkemEncryption) + "#external"
	for _, source := range []string{"link", "subscription-plain", "subscription-base64"} {
		t.Run(source, func(t *testing.T) {
			seedSubDB(t)
			resetSubscriptionCache(t)
			seedSubInbound(t, "merged-vless", "local", 10001, 1, wsTLSStream)
			db := database.GetDB()
			var client model.ClientRecord
			if err := db.Where("email = ?", "local@e").First(&client).Error; err != nil {
				t.Fatal(err)
			}
			entry := model.ClientExternalLink{ClientId: client.Id, Kind: model.ExternalLinkKindLink, Value: link}
			if source != "link" {
				body := link + "\n"
				if source == "subscription-base64" {
					body = base64.StdEncoding.EncodeToString([]byte(body))
				}
				srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(body))
				}))
				defer srv.Close()
				previousClient := subscriptionHTTPClient
				subscriptionHTTPClient = srv.Client()
				t.Cleanup(func() { subscriptionHTTPClient = previousClient })
				entry.Kind = model.ExternalLinkKindSubscription
				entry.Value = srv.URL
			}
			if err := db.Create(&entry).Error; err != nil {
				t.Fatal(err)
			}

			w := fetchClashSub(t, clashSubRouter(t), "/clash/merged-vless?view=raw")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}
			var config struct {
				Proxies []struct {
					UUID       string `yaml:"uuid"`
					Encryption string `yaml:"encryption"`
				} `yaml:"proxies"`
			}
			if err := yaml.Unmarshal(w.Body.Bytes(), &config); err != nil {
				t.Fatalf("decode Clash YAML: %v", err)
			}
			if len(config.Proxies) != 2 {
				t.Fatalf("expected local and external proxies, got %#v", config.Proxies)
			}
			want := map[string]string{client.UUID: "", "22222222-2222-4222-8222-222222222222": testMlkemEncryption}
			for _, proxy := range config.Proxies {
				encryption, ok := want[proxy.UUID]
				if !ok || proxy.Encryption != encryption {
					t.Errorf("unexpected proxy: %#v", proxy)
				}
				delete(want, proxy.UUID)
			}
			if len(want) != 0 {
				t.Errorf("missing proxies: %v", want)
			}
		})
	}
}
