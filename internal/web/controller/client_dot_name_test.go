package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedLegacyDotClient(t *testing.T, email string) (*gin.Engine, model.ClientRecord, model.Inbound) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	db := database.GetDB()
	rec := model.ClientRecord{Email: email, UUID: "11111111-2222-3333-4444-555555555555", SubID: "legacy-dot-sub", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	settings, err := json.Marshal(map[string]any{
		"clients":    []model.Client{*rec.ToClient()},
		"decryption": "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	ib := model.Inbound{Tag: "legacy-dot", Port: 443, Protocol: model.VLESS, Settings: string(settings), StreamSettings: `{"network":"tcp","security":"none"}`}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, InboundId: ib.Id, Up: 111, Down: 222, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewClientController(router.Group("/base/panel/api/clients"))
	return router, rec, ib
}

func dotClientRequest(t *testing.T, router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, "/base/panel/api/clients/"+path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func assertDotClientSuccess(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var msg entity.Msg
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || !msg.Success {
		t.Fatalf("status/body = %d/%s", w.Code, w.Body.String())
	}
}

func TestClientQueryAliasRecoversLegacyDotName(t *testing.T) {
	for _, email := range []string{".", ".."} {
		t.Run(email, func(t *testing.T) {
			router, rec, ib := seedLegacyDotClient(t, email)
			query := "?email=" + url.QueryEscape(email)
			w := dotClientRequest(t, router, http.MethodGet, "get"+query, nil)
			assertDotClientSuccess(t, w)
			var full struct {
				Obj struct {
					Client      model.ClientRecord `json:"client"`
					InboundIDs  []int              `json:"inboundIds"`
					UsedTraffic int64              `json:"usedTraffic"`
				} `json:"obj"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &full); err != nil {
				t.Fatal(err)
			}
			if full.Obj.Client.Email != email || full.Obj.Client.UUID != rec.UUID || len(full.Obj.InboundIDs) != 1 || full.Obj.InboundIDs[0] != ib.Id || full.Obj.UsedTraffic != 333 {
				t.Fatalf("unexpected legacy payload: %s", w.Body.String())
			}
			updated := rec.ToClient()
			updated.Email = "restored-client"
			w = dotClientRequest(t, router, http.MethodPost, "update"+query, updated)
			assertDotClientSuccess(t, w)
			db := database.GetDB()
			var after model.ClientRecord
			if err := db.First(&after, rec.Id).Error; err != nil {
				t.Fatal(err)
			}
			if after.Email != updated.Email || after.UUID != rec.UUID || after.SubID != rec.SubID {
				t.Fatalf("rename changed identity: %+v", after)
			}
			if err := db.First(&ib, ib.Id).Error; err != nil {
				t.Fatal(err)
			}
			var settings struct {
				Clients []model.Client `json:"clients"`
			}
			if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
				t.Fatal(err)
			}
			if len(settings.Clients) != 1 || settings.Clients[0].Email != updated.Email || settings.Clients[0].ID != rec.UUID {
				t.Fatalf("rename lost inbound credentials: %s", ib.Settings)
			}
			var traffic xray.ClientTraffic
			if err := db.Where("email = ?", updated.Email).First(&traffic).Error; err != nil {
				t.Fatal(err)
			}
			if traffic.Up != 111 || traffic.Down != 222 {
				t.Fatalf("rename lost usage: %+v", traffic)
			}
		})
	}
}

func TestClientQueryAliasDeletesLegacyDotName(t *testing.T) {
	for _, email := range []string{".", ".."} {
		for _, keep := range []bool{false, true} {
			t.Run(email+"/keep="+strconv.FormatBool(keep), func(t *testing.T) {
				router, rec, ib := seedLegacyDotClient(t, email)
				path := "del?email=" + url.QueryEscape(email)
				if keep {
					path += "&keepTraffic=1"
				}
				w := dotClientRequest(t, router, http.MethodPost, path, nil)
				assertDotClientSuccess(t, w)
				db := database.GetDB()
				var count int64
				if err := db.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("client remains: count=%d err=%v", count, err)
				}
				if err := db.Model(&model.ClientInbound{}).Where("client_id = ?", rec.Id).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("attachment remains: count=%d err=%v", count, err)
				}
				if err := db.First(&ib, ib.Id).Error; err != nil {
					t.Fatal(err)
				}
				var settings struct {
					Clients []model.Client `json:"clients"`
				}
				if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil || len(settings.Clients) != 0 {
					t.Fatalf("inbound client remains: %s err=%v", ib.Settings, err)
				}
				if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				var want int64
				if keep {
					want = 1
				}
				if count != want {
					t.Fatalf("retained traffic count=%d keep=%v", count, keep)
				}
			})
		}
	}
}

func TestClientQueryAliasRequiresEmail(t *testing.T) {
	router, _, _ := seedLegacyDotClient(t, ".")
	for _, route := range []struct {
		method, path string
	}{
		{http.MethodGet, "get"}, {http.MethodPost, "update"}, {http.MethodPost, "del"},
	} {
		w := dotClientRequest(t, router, route.method, route.path, map[string]any{"email": "must-not-be-used-as-target"})
		var msg entity.Msg
		if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Success || !strings.Contains(msg.Msg, "client email is required") {
			t.Fatalf("missing query accepted: %s", w.Body.String())
		}
	}
}

func TestClientPathEmailTakesPrecedenceOverQuery(t *testing.T) {
	router, legacy, _ := seedLegacyDotClient(t, ".")
	normal := model.ClientRecord{Email: "audit-client", UUID: "normal-uuid", SubID: "normal-sub", Enable: true}
	db := database.GetDB()
	if err := db.Create(&normal).Error; err != nil {
		t.Fatal(err)
	}
	w := dotClientRequest(t, router, http.MethodPost, "del/audit-client?email=.", nil)
	assertDotClientSuccess(t, w)
	var count int64
	if err := db.Model(&model.ClientRecord{}).Where("id = ?", normal.Id).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("path client remains: count=%d err=%v", count, err)
	}
	if err := db.Model(&model.ClientRecord{}).Where("id = ?", legacy.Id).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("query overrode the path identity: count=%d err=%v", count, err)
	}
}
