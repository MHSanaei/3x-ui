package service

import (
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const externalSubHwidKey = "externalSubHwid"

// externalSubHwidMu serializes first-time creation: without it, concurrent first
// fetches each mint and persist their own id.
var externalSubHwidMu sync.Mutex

// ExternalSubscriptionHwid is the X-HWID every subscription fetch sends, so an
// HWID-limited provider counts this panel as one device. Empty: DB unreachable.
func ExternalSubscriptionHwid() string {
	externalSubHwidMu.Lock()
	defer externalSubHwidMu.Unlock()
	db := database.GetDB()
	if db == nil {
		return ""
	}
	var row model.Setting
	if err := db.Where("key = ?", externalSubHwidKey).First(&row).Error; err == nil {
		if strings.TrimSpace(row.Value) != "" {
			return strings.TrimSpace(row.Value)
		}
	}
	hwid := "3x-ui-server-" + uuid.NewString()
	row = model.Setting{Key: externalSubHwidKey, Value: hwid}
	if err := db.Where(model.Setting{Key: externalSubHwidKey}).FirstOrCreate(&row).Error; err != nil {
		logger.Warningf("persisting the external subscription hwid failed: %v", err)
		return ""
	}
	if strings.TrimSpace(row.Value) == "" {
		return hwid
	}
	return strings.TrimSpace(row.Value)
}
