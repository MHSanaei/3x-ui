package service

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// WithClientDepletion derives read-only state before display-only traffic overlays.
// Legacy persisted reasons are overridden by the same predicates as enforcement.
func WithClientDepletion(tx *gorm.DB) *gorm.DB {
	now := tx.NowFunc()
	reason := `CASE WHEN ` + clientExpiryDepletedCond + ` THEN CASE WHEN ` + clientQuotaDepletedCond + ` THEN 'both' ELSE 'expiry' END
		ELSE CASE WHEN ` + clientQuotaDepletedCond + ` THEN 'traffic' ELSE '' END END`
	at := currentDepletionTimestamp(now)
	args := append([]any{now.UnixMilli(), globalTrafficFreshSince(now), globalTrafficFreshSince(now)}, at.Vars...)
	return tx.Select(`client_traffics.*, `+reason+` AS depleted_reason, `+at.SQL+` AS depleted_at`, args...)
}

// A stale global quota cannot bridge a healthy interval before a later expiry.
// Local mutations discard this evidence before reducing an exhausted local quota.
func currentDepletionTimestamp(now time.Time) clause.Expr {
	return gorm.Expr(`CASE WHEN depleted_at <= 0 THEN 0 WHEN NOT COALESCE(`+depletedClientsCond+`, FALSE) OR (
		total > 0 AND NOT `+clientLocalQuotaDepletedCond+` AND expiry_time > depleted_at AND expiry_time <= ?
		AND EXISTS (SELECT 1 FROM client_global_traffics g WHERE g.email = client_traffics.email
			AND CAST(g.up AS NUMERIC) + g.down >= client_traffics.total)
		AND NOT EXISTS (SELECT 1 FROM client_global_traffics g WHERE g.email = client_traffics.email
			AND CAST(g.up AS NUMERIC) + g.down >= client_traffics.total AND g.updated_at >= client_traffics.expiry_time - ?)
	) THEN 0 ELSE depleted_at END`, now.UnixMilli(), globalTrafficFreshSince(now), now.UnixMilli(), globalTrafficFreshWindow.Milliseconds())
}

func acceptNodeDepletion(row *xray.ClientTraffic, node xray.ClientTraffic, nodeUsed, now int64) {
	if row.DepletedAt > 0 || node.Enable || node.DepletedAt <= 0 || node.DepletedAt > now || nodeDisableIsStale(row, nodeUsed, now, 0, 0) {
		return
	}
	row.DepletedAt = node.DepletedAt
}

// clearRecoveredDepletion only touches a mutation's explicit email scope.
func clearRecoveredDepletion(tx *gorm.DB, emails []string) error {
	for _, batch := range chunkStrings(uniqueNonEmptyStrings(emails), sqlInChunk) {
		if err := recoveredDepletionRows(tx).Where("email IN ?", batch).Update("depleted_at", 0).Error; err != nil {
			return err
		}
	}
	return nil
}

func recoveredDepletionRows(tx *gorm.DB) *gorm.DB {
	at := currentDepletionTimestamp(tx.NowFunc())
	return tx.Model(&xray.ClientTraffic{}).Where("depleted_at > 0 AND depleted_at <> "+at.SQL, at.Vars...)
}

// prepareClientDepletionMutation checks continuity before limits or usage change.
// Expired global evidence is irrelevant while a local limit independently binds.
func prepareClientDepletionMutation(tx *gorm.DB, emails []string) error {
	if err := clearRecoveredDepletion(tx, emails); err != nil {
		return err
	}
	now := tx.NowFunc()
	for _, batch := range chunkStrings(uniqueNonEmptyStrings(emails), sqlInChunk) {
		if err := tx.Where(`email IN ? AND updated_at < ? AND EXISTS (
			SELECT 1 FROM client_traffics WHERE client_traffics.email = client_global_traffics.email
			AND depleted_at > 0 AND (`+clientLocalQuotaDepletedCond+` OR `+clientExpiryDepletedCond+`)
		)`, batch, globalTrafficFreshSince(now), now.UnixMilli()).Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ClientDepletionsByEmails loads current state in bounded batches for subscriptions.
func ClientDepletionsByEmails(tx *gorm.DB, emails []string) ([]xray.ClientTraffic, error) {
	var rows []xray.ClientTraffic
	for _, batch := range chunkStrings(uniqueNonEmptyStrings(emails), sqlInChunk) {
		var page []xray.ClientTraffic
		if err := tx.Scopes(WithClientDepletion).Where("email IN ?", batch).Find(&page).Error; err != nil {
			return nil, err
		}
		rows = append(rows, page...)
	}
	return rows, nil
}

// populateClientDepletion replaces mutation-response metadata without changing echoed counters.
// Membership is checked before lookup so supplied foreign emails cannot disclose other events.
func populateClientDepletion(tx *gorm.DB, stats []xray.ClientTraffic, clients []model.Client) error {
	if len(stats) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		allowed[client.Email] = struct{}{}
	}
	indexes := make(map[string][]int, len(stats))
	emails := make([]string, 0, len(stats))
	for i := range stats {
		stats[i].DepletedAt, stats[i].DepletedReason = 0, ""
		if _, ok := allowed[stats[i].Email]; !ok {
			continue
		}
		emails = append(emails, stats[i].Email)
		indexes[stats[i].Email] = append(indexes[stats[i].Email], i)
	}
	rows, err := ClientDepletionsByEmails(tx, emails)
	if err != nil {
		return err
	}
	for _, row := range rows {
		for _, i := range indexes[row.Email] {
			stats[i].DepletedAt, stats[i].DepletedReason = row.DepletedAt, row.DepletedReason
		}
	}
	return nil
}
