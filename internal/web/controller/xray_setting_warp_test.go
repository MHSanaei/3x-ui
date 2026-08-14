package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
)

func TestWarpIntervalReportsClockPersistenceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	for _, setting := range []*model.Setting{
		{Key: "warpUpdateInterval", Value: "0"},
		{Key: "warpLastUpdate", Value: "0"},
	} {
		if err := db.Create(setting).Error; err != nil {
			t.Fatalf("seed %s: %v", setting.Key, err)
		}
	}

	const callback = "test:fail_warp_clock_update"
	errInjected := errors.New("injected WARP clock persistence failure")
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		setting, ok := tx.Statement.Model.(*model.Setting)
		if ok && setting.Key == "warpLastUpdate" {
			tx.AddError(errInjected)
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Update().Remove(callback); err != nil {
			t.Errorf("remove update callback: %v", err)
		}
	})

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	NewXraySettingController(engine.Group("/panel/api"))
	form := url.Values{"interval": {"7"}}
	req := httptest.NewRequest(http.MethodPost, "/panel/api/xray/warp/interval", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Fatalf("interval update reported success after clock persistence failure: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), errInjected.Error()) {
		t.Fatalf("response omitted clock persistence error: %s", w.Body.String())
	}
}
