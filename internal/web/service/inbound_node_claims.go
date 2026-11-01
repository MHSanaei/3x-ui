package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"

	"gorm.io/gorm"
)

// reportedRefusedClaims holds, per node, the emails its last snapshot was refused: a
// node chooses them, so only the latest set is kept and a claim logs once while it lasts.
var reportedRefusedClaims sync.Map

// nodeClientClaims is what one node snapshot may do to the master's client records: a node
// changes only records linked solely to its own inbounds, plus the new ones it creates.
type nodeClientClaims struct {
	refused map[string]string
	// shared records are also served by the master or another node, so they keep the
	// master's values except nodeKeptSharedKeys.
	shared map[string]*model.ClientRecord
	// storedSubID keeps an own record's subId when the node reports one held by a
	// client it does not host.
	storedSubID map[string]string
	// usageOnly records live only outside this node but share the email: the node's usage
	// still adds to them (cross-panel totals), its lifecycle and record values do not.
	usageOnly map[string]struct{}
}

// nodeKeptSharedKeys stay as the node reported them: per-link values, and the lifecycle
// the merge already settles against the master's traffic row.
var nodeKeptSharedKeys = []string{"flow", "privateKey", "publicKey", "allowedIPs", "preSharedKey", "keepAlive", "enable", "expiryTime"}

type clientLinkScope struct{ here, elsewhere bool }

func loadNodeClientClaims(tx *gorm.DB, nodeID int, snap *runtime.TrafficSnapshot) (*nodeClientClaims, error) {
	claims := &nodeClientClaims{
		refused:     map[string]string{},
		shared:      map[string]*model.ClientRecord{},
		storedSubID: map[string]string{},
		usageOnly:   map[string]struct{}{},
	}
	subIDByEmail := map[string]string{}
	// A subId selects a whole subscription, so within one snapshot it may name one client only.
	firstEmailBySub := map[string]string{}
	sharesSubID := map[string]struct{}{}
	for _, snapIb := range snap.Inbounds {
		if snapIb == nil {
			continue
		}
		_, entries, ok := splitSettingsClients(snapIb.Settings)
		if !ok {
			continue
		}
		for _, entry := range entries {
			c, ok := readSettingsClient(entry)
			if !ok || c.Email == "" {
				continue
			}
			if c.Email != strings.TrimSpace(c.Email) {
				claims.refused[c.Email] = "its email carries surrounding white space"
				continue
			}
			if _, seen := subIDByEmail[c.Email]; !seen {
				var subID string
				_ = json.Unmarshal(c.SubID, &subID)
				subIDByEmail[c.Email] = subID
				if first, taken := firstEmailBySub[subID]; subID != "" && taken && !strings.EqualFold(first, c.Email) {
					sharesSubID[c.Email] = struct{}{}
				} else if subID != "" && !taken {
					firstEmailBySub[subID] = c.Email
				}
			}
		}
	}
	if len(subIDByEmail) == 0 {
		logRefusedClaims(nodeID, claims.refused)
		return claims, nil
	}

	reported := make([]string, 0, len(subIDByEmail))
	for email := range subIDByEmail {
		reported = append(reported, email)
	}
	// SQLite's LOWER folds ASCII only, so a non-ASCII capital is matched by its spellings too.
	var records []model.ClientRecord
	for _, batch := range chunkStrings(emailSpellings(reported), sqlInChunk) {
		var rows []model.ClientRecord
		if err := tx.Where("LOWER(email) IN ? OR email IN ?", batch, batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		records = append(records, rows...)
	}
	byEmail := make(map[string]*model.ClientRecord, len(records))
	byLower := make(map[string]string, len(records))
	for i := range records {
		byEmail[records[i].Email] = &records[i]
		byLower[strings.ToLower(records[i].Email)] = records[i].Email
	}

	subIDs := make([]string, 0, len(subIDByEmail))
	for _, subID := range subIDByEmail {
		if subID != "" {
			subIDs = append(subIDs, subID)
		}
	}
	var holders []model.ClientRecord
	for _, batch := range chunkStrings(uniqueNonEmptyStrings(subIDs), sqlInChunk) {
		var rows []model.ClientRecord
		if err := tx.Select("id", "email", "sub_id").Where("sub_id IN ?", batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		holders = append(holders, rows...)
	}

	ids := make([]int, 0, len(records))
	for i := range records {
		ids = append(ids, records[i].Id)
	}
	scopes, err := clientLinkScopes(tx, nodeID, ids)
	if err != nil {
		return nil, err
	}
	holdersBySub := make(map[string][]model.ClientRecord, len(holders))
	for _, h := range holders {
		holdersBySub[h.SubID] = append(holdersBySub[h.SubID], h)
	}
	subIDTaken := func(email, subID string) bool {
		if _, shared := sharesSubID[email]; shared {
			return true
		}
		for _, h := range holdersBySub[subID] {
			if !strings.EqualFold(h.Email, email) {
				return true
			}
		}
		return false
	}

	for email, subID := range subIDByEmail {
		rec, exact := byEmail[email]
		if !exact {
			if _, variant := byLower[strings.ToLower(email)]; variant {
				claims.refused[email] = "another client's email differs from it only in case"
			} else if subID != "" && subIDTaken(email, subID) {
				claims.refused[email] = "its subId belongs to another client"
			}
			continue
		}
		scope := scopes[rec.Id]
		switch {
		case scope.elsewhere && scope.here:
			claims.shared[email] = rec
		case scope.elsewhere:
			claims.refused[email] = "it is attached only to inbounds this node does not host"
			claims.usageOnly[email] = struct{}{}
		// A mark from before sync_orphaned_by existed names no node, so any node may take it back.
		case !scope.here && (rec.SyncOrphanedAt == 0 || (rec.SyncOrphanedBy != 0 && rec.SyncOrphanedBy != nodeID)):
			claims.refused[email] = "it is detached from every inbound on the master"
		case subID != rec.SubID && subID != "" && subIDTaken(email, subID):
			claims.storedSubID[email] = rec.SubID
		}
	}
	logRefusedClaims(nodeID, claims.refused)
	return claims, nil
}

func logRefusedClaims(nodeID int, refused map[string]string) {
	prev, _ := reportedRefusedClaims.Load(nodeID)
	logged, _ := prev.(map[string]struct{})
	current := make(map[string]struct{}, len(refused))
	for email, reason := range refused {
		current[email] = struct{}{}
		if _, seen := logged[email]; !seen {
			logger.Warningf("setRemoteTraffic: node %d reported client %.64q, not adopting: %s", nodeID, email, reason)
		}
	}
	reportedRefusedClaims.Store(nodeID, current)
}

// clientLinkScopes reports, per client id, whether it is linked to an inbound of this node
// and whether to any inbound outside it (the master's own or another node's).
func clientLinkScopes(tx *gorm.DB, nodeID int, ids []int) (map[int]clientLinkScope, error) {
	scopes := make(map[int]clientLinkScope, len(ids))
	for _, batch := range chunkInts(uniqueInts(ids), sqlInChunk) {
		var rows []struct {
			ClientID int
			NodeID   *int
		}
		if err := tx.Table("client_inbounds").
			Select("client_inbounds.client_id AS client_id, inbounds.node_id AS node_id").
			Joins("JOIN inbounds ON inbounds.id = client_inbounds.inbound_id").
			Where("client_inbounds.client_id IN ?", batch).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			s := scopes[r.ClientID]
			if r.NodeID != nil && *r.NodeID == nodeID {
				s.here = true
			} else {
				s.elsewhere = true
			}
			scopes[r.ClientID] = s
		}
	}
	return scopes, nil
}

// apply rewrites a snapshot inbound's settings to what this node may claim.
func (c *nodeClientClaims) apply(settings string) string {
	if len(c.refused) == 0 && len(c.shared) == 0 && len(c.storedSubID) == 0 {
		return settings
	}
	members, entries, ok := splitSettingsClients(settings)
	if !ok {
		return settings
	}
	changed := false
	kept := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		sc, ok := readSettingsClient(entry)
		if !ok || sc.Email == "" {
			kept = append(kept, entry)
			continue
		}
		if _, refused := c.refused[sc.Email]; refused {
			changed = true
			continue
		}
		var override map[string]json.RawMessage
		if rec, shared := c.shared[sc.Email]; shared {
			override = sharedClientEntry(rec, entry)
		} else if subID, keep := c.storedSubID[sc.Email]; keep {
			if json.Unmarshal(entry, &override) == nil {
				override["subId"], _ = json.Marshal(subID)
			}
		}
		if override == nil {
			kept = append(kept, entry)
			continue
		}
		rewritten, err := json.Marshal(override)
		if err != nil {
			kept = append(kept, entry)
			continue
		}
		changed = true
		kept = append(kept, rewritten)
	}
	if !changed {
		return settings
	}
	out, _ := joinSettingsClients(settings, members, kept)
	return out
}

// sharedClientEntry is nil when the node's entry already agrees with the record on every
// master-owned value, so an honest node's differing timestamps cause no re-push.
func sharedClientEntry(rec *model.ClientRecord, nodeEntry json.RawMessage) map[string]json.RawMessage {
	var reported model.Client
	if json.Unmarshal(nodeEntry, &reported) != nil {
		return nil
	}
	if !sharedRecordWouldChange(rec, &reported) {
		return nil
	}
	var fromRecord, fromNode map[string]json.RawMessage
	raw, err := json.Marshal(rec.ToClient())
	if err != nil || json.Unmarshal(raw, &fromRecord) != nil || json.Unmarshal(nodeEntry, &fromNode) != nil {
		return nil
	}
	for _, key := range nodeKeptSharedKeys {
		if v, ok := fromNode[key]; ok {
			fromRecord[key] = v
		} else {
			delete(fromRecord, key)
		}
	}
	return fromRecord
}

// sharedRecordWouldChange merges the reported client onto a copy of the record the way
// SyncInbound does, leaving out what the node may set and what the master restores anyway.
func sharedRecordWouldChange(rec *model.ClientRecord, reported *model.Client) bool {
	incoming := reported.ToRecord()
	incoming.Enable = rec.Enable
	incoming.ExpiryTime = rec.ExpiryTime
	incoming.Comment = rec.Comment
	incoming.CreatedAt = 0
	merged := *rec
	applyClientRecordMerge(&merged, incoming)
	keepLinkOwnedFields(&merged, rec, false)
	return !reflect.DeepEqual(merged, *rec)
}

// A node's stream, sniffing and remark are rendered into every subscription link of the
// inbound, once per client and external proxy, so their size bounds a request's cost.
const (
	maxNodeStreamSettingsBytes = 64 << 10
	maxNodeSniffingBytes       = 16 << 10
	maxNodeRemarkBytes         = 1 << 10
	maxNodeExternalProxies     = 64
)

// reportedOversizeInbounds holds, per node, the tags its last snapshot left out as oversized.
var reportedOversizeInbounds sync.Map

// oversizeNodeInbound names the field that keeps a reported inbound from being taken in.
func oversizeNodeInbound(ib *model.Inbound) string {
	switch {
	case len(ib.StreamSettings) > maxNodeStreamSettingsBytes:
		return fmt.Sprintf("streamSettings exceeds %d bytes", maxNodeStreamSettingsBytes)
	case len(ib.Sniffing) > maxNodeSniffingBytes:
		return fmt.Sprintf("sniffing exceeds %d bytes", maxNodeSniffingBytes)
	case len(ib.Remark) > maxNodeRemarkBytes:
		return fmt.Sprintf("remark exceeds %d bytes", maxNodeRemarkBytes)
	}
	var stream struct {
		ExternalProxy []json.RawMessage `json:"externalProxy"`
	}
	if json.Unmarshal([]byte(ib.StreamSettings), &stream) == nil && len(stream.ExternalProxy) > maxNodeExternalProxies {
		return fmt.Sprintf("more than %d external proxies", maxNodeExternalProxies)
	}
	return ""
}

func logOversizeNodeInbounds(nodeID int, tags map[string]string) {
	prev, _ := reportedOversizeInbounds.Load(nodeID)
	logged, _ := prev.(map[string]string)
	for tag, reason := range tags {
		if _, seen := logged[tag]; !seen {
			logger.Warningf("setRemoteTraffic: node %d reported inbound %.64q, not taking it in: %s", nodeID, tag, reason)
		}
	}
	reportedOversizeInbounds.Store(nodeID, tags)
}

// usageOnlyIn lists, lowercased as the stats merge keys them, the usage-only emails a
// snapshot inbound's own settings carry.
func (c *nodeClientClaims) usageOnlyIn(settings string) map[string]struct{} {
	out := map[string]struct{}{}
	if len(c.usageOnly) == 0 {
		return out
	}
	_, entries, ok := splitSettingsClients(settings)
	if !ok {
		return out
	}
	for _, entry := range entries {
		if sc, ok := readSettingsClient(entry); ok {
			if _, only := c.usageOnly[sc.Email]; only {
				out[strings.ToLower(sc.Email)] = struct{}{}
			}
		}
	}
	return out
}
