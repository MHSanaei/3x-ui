package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedSS2022Inbound(t *testing.T, subId, tag string, port int, srvKey, cliKey string) *model.Inbound {
	t.Helper()
	db := database.GetDB()
	email := tag + "@e"
	settings := fmt.Sprintf(
		`{"method":"2022-blake3-aes-256-gcm","password":%q,"network":"tcp,udp","clients":[{"email":%q,"password":%q,"subId":%q,"enable":true}]}`,
		srvKey, email, cliKey, subId,
	)
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: model.Shadowsocks, Remark: tag, Settings: settings,
		StreamSettings: `{"network":"tcp","security":"none"}`,
		SubSortIndex:   2,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed ss2022 inbound %s: %v", tag, err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Password: cliKey, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed ss2022 client %s: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed ss2022 client_inbound %s: %v", email, err)
	}
	return ib
}

func decodeSubBody(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resp.Body.String()))
	if err != nil {
		t.Fatalf("base64 decode sub body: %v\nraw=%q", err, resp.Body.String())
	}
	return strings.TrimSpace(string(raw))
}

func TestShadowrocketSS2022SubscriptionFormats(t *testing.T) {
	seedSubDB(t)
	gin.SetMode(gin.TestMode)

	const (
		subId  = "sr-ss2022"
		srvKey = "YctPZ6U7xPPcU+gp3u+0tx/tRizJN9K8y+uKlW2qjlI="
		cliKey = "AbC123+dEf456/GhI789+jKl012/MnO345+pQr67890="
	)
	seedSS2022Inbound(t, subId, "HK-11702", 11702, srvKey, cliKey)

	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{
		clashAutoDetect: true,
	})

	t.Run("v2rayNG keeps SIP022 percent-encoded", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/"+subId, nil)
		req.Header.Set("User-Agent", "v2rayNG/1.8.5")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		body := decodeSubBody(t, resp)
		if !strings.Contains(body, "%3D") || !strings.Contains(body, "%2F") || !strings.Contains(body, "%2B") {
			t.Fatalf("v2rayNG should keep SIP022 percent-encoding, got:\n%s", body)
		}
		parsed, err := link.ParseLink(body)
		if err != nil {
			t.Fatalf("ParseLink(v2rayNG): %v", err)
		}
		srv := parsed.Outbound["settings"].(map[string]any)["servers"].([]any)[0].(map[string]any)
		if srv["password"] != srvKey+":"+cliKey {
			t.Fatalf("decoded password = %q, want %q", srv["password"], srvKey+":"+cliKey)
		}
	})

	for _, tc := range []struct {
		name      string
		path      string
		userAgent string
	}{
		{
			name:      "Shadowrocket UA uses SIP002 base64url without percent-escaped keys",
			path:      "/sub/" + subId,
			userAgent: "Shadowrocket/2.2.58 (iPhone; iOS 17.5; Scale/3.00)",
		},
		{
			name:      "flag=shadowrocket uses SIP002 base64url",
			path:      "/sub/" + subId + "?flag=shadowrocket",
			userAgent: "CFNetwork/1496.0.7 Darwin/23.5.0",
		},
		{
			name:      "explicit ss2022=sip002 query param",
			path:      "/sub/" + subId + "?ss2022=sip002",
			userAgent: "curl/8.0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://sub.example.com"+tc.path, nil)
			req.Header.Set("User-Agent", tc.userAgent)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			body := decodeSubBody(t, resp)
			if strings.Contains(body, "proxies:") {
				t.Fatalf("Shadowrocket must receive raw links, not Clash YAML:\n%s", body)
			}
			for _, bad := range []string{"%3D", "%2F", "%2B", "%3d", "%2f", "%2b"} {
				if strings.Contains(body, bad) {
					t.Fatalf("Shadowrocket link must not contain %q:\n%s", bad, body)
				}
			}
			u, err := url.Parse(body)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", body, err)
			}
			if u.Scheme != "ss" || u.Hostname() != "203.0.113.5" || u.Port() != "11702" {
				t.Fatalf("parsed URL = scheme:%q host:%q port:%q from %q", u.Scheme, u.Hostname(), u.Port(), body)
			}
			if !strings.Contains(u.Fragment, "HK-11702") {
				t.Fatalf("remark fragment lost: %q in %q", u.Fragment, body)
			}
			rawUserinfo, err := base64.RawURLEncoding.DecodeString(u.User.Username())
			if err != nil {
				t.Fatalf("decode SIP002 userinfo %q: %v", u.User.Username(), err)
			}
			wantUserinfo := "2022-blake3-aes-256-gcm:" + srvKey + ":" + cliKey
			if string(rawUserinfo) != wantUserinfo {
				t.Fatalf("userinfo = %q, want %q", string(rawUserinfo), wantUserinfo)
			}
			parsed, err := link.ParseLink(body)
			if err != nil {
				t.Fatalf("ParseLink(Shadowrocket): %v", err)
			}
			srv := parsed.Outbound["settings"].(map[string]any)["servers"].([]any)[0].(map[string]any)
			if srv["password"] != srvKey+":"+cliKey {
				t.Fatalf("round-trip password = %q, want %q", srv["password"], srvKey+":"+cliKey)
			}
		})
	}

	t.Run("ss2022=raw emits unescaped keys and flag=shadowrocket bypasses clash regex", func(t *testing.T) {
		clashRegexRouter := newSubscriptionTestRouter(subscriptionTestRouterConfig{
			clashAutoDetect:     true,
			clashUserAgentRegex: `(?i)(clash|mihomo|shadowrocket)`,
		})
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/"+subId+"?ss2022=raw", nil)
		req.Header.Set("User-Agent", "Shadowrocket/2.2.58")
		resp := httptest.NewRecorder()
		clashRegexRouter.ServeHTTP(resp, req)

		body := decodeSubBody(t, resp)
		wantPrefix := "ss://2022-blake3-aes-256-gcm:" + srvKey + ":" + cliKey + "@203.0.113.5:11702"
		if !strings.HasPrefix(body, wantPrefix) {
			t.Fatalf("raw format mismatch:\n got: %q\nwant prefix: %q", body, wantPrefix)
		}
		for _, bad := range []string{"%3D", "%2F", "%2B"} {
			if strings.Contains(body, bad) {
				t.Fatalf("raw link must not contain %q:\n%s", bad, body)
			}
		}
	})
}

func TestHiddenOrAllExcludedHostSuppressesInboundFallbackAcrossFormats(t *testing.T) {
	modes := []string{"inbound-first", "hosts-first"}
	cases := []struct {
		name      string
		setupHost func(t *testing.T, relayID int)
	}{
		{
			name: "is_hidden true",
			setupHost: func(t *testing.T, relayID int) {
				seedHost(t, &model.Host{
					InboundId: relayID,
					GroupId:   "relay-hidden",
					SortOrder: 1,
					Remark:    "GZ-Relay",
					Address:   "relay.example.com",
					IsHidden:  true,
				})
			},
		},
		{
			name: "exclude_from_sub_types all formats",
			setupHost: func(t *testing.T, relayID int) {
				seedHost(t, &model.Host{
					InboundId:           relayID,
					GroupId:             "relay-excl",
					SortOrder:           1,
					Remark:              "GZ-Relay",
					Address:             "relay.example.com",
					ExcludeFromSubTypes: []string{"raw", "json", "clash"},
				})
			},
		},
	}

	for _, mode := range modes {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				seedSubDB(t)
				setHostSortMode(t, mode)
				db := database.GetDB()

				const (
					subId  = "relay-sub"
					srvKey = "YctPZ6U7xPPcU+gp3u+0tx/tRizJN9K8y+uKlW2qjlI="
					cliKey = "AbC123+dEf456/GhI789+jKl012/MnO345+pQr67890="
				)
				relay := seedSubInbound(t, subId, "GZ-Relay", 11701, 1, `{"network":"tcp","security":"none"}`)
				exit := seedSS2022Inbound(t, subId, "HK-11702", 11702, srvKey, cliKey)
				tc.setupHost(t, relay.Id)

				for _, row := range []*xray.ClientTraffic{
					{InboundId: relay.Id, Email: "GZ-Relay@e", Up: 1000, Down: 2000, Enable: true},
					{InboundId: exit.Id, Email: "HK-11702@e", Up: 300, Down: 400, Enable: true},
				} {
					if err := db.Create(row).Error; err != nil {
						t.Fatalf("seed traffic %s: %v", row.Email, err)
					}
				}

				const wantHeader = "upload=1300; download=2400; "

				links, _, _, traffic, err := NewSubService("").GetSubs(subId, "sub.example.com")
				if err != nil {
					t.Fatalf("GetSubs: %v", err)
				}
				rawJoined := strings.Join(links, "\n")
				if strings.Contains(rawJoined, "11701") || strings.Contains(rawJoined, "relay.example.com") {
					t.Fatalf("raw sub leaked hidden relay 11701:\n%s", rawJoined)
				}
				if !strings.Contains(rawJoined, "11702") {
					t.Fatalf("raw sub lost 11702:\n%s", rawJoined)
				}
				if traffic.Up != 1300 || traffic.Down != 2400 {
					t.Fatalf("raw traffic = %d/%d, want 1300/2400", traffic.Up, traffic.Down)
				}

				clashBody, clashHeader, err := NewSubClashService(false, "", false, NewSubService("")).GetClash(subId, "sub.example.com")
				if err != nil {
					t.Fatalf("GetClash: %v", err)
				}
				if strings.Contains(clashBody, "11701") || strings.Contains(clashBody, "relay.example.com") {
					t.Fatalf("clash sub leaked hidden relay 11701:\n%s", clashBody)
				}
				if !strings.Contains(clashBody, "11702") {
					t.Fatalf("clash sub lost 11702:\n%s", clashBody)
				}
				if !strings.HasPrefix(clashHeader, wantHeader) {
					t.Fatalf("clash header = %q, want prefix %q", clashHeader, wantHeader)
				}

				jsonBody, jsonHeader, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson(subId, "sub.example.com", false)
				if err != nil {
					t.Fatalf("GetJson: %v", err)
				}
				if strings.Contains(jsonBody, "11701") || strings.Contains(jsonBody, "relay.example.com") {
					t.Fatalf("json sub leaked hidden relay 11701:\n%s", jsonBody)
				}
				if !strings.Contains(jsonBody, "11702") {
					t.Fatalf("json sub lost 11702:\n%s", jsonBody)
				}
				if !strings.HasPrefix(jsonHeader, wantHeader) {
					t.Fatalf("json header = %q, want prefix %q", jsonHeader, wantHeader)
				}

				if got := NewLinkProvider().LinksForClient("sub.example.com", relay, "GZ-Relay@e"); len(got) != 0 {
					t.Fatalf("LinksForClient must hide relay 11701, got %v", got)
				}
				svc := NewSubService("")
				svc.PrepareForRequest("sub.example.com")
				if got := svc.inboundLinks(relay); len(got) != 0 {
					t.Fatalf("inboundLinks must hide relay 11701, got %v", got)
				}
			})
		}
	}
}

func TestHiddenHostAlongsideVisibleHostAndAllHiddenHTTP200(t *testing.T) {
	seedSubDB(t)
	gin.SetMode(gin.TestMode)
	db := database.GetDB()

	// Mixed visible + hidden host on one inbound: only visible host renders.
	mixed := seedSubInbound(t, "mix-sub", "mixed", 11801, 1, `{"network":"tcp","security":"none"}`)
	seedHost(t, &model.Host{InboundId: mixed.Id, GroupId: "vis", SortOrder: 1, Remark: "VIS", Address: "visible.example.com"})
	seedHost(t, &model.Host{InboundId: mixed.Id, GroupId: "hid", SortOrder: 2, Remark: "HID", Address: "hidden.example.com", IsHidden: true})

	links, _, _, _, err := NewSubService("").GetSubs("mix-sub", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs(mix-sub): %v", err)
	}
	joined := strings.Join(links, "\n")
	if !strings.Contains(joined, "visible.example.com") || strings.Contains(joined, "hidden.example.com") {
		t.Fatalf("mixed hosts output wrong:\n%s", joined)
	}

	// Only hidden inbound for a subscriber: HTTP /sub/ returns 200 with empty body + userinfo header.
	onlyHidden := seedSubInbound(t, "only-hidden", "relay-only", 11701, 1, `{"network":"tcp","security":"none"}`)
	seedHost(t, &model.Host{InboundId: onlyHidden.Id, GroupId: "only-hid", SortOrder: 1, Remark: "H", Address: "relay.example.com", IsHidden: true})
	if err := db.Create(&xray.ClientTraffic{InboundId: onlyHidden.Id, Email: "relay-only@e", Up: 50, Down: 60, Enable: true}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/only-hidden", nil)
	req.Header.Set("User-Agent", "Shadowrocket/2.2.58")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("all-hidden /sub status = %d, want 200", resp.Code)
	}
	if !strings.HasPrefix(resp.Header().Get("Subscription-Userinfo"), "upload=50; download=60; ") {
		t.Fatalf("Subscription-Userinfo = %q", resp.Header().Get("Subscription-Userinfo"))
	}
	if decoded := decodeSubBody(t, resp); decoded != "" {
		t.Fatalf("all-hidden decoded body = %q, want empty", decoded)
	}
}
