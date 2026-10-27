package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// renewalReenables reports whether the client identified by email was switched
// off by the quota enforcement loop and would come back within its limits once
// newTotal and newExpiry are in effect. It is the single-client twin of the
// set-difference re-enable that BulkAdjust applies on a renewal (#5619); the
// client editor's save (/clients/update) and the single-field renewal setters
// are the renewal paths that still left a renewed client disabled, forcing the
// operator to flip the enable switch by hand.
//
// The check reads as disabled-by-quota only when the enforcement row itself
// carries the off switch AND matches the production depletion predicate, so a
// client the operator switched off by hand (or that is simply under quota) is
// never resurrected. The post-update side is projected from the values this
// renewal writes — the traffic row's up/down counters are untouched by an
// update, so only total and expiry move.
func (s *ClientService) renewalReenables(email string, newTotal, newExpiry int64) bool {
	db := database.GetDB()
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&traffic).Error; err != nil {
		return false // no enforcement row: the quota loop never switched this client off
	}
	if traffic.Enable {
		return false
	}

	// Was it the quota loop that switched it off, under the same predicate
	// (including a fresh cross-panel push) the loop itself uses?
	cond, condArgs := depletedCond(db)
	var wasDepleted int64
	if err := db.Model(xray.ClientTraffic{}).
		Where(cond+" AND email = ?", append(append([]any{}, condArgs...), email)...).
		Count(&wasDepleted).Error; err != nil || wasDepleted == 0 {
		return false // disabled by hand, not by the quota loop
	}

	// Does this renewal clear the depletion? The local branches against the
	// projected row values, then the cross-panel branch the loop would apply
	// to the projected total.
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

// storedTrafficLimits reads a client's enforcement row; the second result is
// false when the client has no row, in which case no quota state can be
// projected for it.
func storedTrafficLimits(email string) (total, expiry int64, ok bool) {
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		return 0, 0, false
	}
	return row.Total, row.ExpiryTime, true
}
