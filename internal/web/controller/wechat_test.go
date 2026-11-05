package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/wechat"
)

func TestWechatTestEndpointRefusesDisabledSavedConfiguration(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	previous := wechatService
	t.Cleanup(func() { wechatService = previous })
	SetWechatService(wechat.NewService(service.SettingService{}))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSettingController(router.Group("/panel/api"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/panel/api/setting/testWechat", nil))
	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || result.Success || result.Msg == "" {
		t.Fatalf("disabled test endpoint returned %d %s", response.Code, response.Body.String())
	}
}
