package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var nodeSnapshotBootID = uuid.NewString()

type snapshotInboundVersion struct {
	ID               int
	SnapshotRevision string
	Up               int64
	Down             int64
}

type snapshotMembershipVersion struct {
	model.ClientInbound
	SnapshotRevision string
}

func (s *InboundService) GetNodeTrafficSnapshot(ctx context.Context, userID int, knownRevision string) (*runtime.NodeTrafficSnapshotV1, error) {
	return nodeTrafficSnapshot(database.GetDB().WithContext(ctx), userID, knownRevision, s.panelGuid())
}

func nodeTrafficSnapshot(db *gorm.DB, userID int, knownRevision, guid string) (*runtime.NodeTrafficSnapshotV1, error) {
	result := &runtime.NodeTrafficSnapshotV1{Version: 1, Clients: make([]runtime.NodeClientTraffic, 0), InboundTraffic: make([]runtime.NodeInboundTraffic, 0)}
	err := readNodeSnapshot(db, func(tx *gorm.DB) error {
		var versions []snapshotInboundVersion
		if err := tx.Table("inbounds").Select("id, snapshot_revision, up, down").Where("user_id = ?", userID).Order("id").Scan(&versions).Error; err != nil {
			return err
		}
		var memberships []snapshotMembershipVersion
		if err := tx.Table("client_inbounds ci").Select("ci.*, c.snapshot_revision").Joins("JOIN clients c ON c.id = ci.client_id").Joins("JOIN inbounds ib ON ib.id = ci.inbound_id").Where("ib.user_id = ?", userID).Order("ci.inbound_id, ci.client_id").Scan(&memberships).Error; err != nil {
			return err
		}
		owned := tx.Table("inbounds").Select("id").Where("user_id = ?", userID)
		attached := tx.Table("clients c").Select("c.email").Joins("JOIN client_inbounds ci ON ci.client_id = c.id").Joins("JOIN inbounds ib ON ib.id = ci.inbound_id").Where("ib.user_id = ? AND c.email <> ?", userID, "")
		if err := tx.Model(&xray.ClientTraffic{}).Select("id, inbound_id, email, enable, up, down, total, expiry_time, reset, reset_day, reset_weekday, reset_max, reset_count, last_online").Where("inbound_id IN (?) OR email IN (?)", owned, attached).Order("id").Find(&result.Clients).Error; err != nil {
			return err
		}
		hash := sha256.New()
		encoder := json.NewEncoder(hash)
		for _, value := range []any{nodeSnapshotBootID, database.ConnectionGeneration(), guid, userID, memberships} {
			if err := encoder.Encode(value); err != nil {
				return err
			}
		}
		for _, v := range versions {
			result.InboundTraffic = append(result.InboundTraffic, runtime.NodeInboundTraffic{ID: v.ID, Up: v.Up, Down: v.Down})
			v.Up = 0
			v.Down = 0
			if err := encoder.Encode(v); err != nil {
				return err
			}
		}
		// Lifecycle changes invalidate structural reconciliation; usage and online timestamps do not.
		for _, cs := range result.Clients {
			cs.Up = 0
			cs.Down = 0
			cs.LastOnline = 0
			if err := encoder.Encode(cs); err != nil {
				return err
			}
		}
		result.Revision = hex.EncodeToString(hash.Sum(nil))
		if result.Revision != knownRevision {
			structure := &runtime.NodeSnapshotStructure{Inbounds: make([]*model.Inbound, 0), Attachments: make([]runtime.NodeSnapshotAttachment, 0)}
			if err := tx.Model(&model.Inbound{}).Where("user_id = ?", userID).Order("id").Find(&structure.Inbounds).Error; err != nil {
				return err
			}
			if err := tx.Table("client_inbounds ci").Select("ci.inbound_id, c.email").Joins("JOIN clients c ON c.id = ci.client_id").Joins("JOIN inbounds ib ON ib.id = ci.inbound_id").Where("ib.user_id = ?", userID).Order("ci.inbound_id, c.id").Scan(&structure.Attachments).Error; err != nil {
				return err
			}
			for _, ib := range structure.Inbounds {
				ib.Up = 0
				ib.Down = 0
				if ib.OriginNodeGuid == "" && ib.NodeID == nil {
					ib.OriginNodeGuid = guid
				}
			}
			result.Structure = structure
		}
		var err error
		result.LastOnlineMap, err = clientsLastOnline(tx)
		return err
	})
	return result, err
}

// SQLite's default write transaction is IMMEDIATE; snapshots need a deferred WAL reader.
func readNodeSnapshot(db *gorm.DB, read func(*gorm.DB) error) error {
	if db.Name() != database.DialectSQLite {
		return db.Transaction(read, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	}
	return db.Connection(func(conn *gorm.DB) error {
		conn = conn.Session(&gorm.Session{NewDB: true})
		if err := conn.Exec("BEGIN DEFERRED").Error; err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = conn.WithContext(context.Background()).Exec("ROLLBACK").Error
			}
		}()
		if err := read(conn); err != nil {
			return err
		}
		if err := conn.Exec("COMMIT").Error; err != nil {
			return err
		}
		committed = true
		return nil
	})
}
