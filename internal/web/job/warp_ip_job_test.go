package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestWarpIpJobInitializesMissingLastUpdate(t *testing.T) {
	setupIntegrationDB(t)

	settings := service.SettingService{}
	if err := settings.SetWarpUpdateInterval(1); err != nil {
		t.Fatalf("enable scheduled WARP rotation: %v", err)
	}

	job := &WarpIpJob{settingService: settings}
	job.Run()

	var stored model.Setting
	if err := database.GetDB().Where("key = ?", "warpLastUpdate").First(&stored).Error; err != nil {
		t.Fatalf("scheduled WARP rotation did not establish its baseline: %v", err)
	}
	if stored.Value == "" || stored.Value == "0" {
		t.Fatalf("warpLastUpdate = %q, want a non-zero baseline", stored.Value)
	}
}
