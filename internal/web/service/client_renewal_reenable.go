package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// renewalReenables reports whether the quota loop switched the client off and
// this renewal (newTotal/newExpiry, usage unchanged) clears the depletion (#5619).
func (s *ClientService) renewalReenables(email string, newTotal, newExpiry int64) bool {
	db := database.GetDB()
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&traffic).Error; err != nil {
		return false // no enforcement row: the quota loop never switched this client off
	}
	if traffic.Enable {
		return false
	}

	cond, condArgs := depletedCond(db)
	var wasDepleted int64
	if err := db.Model(xray.ClientTraffic{}).
		Where(cond+" AND email = ?", append(append([]any{}, condArgs...), email)...).
		Count(&wasDepleted).Error; err != nil || wasDepleted == 0 {
		return false // off for another reason than depletion: the operator's off stands
	}

	// Project the post-renewal row: total/expiry move, up/down and a fresh
	// cross-panel push can still deplete it.
	now := time.Now().UnixMilli()
	used := traffic.Up + traffic.Down
	if newTotal > 0 && used >= newTotal {
		return false // still over quota
	}
	if newExpiry > 0 && newExpiry <= now {
		return false // still expired
	}
	if newTotal > 0 {
		var globalDepleted int64
		if err := db.Model(&model.ClientGlobalTraffic{}).
			Where("email = ? AND updated_at >= ? AND up + down >= ?", email, globalTrafficFreshSince(time.Now()), newTotal).
			Count(&globalDepleted).Error; err == nil && globalDepleted > 0 {
			return false // a master's pushed usage still depletes the fresh quota
		}
	}
	return true
}

// storedTrafficLimits reads a client's enforcement row; ok is false when the
// client has no row, in which case no quota state can be projected for it.
func storedTrafficLimits(email string) (total, expiry int64, ok bool) {
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		return 0, 0, false
	}
	return row.Total, row.ExpiryTime, true
}
