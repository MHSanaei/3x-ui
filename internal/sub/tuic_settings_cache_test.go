package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const (
	subTuicUUID     = "11111111-2222-4333-8444-555555555578"
	subTuicPassword = "p@ss:word/?%#"
)

func seedCachedTuicSubscription(t *testing.T, identityKey string) {
	t.Helper()
	seedSubDB(t)
	settings := fmt.Sprintf(`{"server":{"congestion_control":"cubic","alpn":["h3"],"sni":"tuic.example.test","udp_relay_mode":"quic"},"clients":[
		{%q:%q,"password":%q,"email":"tuic@e","subId":"tuic-sub","enable":true},
		{%q:"11111111-2222-4333-8444-555555555579","password":"other-secret","email":"other@e","subId":"other-sub","enable":true},
		{%q:"11111111-2222-4333-8444-555555555580","password":"disabled-secret","email":"disabled@e","subId":"tuic-sub","enable":false}
	]}`, identityKey, subTuicUUID, subTuicPassword, identityKey, identityKey)
	inbound := &model.Inbound{
		UserId: 1, Tag: "tuic-cache", Protocol: model.TUIC, Enable: true,
		Listen: "203.0.113.78", Port: 18478, Remark: "TUIC", Settings: settings,
		StreamSettings: `{}`, SubSortIndex: 1,
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	clients, err := (&service.InboundService{}).GetClients(inbound)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&service.ClientService{}).SyncInbound(nil, inbound.Id, clients); err != nil {
		t.Fatal(err)
	}
	seedSubProtocolInbound(t, "tuic-sub", "vless-cache", 18479, 2, `{"network":"tcp","security":"none"}`, model.VLESS)
}

func TestTuicRawSubscriptionSurvivesSettingsCache(t *testing.T) {
	for _, identityKey := range []string{"id", "uuid"} {
		for _, encrypted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/base64=%t", identityKey, encrypted), func(t *testing.T) {
				seedCachedTuicSubscription(t, identityKey)
				router := gin.New()
				controller := NewSUBController(router.Group("/"), WithSUBEncryption(encrypted))
				t.Cleanup(controller.Close)
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = httptest.NewRequest(http.MethodGet, "http://sub.example.test/sub/tuic-sub", nil)
				ctx.Params = gin.Params{{Key: "subid", Value: "tuic-sub"}}
				page, ok := controller.buildSubPageData(ctx)
				if !ok || len(page.Result) != 2 || !strings.HasPrefix(page.Result[0], "tuic://") {
					t.Fatalf("browser page lost TUIC: %+v", page.Result)
				}
				for _, userAgent := range []string{"v2rayN", "Shadowrocket"} {
					req := httptest.NewRequest(http.MethodGet, "http://sub.example.test/sub/tuic-sub", nil)
					req.Header.Set("User-Agent", userAgent)
					response := httptest.NewRecorder()
					router.ServeHTTP(response, req)
					if response.Code != http.StatusOK {
						t.Fatalf("%s status=%d body=%s", userAgent, response.Code, response.Body.String())
					}
					body := response.Body.String()
					if encrypted {
						decoded, err := base64.StdEncoding.DecodeString(body)
						if err != nil {
							t.Fatal(err)
						}
						body = string(decoded)
					}
					lines := strings.Fields(body)
					if len(lines) != 2 || !strings.HasPrefix(lines[1], "vless://") {
						t.Fatalf("%s subscription must retain TUIC and VLESS only: %s", userAgent, body)
					}
					u, err := url.Parse(lines[0])
					if err != nil {
						t.Fatal(err)
					}
					password, _ := u.User.Password()
					if u.Scheme != "tuic" || u.Host != "203.0.113.78:18478" || u.User.Username() != subTuicUUID || password != subTuicPassword {
						t.Fatalf("%s TUIC endpoint or credentials changed: %s", userAgent, lines[0])
					}
					want := url.Values{"congestion_control": {"cubic"}, "alpn": {"h3"}, "sni": {"tuic.example.test"}, "udp_relay_mode": {"quic"}, "allow_insecure": {"0"}}
					if !reflect.DeepEqual(u.Query(), want) {
						t.Fatalf("TUIC options=%v, want %v", u.Query(), want)
					}
				}
			})
		}
	}
}

func TestTuicCachedClashKeepsLegacyUUIDFallback(t *testing.T) {
	seedCachedTuicSubscription(t, "uuid")
	router := gin.New()
	controller := NewSUBController(router.Group("/"), WithSUBClashEnabled(true))
	t.Cleanup(controller.Close)
	for round := range 2 {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.test/clash/tuic-sub", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("round %d status=%d body=%s", round, response.Code, response.Body.String())
		}
		var profile struct {
			Proxies []struct {
				Type       string   `yaml:"type"`
				Server     string   `yaml:"server"`
				Port       int      `yaml:"port"`
				UUID       string   `yaml:"uuid"`
				Password   string   `yaml:"password"`
				SNI        string   `yaml:"sni"`
				ALPN       []string `yaml:"alpn"`
				Congestion string   `yaml:"congestion-controller"`
				UDPMode    string   `yaml:"udp-relay-mode"`
			} `yaml:"proxies"`
		}
		if err := yaml.Unmarshal(response.Body.Bytes(), &profile); err != nil {
			t.Fatal(err)
		}
		if len(profile.Proxies) != 2 || profile.Proxies[0].Type != "tuic" || profile.Proxies[1].Type != "vless" {
			t.Fatalf("round %d proxies=%+v", round, profile.Proxies)
		}
		proxy := profile.Proxies[0]
		if proxy.UUID != subTuicUUID || proxy.Password != subTuicPassword || proxy.Server != "203.0.113.78" || proxy.Port != 18478 || proxy.SNI != "tuic.example.test" || !reflect.DeepEqual(proxy.ALPN, []string{"h3"}) || proxy.Congestion != "cubic" || proxy.UDPMode != "quic" {
			t.Fatalf("round %d TUIC proxy=%+v", round, proxy)
		}
	}
}
