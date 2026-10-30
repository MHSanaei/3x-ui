package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// Closed-window reasons, one per way a client's counters leave the live row.
const (
	trafficWindowReset  = "reset"
	trafficWindowRenew  = "renew"
	trafficWindowDelete = "deleted"
	trafficWindowRename = "renamed"
)

// Change-log sources.
const (
	clientChangeSourcePanel = "panel"
	clientChangeSourceTgbot = "tgbot"
	clientChangeSourceRenew = "auto-renew"
)

const (
	// usageHistoryWindowLimit caps the per-client window list shown in the UI;
	// the lifetime totals still account for every archived row.
	usageHistoryWindowLimit = 2000
	usageHistoryChangeLimit = 500
)

// ClientNodeUsage is one node's accumulated counters for a client since its
// last reset, with the node's display name resolved for the panel.
type ClientNodeUsage struct {
	NodeID   int    `json:"nodeId"`
	NodeName string `json:"nodeName"`
	Up       int64  `json:"up"`
	Down     int64  `json:"down"`
}

// ClientUsageHistory is the client's whole usage + change story behind the
// edit modal's history button: lifetime totals, closed windows, per-node
// counters, and the metadata change log.
type ClientUsageHistory struct {
	Email        string                      `json:"email"`
	LifetimeUp   int64                       `json:"lifetimeUp"`
	LifetimeDown int64                       `json:"lifetimeDown"`
	CurrentUp    int64                       `json:"currentUp"`
	CurrentDown  int64                       `json:"currentDown"`
	Windows      []model.ClientTrafficWindow `json:"windows"`
	Nodes        []ClientNodeUsage           `json:"nodes"`
	Changes      []model.ClientChangeLog     `json:"changes"`
}

func (s *ClientService) GetUsageHistory(email string) (*ClientUsageHistory, error) {
	db := database.GetDB()
	out := &ClientUsageHistory{
		Email:   email,
		Windows: []model.ClientTrafficWindow{},
		Nodes:   []ClientNodeUsage{},
		Changes: []model.ClientChangeLog{},
	}

	var live xray.ClientTraffic
	err := db.Where("email = ?", email).First(&live).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		out.CurrentUp, out.CurrentDown = live.Up, live.Down
	}

	var sums struct {
		Up   int64
		Down int64
	}
	if err := db.Model(&model.ClientTrafficWindow{}).
		Where("email = ?", email).
		Select("COALESCE(SUM(up), 0) AS up, COALESCE(SUM(down), 0) AS down").
		Scan(&sums).Error; err != nil {
		return nil, err
	}
	out.LifetimeUp = sums.Up + out.CurrentUp
	out.LifetimeDown = sums.Down + out.CurrentDown

	if err := db.Model(&model.ClientTrafficWindow{}).
		Where("email = ?", email).
		Order("at DESC, id DESC").
		Limit(usageHistoryWindowLimit).
		Find(&out.Windows).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.NodeClientTraffic{}).
		Joins("JOIN nodes n ON n.id = node_client_traffics.node_id").
		Where("node_client_traffics.email = ?", email).
		Select("node_client_traffics.node_id, n.name AS node_name, node_client_traffics.up, node_client_traffics.down").
		Order("node_client_traffics.up + node_client_traffics.down DESC").
		Scan(&out.Nodes).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.ClientChangeLog{}).
		Where("email = ?", email).
		Order("at DESC, id DESC").
		Limit(usageHistoryChangeLimit).
		Find(&out.Changes).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// recordClosedWindowsTx archives the clients' about-to-be-zeroed counters as
// closed windows, in the caller's transaction; only rows with real usage
// are archived, and it must run before the counters are zeroed.
func recordClosedWindowsTx(tx *gorm.DB, emails []string, reason string) error {
	if len(emails) == 0 {
		return nil
	}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var seeds []struct {
			Email string
			Up    int64
			Down  int64
		}
		if err := tx.Model(xray.ClientTraffic{}).
			Select("email, up, down").
			Where("email IN ? AND (up > 0 OR down > 0)", batch).
			Scan(&seeds).Error; err != nil {
			return err
		}
		if len(seeds) == 0 {
			continue
		}
		windows := make([]model.ClientTrafficWindow, 0, len(seeds))
		for _, seed := range seeds {
			windows = append(windows, model.ClientTrafficWindow{
				Email: seed.Email, Up: seed.Up, Down: seed.Down, Reason: reason,
			})
		}
		if err := archiveClosedWindowsTx(tx, windows); err != nil {
			return err
		}
	}
	return nil
}

// archiveClosedWindowsTx persists already-built windows in the caller's
// transaction, stamping the close time onto any that lack one.
func archiveClosedWindowsTx(tx *gorm.DB, windows []model.ClientTrafficWindow) error {
	if len(windows) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	for i := range windows {
		if windows[i].At == 0 {
			windows[i].At = now
		}
	}
	return tx.Create(&windows).Error
}

// logClientChangesTx appends the change-log rows of one commit; entries may
// leave At zero, in which case the commit time is stamped on all of them.
func logClientChangesTx(tx *gorm.DB, entries []model.ClientChangeLog) error {
	if len(entries) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	for i := range entries {
		if entries[i].At == 0 {
			entries[i].At = now
		}
	}
	return tx.Create(&entries).Error
}

// recordClosedWindowsAllTx archives every live counter with real usage; run
// it before a whole-table zeroing, inside the caller's transaction.
func recordClosedWindowsAllTx(tx *gorm.DB, reason string) error {
	var seeds []struct {
		Email string
		Up    int64
		Down  int64
	}
	if err := tx.Model(xray.ClientTraffic{}).
		Select("email, up, down").
		Where("up > 0 OR down > 0").
		Scan(&seeds).Error; err != nil {
		return err
	}
	if len(seeds) == 0 {
		return nil
	}
	windows := make([]model.ClientTrafficWindow, 0, len(seeds))
	for _, seed := range seeds {
		windows = append(windows, model.ClientTrafficWindow{
			Email: seed.Email, Up: seed.Up, Down: seed.Down, Reason: reason,
		})
	}
	return archiveClosedWindowsTx(tx, windows)
}

// logClientChange appends one change-log row for an admin action outside a
// caller transaction; a no-op when the value did not actually change.
func logClientChange(email, field, oldValue, newValue, source string) {
	if oldValue == newValue {
		return
	}
	entry := model.ClientChangeLog{
		Email: email, Field: field, OldValue: oldValue, NewValue: newValue,
		Source: source, At: time.Now().UnixMilli(),
	}
	if err := database.GetDB().Create(&entry).Error; err != nil {
		logger.Warning("log client change failed:", err)
	}
}

// diffClientChanges returns one row per client-info field that differs
// between the two states, keyed by newRec's email so history follows a
// rename; empty when nothing changed. Renames are excluded: the record
// rename sites log the "name" event exactly once per rename.
func diffClientChanges(oldRec, newRec *model.ClientRecord) []model.ClientChangeLog {
	var entries []model.ClientChangeLog
	add := func(field, oldVal, newVal string) {
		entries = append(entries, model.ClientChangeLog{
			Email: newRec.Email, Field: field, OldValue: oldVal, NewValue: newVal,
		})
	}
	if oldRec.SubID != newRec.SubID {
		add("subId", oldRec.SubID, newRec.SubID)
	}
	if oldRec.ExpiryTime != newRec.ExpiryTime {
		add("expiry", fmt.Sprintf("%d", oldRec.ExpiryTime), fmt.Sprintf("%d", newRec.ExpiryTime))
	}
	if oldRec.TotalGB != newRec.TotalGB {
		add("dataLimit", fmt.Sprintf("%d", oldRec.TotalGB), fmt.Sprintf("%d", newRec.TotalGB))
	}
	if oldRec.Enable != newRec.Enable {
		add("enable", fmt.Sprintf("%t", oldRec.Enable), fmt.Sprintf("%t", newRec.Enable))
	}
	if oldRec.LimitIP != newRec.LimitIP {
		add("limitIp", fmt.Sprintf("%d", oldRec.LimitIP), fmt.Sprintf("%d", newRec.LimitIP))
	}
	if oldRec.LimitHwid != newRec.LimitHwid {
		add("limitHwid", fmt.Sprintf("%d", oldRec.LimitHwid), fmt.Sprintf("%d", newRec.LimitHwid))
	}
	if oldRec.AdTag != newRec.AdTag {
		add("adTag", oldRec.AdTag, newRec.AdTag)
	}
	if oldRec.Group != newRec.Group {
		add("group", oldRec.Group, newRec.Group)
	}
	if oldRec.Reset != newRec.Reset || oldRec.ResetDay != newRec.ResetDay ||
		oldRec.ResetWeekday != newRec.ResetWeekday || oldRec.ResetMax != newRec.ResetMax ||
		oldRec.TrafficReset != newRec.TrafficReset || oldRec.TrafficResetDay != newRec.TrafficResetDay {
		plan := func(r *model.ClientRecord) string {
			return fmt.Sprintf("%d/%d/%d/%d/%s:%d", r.Reset, r.ResetDay, r.ResetWeekday, r.ResetMax, r.TrafficReset, r.TrafficResetDay)
		}
		add("renewalPlan", plan(oldRec), plan(newRec))
	}
	return entries
}

// migrateClientHistoryEmailTx re-keys a renamed client's closed windows and
// change log onto the new email, in the caller's transaction, so its whole
// history follows the rename instead of being stranded under the old name.
func migrateClientHistoryEmailTx(tx *gorm.DB, oldEmail, newEmail string) error {
	if oldEmail == "" || oldEmail == newEmail {
		return nil
	}
	if err := tx.Model(&model.ClientChangeLog{}).Where("email = ?", oldEmail).
		Update("email", newEmail).Error; err != nil {
		return err
	}
	return tx.Model(&model.ClientTrafficWindow{}).Where("email = ?", oldEmail).
		Update("email", newEmail).Error
}
