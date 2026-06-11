package service

import (
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AcceptGlobalTraffic ingests a master panel's aggregated per-client usage
// into client_global_traffics, keyed by (masterGuid, email). The numbers are
// display/enforcement inputs only — client_traffics keeps this panel's
// local-only counters, so the pushing master's (and any other master's)
// delta accounting over our snapshot stays correct.
//
// Rows are overwritten, not max-merged: a reset on the master propagates here
// on its next push. Emails this panel doesn't host are dropped.
func (s *InboundService) AcceptGlobalTraffic(masterGuid string, traffics []*xray.ClientTraffic) error {
	masterGuid = strings.TrimSpace(masterGuid)
	if masterGuid == "" {
		return nil
	}
	emails := make([]string, 0, len(traffics))
	byEmail := make(map[string]*xray.ClientTraffic, len(traffics))
	for _, t := range traffics {
		if t == nil || t.Email == "" {
			continue
		}
		if _, dup := byEmail[t.Email]; !dup {
			emails = append(emails, t.Email)
		}
		byEmail[t.Email] = t
	}
	if len(emails) == 0 {
		return nil
	}

	return submitTrafficWrite(func() error {
		db := database.GetDB()
		known := make([]string, 0, len(emails))
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var page []string
			if err := db.Model(xray.ClientTraffic{}).
				Where("email IN ?", batch).
				Pluck("email", &page).Error; err != nil {
				return err
			}
			known = append(known, page...)
		}
		if len(known) == 0 {
			return nil
		}

		now := time.Now().UnixMilli()
		rows := make([]model.ClientGlobalTraffic, 0, len(known))
		for _, email := range known {
			t := byEmail[email]
			if t == nil {
				continue
			}
			rows = append(rows, model.ClientGlobalTraffic{
				MasterGuid: masterGuid,
				Email:      email,
				Up:         t.Up,
				Down:       t.Down,
				UpdatedAt:  now,
			})
		}

		return db.Transaction(func(tx *gorm.DB) error {
			for _, batch := range chunkGlobalRows(rows, 200) {
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "master_guid"}, {Name: "email"}},
					DoUpdates: clause.AssignmentColumns([]string{"up", "down", "updated_at"}),
				}).Create(&batch).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func chunkGlobalRows(rows []model.ClientGlobalTraffic, size int) [][]model.ClientGlobalTraffic {
	if len(rows) == 0 {
		return nil
	}
	out := make([][]model.ClientGlobalTraffic, 0, (len(rows)+size-1)/size)
	for start := 0; start < len(rows); start += size {
		end := min(start+size, len(rows))
		out = append(out, rows[start:end])
	}
	return out
}

// overlayGlobalTraffic raises Up/Down on the given rows to the largest global
// value any master pushed for that email. Read-path only — callers hand it
// rows about to be serialized for display; the stored counters are untouched.
func overlayGlobalTraffic(db *gorm.DB, rows []*xray.ClientTraffic) {
	if len(rows) == 0 {
		return
	}
	// Cheap short-circuit for the common case (a panel no master pushes to).
	var probe int64
	if err := db.Model(&model.ClientGlobalTraffic{}).Limit(1).Count(&probe).Error; err != nil || probe == 0 {
		return
	}

	emails := make([]string, 0, len(rows))
	byEmail := make(map[string][]*xray.ClientTraffic, len(rows))
	for _, r := range rows {
		if r == nil || r.Email == "" {
			continue
		}
		key := strings.ToLower(r.Email)
		if _, ok := byEmail[key]; !ok {
			emails = append(emails, r.Email)
		}
		byEmail[key] = append(byEmail[key], r)
	}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var globals []model.ClientGlobalTraffic
		if err := db.Where("email IN ?", batch).Find(&globals).Error; err != nil {
			logger.Warning("overlayGlobalTraffic:", err)
			return
		}
		for i := range globals {
			for _, r := range byEmail[strings.ToLower(globals[i].Email)] {
				if globals[i].Up > r.Up {
					r.Up = globals[i].Up
				}
				if globals[i].Down > r.Down {
					r.Down = globals[i].Down
				}
			}
		}
	}
}

// overlayGlobalTrafficValues is overlayGlobalTraffic for value slices.
func overlayGlobalTrafficValues(db *gorm.DB, rows []xray.ClientTraffic) {
	if len(rows) == 0 {
		return
	}
	ptrs := make([]*xray.ClientTraffic, 0, len(rows))
	for i := range rows {
		ptrs = append(ptrs, &rows[i])
	}
	overlayGlobalTraffic(db, ptrs)
}

// GetNodeClientTraffics returns this panel's aggregated traffic rows for the
// clients known to live on the given node (those with a delta baseline) —
// the payload for Remote.PushGlobalClientTraffics. The rows carry the global
// overlay so a mid-chain panel forwards the widest view it has seen, not just
// its own aggregate.
func (s *InboundService) GetNodeClientTraffics(nodeID int) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var emails []string
	if err := db.Model(&model.NodeClientTraffic{}).
		Where("node_id = ?", nodeID).
		Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	if len(emails) == 0 {
		return nil, nil
	}
	out := make([]*xray.ClientTraffic, 0, len(emails))
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var page []*xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			return nil, err
		}
		out = append(out, page...)
	}
	overlayGlobalTraffic(db, out)
	return out, nil
}

// overlayInboundsClientStats applies the global overlay to every preloaded
// ClientStats row across the given inbounds. UI read paths only — never the
// full /panel/api/inbounds/list payload, which doubles as the traffic
// snapshot masters poll: overlaying that would leak pushed globals back into
// the masters' delta accounting.
func (s *InboundService) overlayInboundsClientStats(db *gorm.DB, inbounds []*model.Inbound) {
	rows := make([]*xray.ClientTraffic, 0)
	for _, ib := range inbounds {
		for j := range ib.ClientStats {
			rows = append(rows, &ib.ClientStats[j])
		}
	}
	overlayGlobalTraffic(db, rows)
}

// clearGlobalTraffic drops every master's pushed rows for the given emails.
// Used by client deletion and traffic-reset flows: after a node-local reset
// the next master push restores the master's (authoritative) numbers, and
// after a master-side reset that push carries the reset values anyway.
func clearGlobalTraffic(tx *gorm.DB, emails ...string) error {
	if len(emails) == 0 {
		return nil
	}
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		if err := tx.Where("email IN ?", batch).Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}
