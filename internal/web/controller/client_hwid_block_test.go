package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
)

func TestClientHwidBlockAPI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	rec := model.ClientRecord{Email: "ban-device", SubID: "ban-sub", Enable: true}
	if err := database.GetDB().Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewClientController(router.Group("/panel/api/clients"))
	device := model.ClientHwid{SubID: rec.SubID, HwidHash: "hashed-device"}
	if err := database.GetDB().Create(&device).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("hwids/%s/%d/block", rec.Email, device.Id)
	request := func(body any) entity.Msg {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/panel/api/clients/"+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var msg entity.Msg
		if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil || w.Code != http.StatusOK {
			t.Fatalf("response: %d %s, %v", w.Code, w.Body, err)
		}
		return msg
	}
	for _, body := range []any{map[string]any{}, map[string]any{"blocked": nil}, map[string]any{"blocked": "true"}} {
		if msg := request(body); msg.Success {
			t.Fatalf("invalid payload accepted: %+v", msg)
		}
	}
	for _, blocked := range []bool{true, false} {
		if msg := request(map[string]bool{"blocked": blocked}); !msg.Success {
			t.Fatalf("valid payload rejected: %+v", msg)
		}
		var stored model.ClientHwid
		if err := database.GetDB().First(&stored, device.Id).Error; err != nil || stored.Blocked != blocked {
			t.Fatalf("blocked=%v: %+v, %v", blocked, stored, err)
		}
	}
}
