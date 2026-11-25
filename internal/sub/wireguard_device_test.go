package sub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSelectWireguardDeviceKeepsParentLifecycle(t *testing.T) {
	client := model.Client{
		Email: "parent", Enable: false, TotalGB: 100, ExpiryTime: 1234,
		PrivateKey: "primary", AllowedIPs: []string{"10.0.0.2/32"},
		WireguardDevices: []model.WireguardDevice{{
			Id: "phone", Enable: true, PrivateKey: "phone-key", PublicKey: "phone-public",
			PreSharedKey: "phone-psk", AllowedIPs: []string{"10.0.0.3/32"},
		}},
	}
	selected, err := selectWireguardDevice(client, "phone")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Enable || selected.Email != client.Email || selected.TotalGB != client.TotalGB || selected.ExpiryTime != client.ExpiryTime {
		t.Fatalf("parent lifecycle changed: %+v", selected)
	}
	if selected.PrivateKey != "phone-key" || selected.PublicKey != "phone-public" || selected.PreSharedKey != "phone-psk" || selected.AllowedIPs[0] != "10.0.0.3/32" {
		t.Fatalf("device credentials were not selected: %+v", selected)
	}
	selected.AllowedIPs[0] = "changed"
	if client.WireguardDevices[0].AllowedIPs[0] != "10.0.0.3/32" {
		t.Fatal("selection mutated stored addresses")
	}
	for _, id := range []string{"", "primary"} {
		primary, err := selectWireguardDevice(client, id)
		if err != nil || primary.PrivateKey != "primary" {
			t.Fatalf("primary selection = %+v, %v", primary, err)
		}
	}
	client.WireguardDevices[0].Enable = false
	for _, id := range []string{"phone", "missing"} {
		if _, err := selectWireguardDevice(client, id); !errors.Is(err, errWireguardDeviceUnavailable) {
			t.Fatalf("selection %q error = %v", id, err)
		}
	}
}

func TestWireguardDeviceURLsRetainSelection(t *testing.T) {
	service := NewSubService("").ForWireguardDevice("phone id")
	link, err := url.Parse(service.buildSingleURL("https://example.com/sub/", "", "", "subscriber"))
	if err != nil {
		t.Fatal(err)
	}
	if link.Path != "/sub/subscriber" || link.Query().Get("wgDevice") != "phone id" {
		t.Fatalf("subscription URL = %s", link)
	}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "https://example.com/sub/subscriber?wgDevice=phone+id&view=raw", nil)
	profile, err := url.Parse(builtinProfileURL(context, "https", "example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if profile.Query().Get("wgDevice") != "phone id" || profile.Query().Get("html") != "1" || profile.Query().Has("view") {
		t.Fatalf("profile URL = %s", profile)
	}
}

func TestWireguardDeviceSubscriptionFormats(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	serverPrivate, _ := mustWireguardKeypair(t)
	primaryPrivate, primaryPublic := mustWireguardKeypair(t)
	phonePrivate, phonePublic := mustWireguardKeypair(t)
	const subID = "device-sub"
	client := model.Client{
		Email: "devices@example.com", SubID: subID, Enable: true,
		PrivateKey: primaryPrivate, PublicKey: primaryPublic, AllowedIPs: []string{"10.0.0.2/32"},
		WireguardDevices: []model.WireguardDevice{{
			Id: "phone", Name: "Phone", Enable: true, PrivateKey: phonePrivate, PublicKey: phonePublic,
			AllowedIPs: []string{"10.0.0.3/32"},
		}},
	}
	otherPrivate, otherPublic := mustWireguardKeypair(t)
	other := model.Client{
		Email: "other@example.com", SubID: subID, Enable: true,
		PrivateKey: otherPrivate, PublicKey: otherPublic, AllowedIPs: []string{"10.0.0.4/32"},
	}
	settings, err := json.Marshal(map[string]any{"secretKey": serverPrivate, "clients": []model.Client{client, other}})
	if err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{
		UserId: 1, Tag: "wg-devices", Protocol: model.WireGuard, Enable: true,
		Listen: "203.0.113.7", Port: 51820, Remark: "WG", Settings: string(settings),
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	record := model.ClientRecord{Email: client.Email, SubID: subID, Enable: true, PrivateKey: primaryPrivate, AllowedIPs: "10.0.0.2/32"}
	if err := db.Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatal(err)
	}
	otherRecord := model.ClientRecord{Email: other.Email, SubID: subID, Enable: true}
	if err := db.Create(&otherRecord).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: otherRecord.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSUBController(router.Group("/"), WithSUBEncryption(false), WithSUBJsonEnabled(true), WithSUBClashEnabled(true))
	for _, path := range []string{"/sub/", "/json/", "/clash/"} {
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				query string
				key   string
				addr  string
			}{
				{"?wgDevice=phone", phonePrivate, "10.0.0.3"},
				{"", primaryPrivate, "10.0.0.2"},
				{"?wgDevice=primary", primaryPrivate, "10.0.0.2"},
			} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+subID+tc.query, nil))
				if response.Code != http.StatusOK {
					t.Fatalf("%s%s: status %d, body %s", path, tc.query, response.Code, response.Body.String())
				}
				body := response.Body.String()
				if path == "/sub/" {
					found := false
					for line := range strings.SplitSeq(strings.TrimSpace(body), "\n") {
						link := parseWireguardSubLink(t, line)
						if tc.query == "?wgDevice=phone" && link.User.Username() == otherPrivate {
							t.Fatal("device subscription included another client's WireGuard identity")
						}
						found = found || (link.User.Username() == tc.key && strings.Contains(link.Query().Get("address"), tc.addr))
					}
					if !found {
						t.Fatalf("wrong device in link: %s", body)
					}
				} else if !strings.Contains(body, tc.key) || !strings.Contains(body, tc.addr) {
					t.Fatalf("wrong device in %s: %s", path, body)
				}
				if tc.query == "?wgDevice=phone" && strings.Contains(body, otherPrivate) {
					t.Fatal("device subscription included another client's WireGuard identity")
				}
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+subID+"?wgDevice=missing", nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("unknown device status = %d", response.Code)
			}
		})
	}
	client.WireguardDevices[0].Enable = false
	settings, err = json.Marshal(map[string]any{"secretKey": serverPrivate, "clients": []model.Client{client, other}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&inbound).Update("settings", string(settings)).Error; err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/sub/", "/json/", "/clash/"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+subID+"?wgDevice=phone", nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("disabled device %s status = %d", path, response.Code)
		}
	}
}
