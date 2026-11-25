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

// Why a closed window exists. "earlier" is the fold of windows past the cap.
const (
	trafficWindowReset   = "reset"
	trafficWindowRenew   = "renew"
	trafficWindowEarlier = "earlier"
)

const (
	clientChangeSourcePanel = "panel"
	clientChangeSourceTgbot = "tgbot"
	clientChangeSourceRenew = "auto-renew"
)

// Per-client row caps: both tables stay bounded by the number of clients.
const (
	clientTrafficWindowKeep = 100
	clientChangeLogKeep     = 200
)

// ClientNodeUsage is one node's counters for a client since its last reset.
type ClientNodeUsage struct {
	NodeID   int    `json:"nodeId" example:"7"`
	NodeName string `json:"nodeName" example:"de-fra-1"`
	Up       int64  `json:"up" example:"1048576"`
	Down     int64  `json:"down" example:"2097152"`
}

// ClientUsageHistory is what the client editor's history button shows.
type ClientUsageHistory struct {
	Email        string                      `json:"email" example:"user1@example.com"`
	LifetimeUp   int64                       `json:"lifetimeUp" example:"10737418240"`
	LifetimeDown int64                       `json:"lifetimeDown" example:"21474836480"`
	CurrentUp    int64                       `json:"currentUp" example:"1048576"`
	CurrentDown  int64                       `json:"currentDown" example:"2097152"`
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

	if err := db.Where("email = ?", email).Order("at DESC, id DESC").Find(&out.Windows).Error; err != nil {
		return nil, err
	}
	out.LifetimeUp, out.LifetimeDown = out.CurrentUp, out.CurrentDown
	for _, window := range out.Windows {
		out.LifetimeUp = clampedTrafficAdd(out.LifetimeUp, window.Up)
		out.LifetimeDown = clampedTrafficAdd(out.LifetimeDown, window.Down)
	}
	if err := db.Model(&model.NodeClientTraffic{}).
		Joins("JOIN nodes n ON n.id = node_client_traffics.node_id").
		Where("node_client_traffics.email = ?", email).
		Select("node_client_traffics.node_id, n.name AS node_name, node_client_traffics.up, node_client_traffics.down").
		Order("node_client_traffics.up + node_client_traffics.down DESC").
		Scan(&out.Nodes).Error; err != nil {
		return nil, err
	}
	if err := db.Where("email = ?", email).Order("at DESC, id DESC").Find(&out.Changes).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// archiveClosedWindowsTx is the one place a client's used counters become history. Call it
// before zeroing them, in the same transaction; nil emails means every client.
func archiveClosedWindowsTx(tx *gorm.DB, emails []string, reason string) error {
	if emails != nil && len(emails) == 0 {
		return nil
	}
	used := func() *gorm.DB {
		return tx.Model(&xray.ClientTraffic{}).Select("email, up, down").Where("up > 0 OR down > 0")
	}
	var windows []model.ClientTrafficWindow
	if emails == nil {
		if err := used().Scan(&windows).Error; err != nil {
			return err
		}
	} else {
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var page []model.ClientTrafficWindow
			if err := used().Where("email IN ?", batch).Scan(&page).Error; err != nil {
				return err
			}
			windows = append(windows, page...)
		}
	}
	if len(windows) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	closed := make([]string, 0, len(windows))
	for i := range windows {
		windows[i].At, windows[i].Reason = now, reason
		closed = append(closed, windows[i].Email)
	}
	if err := tx.CreateInBatches(&windows, sqlInChunk).Error; err != nil {
		return err
	}
	return foldOldWindowsTx(tx, closed)
}

// foldOldWindowsTx merges a client's windows past the cap into one "earlier" row, so the
// table stops growing while the lifetime total it sums to stays the same.
func foldOldWindowsTx(tx *gorm.DB, emails []string) error {
	var over []string
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var page []string
		if err := tx.Model(&model.ClientTrafficWindow{}).Where("email IN ?", batch).
			Group("email").Having("COUNT(*) > ?", clientTrafficWindowKeep+1).
			Pluck("email", &page).Error; err != nil {
			return err
		}
		over = append(over, page...)
	}
	for _, email := range over {
		var old []model.ClientTrafficWindow
		if err := tx.Where("email = ?", email).Order("at DESC, id DESC").
			Offset(clientTrafficWindowKeep).Find(&old).Error; err != nil {
			return err
		}
		folded := model.ClientTrafficWindow{Email: email, At: old[0].At, Reason: trafficWindowEarlier}
		ids := make([]int64, 0, len(old))
		for _, window := range old {
			folded.Up = clampedTrafficAdd(folded.Up, window.Up)
			folded.Down = clampedTrafficAdd(folded.Down, window.Down)
			ids = append(ids, window.Id)
		}
		if err := tx.Where("id IN ?", ids).Delete(&model.ClientTrafficWindow{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&folded).Error; err != nil {
			return err
		}
	}
	return nil
}

// logClientChangesTx appends change rows stamped with the commit time, then drops each
// client's rows past the cap.
func logClientChangesTx(tx *gorm.DB, entries []model.ClientChangeLog) error {
	if len(entries) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	seen := make(map[string]struct{}, len(entries))
	emails := make([]string, 0, len(entries))
	for i := range entries {
		entries[i].At = now
		if _, dup := seen[entries[i].Email]; !dup {
			seen[entries[i].Email] = struct{}{}
			emails = append(emails, entries[i].Email)
		}
	}
	if err := tx.CreateInBatches(&entries, sqlInChunk).Error; err != nil {
		return err
	}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var over []string
		if err := tx.Model(&model.ClientChangeLog{}).Where("email IN ?", batch).
			Group("email").Having("COUNT(*) > ?", clientChangeLogKeep).
			Pluck("email", &over).Error; err != nil {
			return err
		}
		for _, email := range over {
			var oldest model.ClientChangeLog
			if err := tx.Where("email = ?", email).Order("at DESC, id DESC").
				Offset(clientChangeLogKeep).Take(&oldest).Error; err != nil {
				return err
			}
			if err := tx.Where("email = ? AND (at < ? OR (at = ? AND id <= ?))", email, oldest.At, oldest.At, oldest.Id).
				Delete(&model.ClientChangeLog{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// clientChangeWatch holds client records as they were before an operation. Diffing them
// against the stored rows afterwards logs what changed, not what a caller meant to write.
type clientChangeWatch struct {
	source string
	before []model.ClientRecord
}

func watchClientChanges(source string, emails ...string) *clientChangeWatch {
	watch := &clientChangeWatch{source: source}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var page []model.ClientRecord
		if err := database.GetDB().Where("email IN ?", batch).Find(&page).Error; err != nil {
			logger.Warning("client change log: read records failed:", err)
			return &clientChangeWatch{source: source}
		}
		watch.before = append(watch.before, page...)
	}
	return watch
}

// record pairs rows by id, so a client renamed meanwhile is logged under its new email.
func (w *clientChangeWatch) record() {
	if len(w.before) == 0 {
		return
	}
	db := database.GetDB()
	ids := make([]int, 0, len(w.before))
	for i := range w.before {
		ids = append(ids, w.before[i].Id)
	}
	after := make(map[int]*model.ClientRecord, len(ids))
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var page []model.ClientRecord
		if err := db.Where("id IN ?", batch).Find(&page).Error; err != nil {
			logger.Warning("client change log: read records failed:", err)
			return
		}
		for i := range page {
			after[page[i].Id] = &page[i]
		}
	}
	var entries []model.ClientChangeLog
	for i := range w.before {
		if now, kept := after[w.before[i].Id]; kept {
			entries = append(entries, diffClientChanges(&w.before[i], now, w.source)...)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return logClientChangesTx(tx, entries) }); err != nil {
		logger.Warning("client change log: write failed:", err)
	}
}

// diffClientChanges returns one row per client field that differs. The email is left
// out: a rename is logged where the record is renamed.
func diffClientChanges(oldRec, newRec *model.ClientRecord, source string) []model.ClientChangeLog {
	var entries []model.ClientChangeLog
	add := func(field, oldVal, newVal string) {
		entries = append(entries, model.ClientChangeLog{
			Email: newRec.Email, Field: field, OldValue: oldVal, NewValue: newVal, Source: source,
		})
	}
	if oldRec.SubID != newRec.SubID {
		add("subId", oldRec.SubID, newRec.SubID)
	}
	if oldRec.ExpiryTime != newRec.ExpiryTime {
		add("expiry", fmt.Sprint(oldRec.ExpiryTime), fmt.Sprint(newRec.ExpiryTime))
	}
	if oldRec.TotalGB != newRec.TotalGB {
		add("dataLimit", fmt.Sprint(oldRec.TotalGB), fmt.Sprint(newRec.TotalGB))
	}
	if oldRec.Enable != newRec.Enable {
		add("enable", fmt.Sprint(oldRec.Enable), fmt.Sprint(newRec.Enable))
	}
	if oldRec.LimitIP != newRec.LimitIP {
		add("limitIp", fmt.Sprint(oldRec.LimitIP), fmt.Sprint(newRec.LimitIP))
	}
	if oldRec.LimitHwid != newRec.LimitHwid {
		add("limitHwid", fmt.Sprint(oldRec.LimitHwid), fmt.Sprint(newRec.LimitHwid))
	}
	if oldRec.AdTag != newRec.AdTag {
		add("adTag", oldRec.AdTag, newRec.AdTag)
	}
	if oldRec.Group != newRec.Group {
		add("group", oldRec.Group, newRec.Group)
	}
	plan := func(r *model.ClientRecord) string {
		return fmt.Sprintf("%d/%d/%d/%d/%s:%d", r.Reset, r.ResetDay, r.ResetWeekday, r.ResetMax, r.TrafficReset, r.TrafficResetDay)
	}
	if plan(oldRec) != plan(newRec) {
		add("renewalPlan", plan(oldRec), plan(newRec))
	}
	return entries
}

// renameClientHistoryTx moves a renamed client's history to its new email and logs the
// rename, so the history follows the client instead of being reaped with the old name.
func renameClientHistoryTx(tx *gorm.DB, oldEmail, newEmail, source string) error {
	if oldEmail == "" || oldEmail == newEmail {
		return nil
	}
	if err := tx.Model(&model.ClientChangeLog{}).Where("email = ?", oldEmail).Update("email", newEmail).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.ClientTrafficWindow{}).Where("email = ?", oldEmail).Update("email", newEmail).Error; err != nil {
		return err
	}
	return logClientChangesTx(tx, []model.ClientChangeLog{{
		Email: newEmail, Field: "name", OldValue: oldEmail, NewValue: newEmail, Source: source,
	}})
}

var clientHistoryTables = []struct {
	model any
	name  string
}{
	{&model.ClientTrafficWindow{}, "client_traffic_windows"},
	{&model.ClientChangeLog{}, "client_change_logs"},
}

// ReapOrphanClientHistory drops the history of emails no client holds any more: every
// path that deletes a client is covered here instead of in each of them.
func (s *ClientService) ReapOrphanClientHistory() (int64, error) {
	var reaped int64
	for _, history := range clientHistoryTables {
		emails, err := orphanHistoryEmails(database.GetDB(), history.name)
		if err != nil {
			return reaped, err
		}
		dropped, err := deleteOrphanHistory(history.model, emails)
		reaped += dropped
		if err != nil {
			return reaped, err
		}
	}
	return reaped, nil
}

// orphanHistoryEmails walks the email index one distinct value at a time, so a run that
// finds nothing costs one seek per client instead of a scan of every history row.
func orphanHistoryEmails(db *gorm.DB, table string) ([]string, error) {
	var emails []string
	err := db.Raw(`WITH RECURSIVE held(email) AS (
	SELECT MIN(email) FROM ` + table + `
	UNION ALL
	SELECT (SELECT MIN(email) FROM ` + table + ` WHERE email > held.email) FROM held WHERE held.email IS NOT NULL
) SELECT email FROM held WHERE email IS NOT NULL AND email NOT IN (SELECT email FROM clients)`).Scan(&emails).Error
	return emails, err
}

func deleteOrphanHistory(table any, emails []string) (int64, error) {
	var dropped int64
	for _, listed := range chunkStrings(emails, sqlInChunk) {
		var res *gorm.DB
		if err := runSerializedTx(func(tx *gorm.DB) error {
			// The list was read outside the writer; a client may hold one of the emails by now.
			res = tx.Where("email IN ?", listed).
				Where("email NOT IN (?)", tx.Model(&model.ClientRecord{}).Select("email")).Delete(table)
			return res.Error
		}); err != nil {
			return dropped, err
		}
		dropped += res.RowsAffected
	}
	return dropped, nil
}
