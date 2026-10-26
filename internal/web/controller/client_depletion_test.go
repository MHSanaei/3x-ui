package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientDetailDepletionIsAdditiveAndReadOnly(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()
	rec := model.ClientRecord{Email: "depleted", SubID: "depleted-sub", TgID: 123, Enable: false}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: rec.Email, Up: 111, Down: 222, Total: 300, DepletedAt: 1700000000000, DepletedReason: "traffic"}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewClientController(router.Group("/panel/api/clients"))
	for _, path := range []string{"get/depleted", "get/subId/depleted-sub", "get/tgId/123"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panel/api/clients/"+path, nil))
		var envelope struct {
			Success bool            `json:"success"`
			Obj     json.RawMessage `json:"obj"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || !envelope.Success {
			t.Fatalf("%s: %s (%v)", path, w.Body.String(), err)
		}
		var payload map[string]json.RawMessage
		if path == "get/depleted" {
			if err := json.Unmarshal(envelope.Obj, &payload); err != nil {
				t.Fatal(err)
			}
		} else {
			var payloads []map[string]json.RawMessage
			if err := json.Unmarshal(envelope.Obj, &payloads); err != nil || len(payloads) != 1 {
				t.Fatalf("%s: %s (%v)", path, envelope.Obj, err)
			}
			payload = payloads[0]
		}
		if string(payload["depletedAt"]) != "1700000000000" || string(payload["depletedReason"]) != `"traffic"` || string(payload["usedTraffic"]) != "333" || len(payload) != 7 {
			t.Fatalf("%s payload = %s", path, envelope.Obj)
		}
		var client map[string]any
		if err := json.Unmarshal(payload["client"], &client); err != nil {
			t.Fatal(err)
		}
		if _, writable := client["depletedAt"]; writable {
			t.Fatal("depletion must not be duplicated on the writable client model")
		}
	}
	for _, path := range []string{"list", "list/paged", "traffic/depleted"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panel/api/clients/"+path, nil))
		var envelope struct {
			Success bool            `json:"success"`
			Obj     json.RawMessage `json:"obj"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || !envelope.Success {
			t.Fatalf("%s: %s (%v)", path, w.Body.String(), err)
		}
		var traffic xray.ClientTraffic
		if path == "traffic/depleted" {
			if err := json.Unmarshal(envelope.Obj, &traffic); err != nil {
				t.Fatal(err)
			}
		} else {
			var rows []struct {
				Traffic xray.ClientTraffic `json:"traffic"`
			}
			data := envelope.Obj
			if path == "list/paged" {
				var page struct {
					Items json.RawMessage `json:"items"`
				}
				if err := json.Unmarshal(data, &page); err != nil {
					t.Fatal(err)
				}
				data = page.Items
			}
			if err := json.Unmarshal(data, &rows); err != nil || len(rows) != 1 {
				t.Fatalf("%s rows = %s (%v)", path, data, err)
			}
			traffic = rows[0].Traffic
		}
		if traffic.DepletedAt != 1700000000000 || traffic.DepletedReason != "traffic" || traffic.Up+traffic.Down != 333 {
			t.Fatalf("%s traffic = %+v", path, traffic)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/panel/api/clients/updateTraffic/depleted",
		strings.NewReader(`{"upload":500,"download":0,"depletedAt":1,"depletedReason":"expiry"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	var row xray.ClientTraffic
	if err := db.Where("email = ?", rec.Email).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Up != 500 || row.DepletedAt != 1700000000000 || row.DepletedReason != "" {
		t.Fatalf("a traffic update accepted read-only metadata: %+v; response=%s", row, w.Body.String())
	}
	got, err := (&service.InboundService{}).GetClientTrafficByEmail(rec.Email)
	if err != nil || got.DepletedReason != "traffic" {
		t.Fatalf("current reason after read-only update = %+v, %v", got, err)
	}
}
