package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *InboundService) AddTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (needRestart bool, clientsDisabled bool, err error) {
	err = submitTrafficWrite(func() error {
		var inner error
		needRestart, clientsDisabled, inner = s.addTrafficLocked(inboundTraffics, clientTraffics)
		return inner
	})
	return
}

func (s *InboundService) addTrafficLocked(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (bool, bool, error) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback().Error; rbErr != nil {
				logger.Warning("Error rolling back traffic tx:", rbErr)
			}
		} else if cErr := tx.Commit().Error; cErr != nil {
			logger.Warning("Error committing traffic tx:", cErr)
		}
	}()
	err = s.addInboundTraffic(tx, inboundTraffics)
	if err != nil {
		return false, false, err
	}
	err = s.addClientTraffic(tx, clientTraffics)
	if err != nil {
		return false, false, err
	}

	needRestart0, count, renewErr := s.autoRenewClients(tx)
	if renewErr != nil {
		logger.Warning("Error in renew clients:", renewErr)
	} else if count > 0 {
		logger.Debugf("%v clients renewed", count)
	}

	disabledClientsCount := int64(0)
	needRestart1, count, disableClientsErr := s.disableInvalidClients(tx)
	if disableClientsErr != nil {
		logger.Warning("Error in disabling invalid clients:", disableClientsErr)
	} else if count > 0 {
		logger.Debugf("%v clients disabled", count)
		disabledClientsCount = count
	}

	needRestart2, count, disableInboundsErr := s.disableInvalidInbounds(tx)
	if disableInboundsErr != nil {
		logger.Warning("Error in disabling invalid inbounds:", disableInboundsErr)
	} else if count > 0 {
		logger.Debugf("%v inbounds disabled", count)
	}
	return needRestart0 || needRestart1 || needRestart2, disabledClientsCount > 0, nil
}

func (s *InboundService) addInboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Model(&model.Inbound{}).Where("tag = ? AND node_id IS NULL", traffic.Tag).
				Updates(map[string]any{
					"up":   gorm.Expr(database.ClampedAddExpr("up"), traffic.Up),
					"down": gorm.Expr(database.ClampedAddExpr("down"), traffic.Down),
				}).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *InboundService) addClientTraffic(tx *gorm.DB, traffics []*xray.ClientTraffic) (err error) {
	if len(traffics) == 0 {
		return nil
	}

	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		emails = append(emails, traffic.Email)
	}
	dbClientTraffics := make([]*xray.ClientTraffic, 0, len(traffics))
	// Match purely by email. client_traffics is email-keyed (one shared row per
	// email regardless of how many inbounds the client is attached to), and these
	// emails come from the local xray's report, so they always belong to a client
	// attached to a local inbound. The old `inbound_id NOT IN (node inbounds)`
	// filter dropped the local traffic of a client attached to both a node and the
	// mother inbound whenever the node inbound happened to be attached first — its
	// shared row then carried the node inbound's id (AddClientStat used to use
	// OnConflict DoNothing and never refreshed it; it now refreshes inbound_id on
	// conflict, but this filter was removed rather than relying on that ordering).
	err = tx.Model(xray.ClientTraffic{}).
		Where("email IN (?)", emails).
		Find(&dbClientTraffics).Error
	if err != nil {
		return err
	}

	// Avoid empty slice error
	if len(dbClientTraffics) == 0 {
		return nil
	}

	dbClientTraffics, convertedExpiryByEmail, err := s.adjustTraffics(tx, dbClientTraffics)
	if err != nil {
		return err
	}

	// Index by email for O(N) merge.
	trafficByEmail := make(map[string]*xray.ClientTraffic, len(traffics))
	for i := range traffics {
		if traffics[i] != nil {
			trafficByEmail[traffics[i].Email] = traffics[i]
		}
	}
	now := time.Now().UnixMilli()
	// Use atomic per-row UPDATE instead of read-modify-write Save. tx.Save
	// issues UPDATEs in slice order, which varies between concurrent callers;
	// on PostgreSQL two transactions locking the same rows in opposite order
	// deadlock. An atomic "SET up = up + ?" never holds a row lock across a
	// subsequent lock acquisition, so concurrent writers cannot deadlock.
	for _, ct := range dbClientTraffics {
		t, ok := trafficByEmail[ct.Email]
		if !ok || (t.Up == 0 && t.Down == 0) {
			continue
		}
		if err = tx.Exec(
			fmt.Sprintf(
				`UPDATE client_traffics SET up = %s, down = %s, last_online = %s WHERE email = ?`,
				database.ClampedAddExpr("up"),
				database.ClampedAddExpr("down"),
				database.GreatestExpr("last_online", "?"),
			),
			t.Up, t.Down, now, ct.Email,
		).Error; err != nil {
			logger.Warning("AddClientTraffic update data ", err)
		}
	}

	// adjustTraffics converts delayed-start rows (negative ExpiryTime → absolute
	// deadline) in-memory. Persist that conversion now since the traffic UPDATE
	// above only touches up/down/last_online. Only converted emails are written:
	// updating every polled row issued one no-op UPDATE per active client per
	// poll. Sorted order keeps concurrent writers lock-compatible on Postgres.
	for _, email := range slices.Sorted(maps.Keys(convertedExpiryByEmail)) {
		if err = tx.Exec(
			`UPDATE client_traffics SET expiry_time = ? WHERE email = ? AND expiry_time < 0`,
			convertedExpiryByEmail[email], email,
		).Error; err != nil {
			logger.Warning("AddClientTraffic update expiry_time ", err)
		}
	}

	return nil
}

func (s *InboundService) adjustTraffics(tx *gorm.DB, dbClientTraffics []*xray.ClientTraffic) ([]*xray.ClientTraffic, map[string]int64, error) {
	now := time.Now().UnixMilli()

	// "Start After First Use" stores a negative expiry (the duration). On the
	// first traffic tick it becomes an absolute deadline of now+duration. Compute
	// it once per email so every inbound the client is attached to lands on the
	// same value (recomputing per inbound would skip all but the first one).
	newExpiryByEmail := make(map[string]int64, len(dbClientTraffics))
	for traffic_index := range dbClientTraffics {
		if dbClientTraffics[traffic_index].ExpiryTime < 0 {
			newExpiryByEmail[dbClientTraffics[traffic_index].Email] = now - dbClientTraffics[traffic_index].ExpiryTime
		}
	}
	if len(newExpiryByEmail) == 0 {
		return dbClientTraffics, nil, nil
	}

	delayedEmails := make([]string, 0, len(newExpiryByEmail))
	for email := range newExpiryByEmail {
		delayedEmails = append(delayedEmails, email)
	}

	// Resolve the owning inbounds through the client_inbounds link, which is
	// authoritative. client_traffics.inbound_id goes stale when an inbound is
	// deleted and recreated, which would leave the negative expiry unconverted.
	var inboundIds []int
	err := tx.Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email IN (?)", delayedEmails).
		Distinct().
		Pluck("client_inbounds.inbound_id", &inboundIds).Error
	if err != nil {
		return nil, nil, err
	}
	if len(inboundIds) == 0 {
		return dbClientTraffics, nil, nil
	}

	var inbounds []*model.Inbound
	err = tx.Model(model.Inbound{}).Where("id IN (?)", inboundIds).Find(&inbounds).Error
	if err != nil {
		return nil, nil, err
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			var newClients []any
			for client_index := range clients {
				c := clients[client_index].(map[string]any)
				email, _ := c["email"].(string)
				if newExpiry, ok := newExpiryByEmail[email]; ok {
					c["expiryTime"] = newExpiry
					c["updated_at"] = now
				}
				if _, ok := c["created_at"]; !ok {
					c["created_at"] = now
				}
				if _, ok := c["updated_at"]; !ok {
					c["updated_at"] = now
				}
				newClients = append(newClients, any(c))
			}
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, nil, err
			}

			inbounds[inbound_index].Settings = string(modifiedSettings)
		}
	}

	for traffic_index := range dbClientTraffics {
		if newExpiry, ok := newExpiryByEmail[dbClientTraffics[traffic_index].Email]; ok {
			dbClientTraffics[traffic_index].ExpiryTime = newExpiry
		}
	}

	err = tx.Save(inbounds).Error
	if err != nil {
		logger.Warning("AddClientTraffic update inbounds ", err)
		logger.Error(inbounds)
	} else {
		for _, ib := range inbounds {
			if ib == nil {
				continue
			}
			cs, gcErr := s.GetClients(ib)
			if gcErr != nil {
				logger.Warning("AddClientTraffic sync clients: GetClients failed", gcErr)
				continue
			}
			if syncErr := s.clientService.SyncInbound(tx, ib.Id, cs); syncErr != nil {
				logger.Warning("AddClientTraffic sync clients: SyncInbound failed", syncErr)
			}
		}
	}

	return dbClientTraffics, newExpiryByEmail, nil
}

func (s *InboundService) autoRenewClients(tx *gorm.DB) (bool, int64, error) {
	// check for time expired
	var traffics []*xray.ClientTraffic
	now := time.Now().Unix() * 1000
	var err, err1 error

	// Filter to clients that have at least one local inbound. Using
	// client_traffics.inbound_id is wrong: it goes stale after an inbound is
	// deleted/recreated and always points to the first inbound the client was
	// attached to, so it could be a node inbound even when the client also has
	// local inbounds. The email-based join through client_inbounds is authoritative.
	err = tx.Model(xray.ClientTraffic{}).
		Where("reset > 0 and expiry_time > 0 and expiry_time <= ?", now).
		Where("email IN (?)", tx.Table("client_inbounds ci").
			Select("c.email").
			Joins("JOIN clients c ON c.id = ci.client_id").
			Joins("JOIN inbounds i ON i.id = ci.inbound_id").
			Where("i.node_id IS NULL")).
		Find(&traffics).Error
	if err != nil {
		return false, 0, err
	}
	// return if there is no client to renew
	if len(traffics) == 0 {
		return false, 0, nil
	}

	var inbound_ids []int
	var inbounds []*model.Inbound
	needRestart := false
	var clientsToAdd []struct {
		protocol string
		tag      string
		client   map[string]any
	}

	// Resolve the inbounds to renew through the client_inbounds link rather than
	// client_traffics.inbound_id, which goes stale after an inbound is deleted and
	// recreated and would otherwise skip the renew entirely.
	renewEmails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		renewEmails = append(renewEmails, traffic.Email)
	}
	for _, batch := range chunkStrings(renewEmails, sqliteMaxVars) {
		var ids []int
		if err = tx.Table("client_inbounds").
			Joins("JOIN clients ON clients.id = client_inbounds.client_id").
			Where("clients.email IN ?", batch).
			Distinct().
			Pluck("client_inbounds.inbound_id", &ids).Error; err != nil {
			return false, 0, err
		}
		inbound_ids = append(inbound_ids, ids...)
	}
	// Dedupe so an inbound hosting N expired clients is fetched and saved once
	// per tick instead of N times across chunk boundaries.
	inbound_ids = uniqueInts(inbound_ids)
	// Chunked to stay under SQLite's bind-variable limit when many inbounds
	// are touched in a single tick.
	for _, batch := range chunkInts(inbound_ids, sqliteMaxVars) {
		var page []*model.Inbound
		if err = tx.Model(model.Inbound{}).Where("id IN ?", batch).Find(&page).Error; err != nil {
			return false, 0, err
		}
		inbounds = append(inbounds, page...)
	}
	// Index the expired traffics by email so each client is an O(1) lookup
	// instead of a linear scan of every expired row (O(clients × expired) per
	// inbound, quadratic at scale). Pointers keep the in-place mutation below.
	trafficByEmail := make(map[string]*xray.ClientTraffic, len(traffics))
	for i := range traffics {
		trafficByEmail[traffics[i].Email] = traffics[i]
	}
	for inbound_index := range inbounds {
		settings := map[string]any{}
		_ = json.Unmarshal([]byte(inbounds[inbound_index].Settings), &settings)
		clients, _ := settings["clients"].([]any)
		if len(clients) == 0 {
			continue
		}
		for client_index := range clients {
			c := clients[client_index].(map[string]any)
			email, _ := c["email"].(string)
			traffic, ok := trafficByEmail[email]
			if !ok {
				continue
			}
			newExpiryTime := traffic.ExpiryTime
			for newExpiryTime < now {
				newExpiryTime += (int64(traffic.Reset) * 86400000)
			}
			c["expiryTime"] = newExpiryTime
			traffic.ExpiryTime = newExpiryTime
			traffic.Down = 0
			traffic.Up = 0
			if !traffic.Enable {
				traffic.Enable = true
				c["enable"] = true
				clientsToAdd = append(clientsToAdd,
					struct {
						protocol string
						tag      string
						client   map[string]any
					}{
						protocol: string(inbounds[inbound_index].Protocol),
						tag:      inbounds[inbound_index].Tag,
						client:   c,
					})
			}
			clients[client_index] = any(c)
		}
		settings["clients"] = clients
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return false, 0, err
		}
		inbounds[inbound_index].Settings = string(newSettings)
	}
	err = tx.Save(inbounds).Error
	if err != nil {
		return false, 0, err
	}
	for _, ib := range inbounds {
		if ib == nil {
			continue
		}
		cs, gcErr := s.GetClients(ib)
		if gcErr != nil {
			logger.Warning("autoRenewClients sync clients: GetClients failed", gcErr)
			continue
		}
		if syncErr := s.clientService.SyncInbound(tx, ib.Id, cs); syncErr != nil {
			logger.Warning("autoRenewClients sync clients: SyncInbound failed", syncErr)
		}
	}
	err = tx.Save(traffics).Error
	if err != nil {
		return false, 0, err
	}
	// A renewed client starts a fresh quota window: drop the cross-panel rows
	// too, or the stale pushed totals would re-deplete it immediately.
	if err = clearGlobalTraffic(tx, renewEmails...); err != nil {
		return false, 0, err
	}
	if p != nil {
		err1 = s.xrayApi.Init(p.GetAPIPort())
		if err1 != nil {
			return true, int64(len(traffics)), nil
		}
		for _, clientToAdd := range clientsToAdd {
			err1 = s.xrayApi.AddUser(clientToAdd.protocol, clientToAdd.tag, clientToAdd.client)
			if err1 != nil {
				needRestart = true
			}
		}
		s.xrayApi.Close()
	}
	return needRestart, int64(len(traffics)), nil
}

// AddClientStat inserts a per-client accounting row, or refreshes the
// config-derived columns on an email conflict. Xray reports traffic per
// email, so the surviving row also acts as the shared accumulator for
// inbounds that re-use the same identity — every call for that identity
// (one per attached inbound) carries the same enable/expiry/reset/total,
// so re-asserting them here is idempotent for that legitimate case.
//
// The conflict path matters on its own for a second reason: an inbound
// delete detaches its clients (InboundService.DelInbound) without deleting
// their client_traffics row, by design — mirroring ClientService.Detach,
// which intentionally leaves a fully-detached client's row in place so a
// later Attach can resume it with its accumulated traffic intact. If that
// same email is instead reused for a freshly (re)created client, the new
// config's enable/expiry/reset/total must win over whatever the orphaned
// row still holds; DoNothing left them stale indefinitely (#5958).
//
// up/down are deliberately excluded from the refresh: they are the
// accumulated traffic totals, and zeroing them here would erase real usage
// every time an existing, actively-used client is attached to one more
// inbound. One tradeoff this does not resolve: a genuinely new client that
// happens to reuse an orphaned email still inherits that row's leftover
// up/down, since nothing at this call site can tell the two cases apart.
func (s *InboundService) AddClientStat(tx *gorm.DB, inboundId int, client *model.Client) error {
	clientTraffic := xray.ClientTraffic{
		InboundId:  inboundId,
		Email:      client.Email,
		Total:      client.TotalGB,
		ExpiryTime: client.ExpiryTime,
		Enable:     client.Enable,
		Reset:      client.Reset,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"inbound_id", "total", "expiry_time", "enable", "reset"}),
	}).Create(&clientTraffic).Error
}

func (s *InboundService) UpdateClientStat(tx *gorm.DB, email string, client *model.Client) error {
	result := tx.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{
			"enable":      client.Enable,
			"email":       client.Email,
			"total":       client.TotalGB,
			"expiry_time": client.ExpiryTime,
			"reset":       client.Reset,
		})
	err := result.Error
	return err
}

func (s *InboundService) DelClientStat(tx *gorm.DB, email string) error {
	if err := adjustGroupBaselinesForRemovedTraffic(tx, []string{email}); err != nil {
		return err
	}
	if err := tx.Where("email = ?", email).Delete(xray.ClientTraffic{}).Error; err != nil {
		return err
	}
	if err := clearGlobalTraffic(tx, email); err != nil {
		return err
	}
	return tx.Where("email = ?", email).Delete(&model.NodeClientTraffic{}).Error
}

func (s *InboundService) delClientStatsByEmails(tx *gorm.DB, emails []string) error {
	if err := adjustGroupBaselinesForRemovedTraffic(tx, emails); err != nil {
		return err
	}
	const chunk = 400
	for start := 0; start < len(emails); start += chunk {
		end := min(start+chunk, len(emails))
		batch := emails[start:end]
		if err := tx.Where("email IN ?", batch).Delete(xray.ClientTraffic{}).Error; err != nil {
			return err
		}
		if err := tx.Where("email IN ?", batch).Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
			return err
		}
		if err := tx.Where("email IN ?", batch).Delete(&model.NodeClientTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) ResetClientTrafficByEmail(clientEmail string) error {
	err := submitTrafficWrite(func() error {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			if err := adjustGroupBaselinesForRemovedTraffic(tx, []string{clientEmail}); err != nil {
				return err
			}
			if err := clearGlobalTraffic(tx, clientEmail); err != nil {
				return err
			}
			if err := tx.Model(xray.ClientTraffic{}).
				Where("email = ?", clientEmail).
				Updates(map[string]any{"enable": true, "up": 0, "down": 0}).Error; err != nil {
				return err
			}
			return tx.Where("email = ?", clientEmail).Delete(&model.NodeClientTraffic{}).Error
		})
	})
	if err == nil {
		s.resetMtprotoClientQuota(clientEmail)
	}
	return err
}

func (s *InboundService) ResetClientTraffic(id int, clientEmail string) (needRestart bool, err error) {
	err = submitTrafficWrite(func() error {
		var inner error
		needRestart, inner = s.resetClientTrafficLocked(id, clientEmail)
		return inner
	})
	if err == nil {
		s.resetMtprotoClientQuota(clientEmail)
	}
	return
}

func (s *InboundService) resetClientTrafficLocked(id int, clientEmail string) (bool, error) {
	needRestart := false

	traffic, err := s.GetClientTrafficByEmail(clientEmail)
	if err != nil {
		return false, err
	}

	if !traffic.Enable {
		inbound, err := s.GetInbound(id)
		if err != nil {
			return false, err
		}
		clients, err := s.GetClients(inbound)
		if err != nil {
			return false, err
		}
		for _, client := range clients {
			if client.Email == clientEmail && client.Enable {
				rt, push, _, perr := s.nodePushPlan(inbound)
				if perr != nil {
					return false, perr
				}
				if !push {
					if inbound.NodeID == nil {
						needRestart = true
					}
					break
				}
				cipher := ""
				if string(inbound.Protocol) == "shadowsocks" {
					var oldSettings map[string]any
					err = json.Unmarshal([]byte(inbound.Settings), &oldSettings)
					if err != nil {
						return false, err
					}
					cipher = oldSettings["method"].(string)
				}
				err1 := rt.AddUser(context.Background(), inbound, map[string]any{
					"email":    client.Email,
					"id":       client.ID,
					"auth":     client.Auth,
					"security": client.Security,
					"flow":     client.Flow,
					"password": client.Password,
					"cipher":   cipher,
				})
				if err1 == nil {
					logger.Debug("Client enabled on", rt.Name(), "due to reset traffic:", clientEmail)
				} else if inbound.NodeID != nil {
					logger.Warning("Error in enabling client on", rt.Name(), ":", err1)
				} else {
					logger.Debug("Error in enabling client on", rt.Name(), ":", err1)
					needRestart = true
				}
				break
			}
		}
	}

	traffic.Up = 0
	traffic.Down = 0
	traffic.Enable = true

	db := database.GetDB()
	now := time.Now().UnixMilli()
	inbound, err := s.GetInbound(id)
	if err != nil {
		return false, err
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := adjustGroupBaselinesForRemovedTraffic(tx, []string{clientEmail}); err != nil {
			return err
		}
		if err := tx.Save(traffic).Error; err != nil {
			return err
		}
		if err := clearGlobalTraffic(tx, clientEmail); err != nil {
			return err
		}
		if err := tx.Where("email = ?", clientEmail).Delete(&model.NodeClientTraffic{}).Error; err != nil {
			return err
		}
		if err := tx.Model(model.Inbound{}).
			Where("id = ?", id).
			Update("last_traffic_reset_time", now).Error; err != nil {
			return err
		}
		if inbound != nil && inbound.NodeID != nil {
			return (&NodeService{}).MarkNodeDirtyTx(tx, *inbound.NodeID)
		}
		return nil
	}); err != nil {
		return false, err
	}

	if inbound != nil && inbound.NodeID != nil {
		if rt, rterr := s.runtimeFor(inbound); rterr == nil {
			if e := rt.ResetClientTraffic(context.Background(), inbound, clientEmail); e != nil {
				logger.Warning("ResetClientTraffic: remote propagation to", rt.Name(), "failed:", e)
			}
		} else {
			logger.Warning("ResetClientTraffic: runtime lookup failed:", rterr)
		}
	}

	return needRestart, nil
}

func (s *InboundService) ResetAllTraffics() error {
	err := submitTrafficWrite(func() error {
		return s.resetAllTrafficsLocked()
	})
	if err == nil {
		s.propagateResetAllTrafficsToNodes()
		s.resetAllMtprotoQuotas()
	}
	return err
}

func (s *InboundService) resetAllTrafficsLocked() error {
	db := database.GetDB()
	now := time.Now().UnixMilli()

	return db.Model(model.Inbound{}).
		Where("user_id > ?", 0).
		Updates(map[string]any{
			"up":                      0,
			"down":                    0,
			"last_traffic_reset_time": now,
		}).Error
}

// propagateResetAllTrafficsToNodes tells every node to zero its own counters.
// Kept OUT of the traffic-writer transaction: each remote call can block up to
// remoteHTTPTimeout, and holding the single serial writer across N such calls
// stalls traffic accounting and drops the deltas of every concurrent poll.
func (s *InboundService) propagateResetAllTrafficsToNodes() {
	nodes, err := (&NodeService{}).GetAll()
	if err != nil {
		return
	}
	for _, node := range nodes {
		if rt, err := runtime.GetManager().RuntimeFor(&node.Id); err == nil {
			if e := rt.ResetAllTraffics(context.Background()); e != nil {
				logger.Warning("ResetAllTraffics: remote propagation to", rt.Name(), "failed:", e)
			}
		}
	}
}

func (s *InboundService) ResetInboundTraffic(id int) error {
	if err := submitTrafficWrite(func() error {
		return database.GetDB().Model(model.Inbound{}).
			Where("id = ?", id).
			Updates(map[string]any{"up": 0, "down": 0}).Error
	}); err != nil {
		return err
	}

	inbound, err := s.GetInbound(id)
	if err == nil && inbound != nil && inbound.NodeID != nil {
		if rt, rterr := s.runtimeFor(inbound); rterr == nil {
			if e := rt.ResetInboundTraffic(context.Background(), inbound); e != nil {
				logger.Warning("ResetInboundTraffic: remote propagation to", rt.Name(), "failed:", e)
			}
		} else {
			logger.Warning("ResetInboundTraffic: runtime lookup failed:", rterr)
		}
	}
	return nil
}

func (s *InboundService) DelDepletedClients(id int) (err error) {
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	// Collect depleted emails globally — a shared-email row owned by one
	// inbound depletes every sibling that lists the email.
	now := time.Now().Unix() * 1000
	depletedClause := "reset = 0 and ((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?))"
	var depletedRows []xray.ClientTraffic
	err = db.Model(xray.ClientTraffic{}).
		Where(depletedClause, now).
		Find(&depletedRows).Error
	if err != nil {
		return err
	}
	if len(depletedRows) == 0 {
		return nil
	}

	depletedEmails := make(map[string]struct{}, len(depletedRows))
	for _, r := range depletedRows {
		if r.Email == "" {
			continue
		}
		depletedEmails[strings.ToLower(r.Email)] = struct{}{}
	}
	if len(depletedEmails) == 0 {
		return nil
	}

	var inbounds []*model.Inbound
	inboundQuery := db.Model(model.Inbound{})
	if id >= 0 {
		inboundQuery = inboundQuery.Where("id = ?", id)
	}
	if err = inboundQuery.Find(&inbounds).Error; err != nil {
		return err
	}

	for _, inbound := range inbounds {
		var settings map[string]any
		if err = json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			return err
		}
		rawClients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		newClients := make([]any, 0, len(rawClients))
		removed := 0
		for _, client := range rawClients {
			c, ok := client.(map[string]any)
			if !ok {
				newClients = append(newClients, client)
				continue
			}
			email, _ := c["email"].(string)
			if _, isDepleted := depletedEmails[strings.ToLower(email)]; isDepleted {
				removed++
				continue
			}
			newClients = append(newClients, client)
		}
		if removed == 0 {
			continue
		}
		if len(newClients) == 0 {
			_, _ = s.DelInbound(inbound.Id)
			continue
		}
		settings["clients"] = newClients
		ns, mErr := json.MarshalIndent(settings, "", "  ")
		if mErr != nil {
			return mErr
		}
		inbound.Settings = string(ns)
		if err = tx.Save(inbound).Error; err != nil {
			return err
		}
		survivingClients, gcErr := s.GetClients(inbound)
		if gcErr != nil {
			err = gcErr
			return err
		}
		if err = s.clientService.SyncInbound(tx, inbound.Id, survivingClients); err != nil {
			return err
		}
	}

	// Drop now-orphaned rows. With id >= 0, a row is safe to drop only when
	// no out-of-scope inbound still references the email.
	if id < 0 {
		err = tx.Where(depletedClause, now).Delete(xray.ClientTraffic{}).Error
		return err
	}
	emails := make([]string, 0, len(depletedEmails))
	for e := range depletedEmails {
		emails = append(emails, e)
	}
	var stillReferenced []string
	emailExpr := database.JSONFieldText("client.value", "email")
	stillQuery := fmt.Sprintf(
		"SELECT DISTINCT LOWER(%s) %s WHERE LOWER(%s) IN ?",
		emailExpr,
		database.JSONClientsFromInbound(),
		emailExpr,
	)
	if err = tx.Raw(stillQuery, emails).Scan(&stillReferenced).Error; err != nil {
		return err
	}
	stillSet := make(map[string]struct{}, len(stillReferenced))
	for _, e := range stillReferenced {
		stillSet[e] = struct{}{}
	}
	toDelete := make([]string, 0, len(emails))
	for _, e := range emails {
		if _, kept := stillSet[e]; !kept {
			toDelete = append(toDelete, e)
		}
	}
	if len(toDelete) > 0 {
		if err = tx.Where("LOWER(email) IN ?", toDelete).Delete(xray.ClientTraffic{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) GetClientTrafficTgBot(tgId int64) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()

	idQuery := fmt.Sprintf(
		"SELECT DISTINCT inbounds.id %s WHERE %s = ?",
		database.JSONClientsFromInbound(),
		database.JSONFieldText("client.value", "tgId"),
	)
	var inboundIds []int
	if err := db.Raw(idQuery, strconv.FormatInt(tgId, 10)).Scan(&inboundIds).Error; err != nil {
		logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
		return nil, err
	}

	var inbounds []*model.Inbound
	if len(inboundIds) > 0 {
		err := db.Model(model.Inbound{}).Where("id IN ?", inboundIds).Find(&inbounds).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Errorf("Error retrieving inbounds with tgId %d: %v", tgId, err)
			return nil, err
		}
	}

	var emails []string
	for _, inbound := range inbounds {
		clients, err := s.GetClients(inbound)
		if err != nil {
			logger.Errorf("Error retrieving clients for inbound %d: %v", inbound.Id, err)
			continue
		}
		for _, client := range clients {
			if client.TgID == tgId {
				emails = append(emails, client.Email)
			}
		}
	}

	// Chunked to stay under SQLite's bind-variable limit when a single Telegram
	// account owns thousands of clients across inbounds.
	uniqEmails := uniqueNonEmptyStrings(emails)
	traffics := make([]*xray.ClientTraffic, 0, len(uniqEmails))
	for _, batch := range chunkStrings(uniqEmails, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			logger.Errorf("Error retrieving ClientTraffic for emails %v: %v", batch, err)
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	if len(traffics) == 0 {
		logger.Warning("No ClientTraffic records found for emails:", emails)
		return nil, nil
	}

	// Populate UUID and other client data for each traffic record
	for i := range traffics {
		if ct, client, e := s.GetClientByEmail(traffics[i].Email); e == nil && ct != nil && client != nil {
			traffics[i].Enable = client.Enable
			traffics[i].UUID = client.ID
			traffics[i].SubId = client.SubID
		}
	}

	return traffics, nil
}

// BumpClientsLastOnline sets client_traffics.last_online to now for the given
// emails. Used in online-API mode for clients that hold a live connection but
// moved no bytes this poll — the traffic path (addClientTraffic) only bumps
// last_online on a non-zero delta, so idle-but-connected clients would
// otherwise show a stale "last online" while being reported online.
func (s *InboundService) BumpClientsLastOnline(emails []string) error {
	uniq := uniqueNonEmptyStrings(emails)
	if len(uniq) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	return submitTrafficWrite(func() error {
		db := database.GetDB()
		for _, batch := range chunkStrings(uniq, sqliteMaxVars) {
			if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Update("last_online", now).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *InboundService) GetActiveClientTraffics(emails []string) ([]*xray.ClientTraffic, error) {
	uniq := uniqueNonEmptyStrings(emails)
	if len(uniq) == 0 {
		return nil, nil
	}
	db := database.GetDB()
	traffics := make([]*xray.ClientTraffic, 0, len(uniq))
	for _, batch := range chunkStrings(uniq, sqliteMaxVars) {
		var page []*xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Find(&page).Error; err != nil {
			return nil, err
		}
		traffics = append(traffics, page...)
	}
	overlayGlobalTraffic(db, traffics)
	return traffics, nil
}

// GetAllClientTraffics returns the full set of client_traffics rows so the
// websocket broadcasters can ship a complete snapshot every cycle. A pure
// delta path silently dropped the per-client section whenever no client moved
// bytes in the cycle or a node sync failed, leaving client rows in the UI
// stuck at stale numbers — so small installs broadcast this snapshot, and only
// above the traffic job's snapshot threshold (where the marshaled snapshot
// would exceed the hub's payload cap and be dropped wholesale) does the job
// fall back to active-row deltas.
func (s *InboundService) GetAllClientTraffics() ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Find(&traffics).Error; err != nil {
		return nil, err
	}
	overlayGlobalTraffic(db, traffics)
	return traffics, nil
}

func (s *InboundService) CountClientTraffics() (int64, error) {
	db := database.GetDB()
	var count int64
	err := db.Model(xray.ClientTraffic{}).Count(&count).Error
	return count, err
}

type InboundTrafficSummary struct {
	Id     int   `json:"id"`
	Up     int64 `json:"up"`
	Down   int64 `json:"down"`
	Total  int64 `json:"total"`
	Enable bool  `json:"enable"`
}

func (s *InboundService) GetInboundsTrafficSummary() ([]InboundTrafficSummary, error) {
	db := database.GetDB()
	var summaries []InboundTrafficSummary
	if err := db.Model(&model.Inbound{}).
		Select("id, up, down, total, enable").
		Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *InboundService) GetClientTrafficByEmail(email string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error; err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	if len(traffics) == 0 {
		return nil, nil
	}
	overlayGlobalTraffic(db, traffics)
	t := traffics[0]

	if rec, rErr := s.clientService.GetRecordByEmail(db, email); rErr == nil && rec != nil {
		c := rec.ToClient()
		t.UUID = c.ID
		t.SubId = c.SubID
		return t, nil
	}

	t2, client, err := s.GetClientByEmail(email)
	if err != nil {
		logger.Warningf("Error retrieving ClientTraffic with email %s: %v", email, err)
		return nil, err
	}
	if t2 != nil && client != nil {
		t2.UUID = client.ID
		t2.SubId = client.SubID
		return t2, nil
	}
	return nil, nil
}

func (s *InboundService) UpdateClientTrafficByEmail(email string, upload int64, download int64) error {
	return submitTrafficWrite(func() error {
		db := database.GetDB()
		err := db.Model(xray.ClientTraffic{}).
			Where("email = ?", email).
			Updates(map[string]any{
				"up":   upload,
				"down": download,
			}).Error
		if err != nil {
			logger.Warningf("Error updating ClientTraffic with email %s: %v", email, err)
		}
		return err
	})
}

func (s *InboundService) SearchClientTraffic(query string) (traffic *xray.ClientTraffic, err error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	traffic = &xray.ClientTraffic{}

	// Search for inbound settings that contain the query
	err = db.Model(model.Inbound{}).Where("settings LIKE ?", "%\""+query+"\"%").First(inbound).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warningf("Inbound settings containing query %s not found: %v", query, err)
			return nil, err
		}
		logger.Errorf("Error searching for inbound settings with query %s: %v", query, err)
		return nil, err
	}

	traffic.InboundId = inbound.Id

	clients, err := ParseInboundSettingsClients(inbound.Settings)
	if err != nil {
		logger.Errorf("Error unmarshalling inbound settings for inbound ID %d: %v", inbound.Id, err)
		return nil, err
	}

	for _, client := range clients {
		if (client.ID == query || client.Password == query) && client.Email != "" {
			traffic.Email = client.Email
			break
		}
	}

	if traffic.Email == "" {
		logger.Warningf("No client found with query %s in inbound ID %d", query, inbound.Id)
		return nil, gorm.ErrRecordNotFound
	}

	// Retrieve ClientTraffic based on the found email
	err = db.Model(xray.ClientTraffic{}).Where("email = ?", traffic.Email).First(traffic).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warningf("ClientTraffic for email %s not found: %v", traffic.Email, err)
			return nil, err
		}
		logger.Errorf("Error retrieving ClientTraffic for email %s: %v", traffic.Email, err)
		return nil, err
	}

	return traffic, nil
}
