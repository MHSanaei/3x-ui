package service

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm/clause"
)

// node_client_ips.go implements per-node client-IP attribution. The flat
// inbound_client_ips table is a cluster-wide union (used for IP-limit counting
// and pushed back to every node), so it cannot tell which node a given IP is
// on. NodeClientIp keeps that attribution: each panel records its own Xray
// observations under its panelGuid, and the master merges every node's
// guid-keyed report — never mixing in IPs a parent pushed down.

// mergeModelClientIpEntries unions old and incoming observations, drops anything
// older than cutoff, keeps the newest timestamp per IP, and sorts newest-first.
// It mirrors mergeClientIpEntries but operates on the exported wire type.
func mergeModelClientIpEntries(old, incoming []model.ClientIpEntry, cutoff int64) []model.ClientIpEntry {
	ipMap := make(map[string]int64, len(old)+len(incoming))
	for _, e := range old {
		if e.IP == "" || e.Timestamp < cutoff {
			continue
		}
		ipMap[e.IP] = e.Timestamp
	}
	for _, e := range incoming {
		if e.IP == "" || e.Timestamp < cutoff {
			continue
		}
		if cur, ok := ipMap[e.IP]; !ok || e.Timestamp > cur {
			ipMap[e.IP] = e.Timestamp
		}
	}
	out := make([]model.ClientIpEntry, 0, len(ipMap))
	for ip, ts := range ipMap {
		out = append(out, model.ClientIpEntry{IP: ip, Timestamp: ts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out
}

// upsertNodeClientIps folds a guid's per-email observations into NodeClientIp,
// merging with whatever is already stored for that (guid, email) and dropping
// stale entries. Empty merged results delete the row so the table stays bounded.
func upsertNodeClientIps(guid string, perEmail map[string][]model.ClientIpEntry) error {
	if guid == "" || len(perEmail) == 0 {
		return nil
	}
	db := database.GetDB()
	cutoff := time.Now().Unix() - clientIpStaleAfterSeconds

	var existing []model.NodeClientIp
	if err := db.Where("node_guid = ?", guid).Find(&existing).Error; err != nil {
		return err
	}
	existingByEmail := make(map[string]*model.NodeClientIp, len(existing))
	for i := range existing {
		existingByEmail[existing[i].Email] = &existing[i]
	}

	// Deterministic row order keeps concurrent guid merges from deadlocking on
	// Postgres (40P01) — same discipline as MergeInboundClientIps.
	emails := make([]string, 0, len(perEmail))
	for email := range perEmail {
		if email != "" {
			emails = append(emails, email)
		}
	}
	sort.Strings(emails)

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, email := range emails {
		incoming := perEmail[email]
		var old []model.ClientIpEntry
		if cur, ok := existingByEmail[email]; ok && cur.Ips != "" {
			_ = json.Unmarshal([]byte(cur.Ips), &old)
		}
		merged := mergeModelClientIpEntries(old, incoming, cutoff)
		if len(merged) == 0 {
			// Nothing fresh: drop any stale row so attribution doesn't linger.
			if _, ok := existingByEmail[email]; ok {
				if err := tx.Where("node_guid = ? AND email = ?", guid, email).
					Delete(&model.NodeClientIp{}).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
			continue
		}
		b, _ := json.Marshal(merged)
		row := model.NodeClientIp{NodeGuid: guid, Email: email, Ips: string(b)}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "node_guid"}, {Name: "email"}},
			DoUpdates: clause.AssignmentColumns([]string{"ips"}),
		}).Create(&row).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// RecordLocalClientIps stores this panel's own Xray observations under its
// panelGuid. Called by check_client_ip_job each scan with the live per-email IPs
// the local core reported.
func (s *InboundService) RecordLocalClientIps(panelGuid string, observed map[string][]model.ClientIpEntry) error {
	return upsertNodeClientIps(panelGuid, observed)
}

// MergeClientIpsByGuid folds a node's guid-keyed attribution report (its own
// panelGuid subtree plus any descendants) into the local table, preserving which
// physical node each IP is on across a chain. When node is non-nil and its own
// panelGuid is ambiguous (shared with another node or the master — a cloned
// server), the node's own subtree is remapped to its node-unique key so two
// clones don't collapse into one attribution row; descendant subtrees keep their
// distinct GUIDs. A nil node merges the report verbatim.
func (s *InboundService) MergeClientIpsByGuid(node *model.Node, trees map[string]map[string][]model.ClientIpEntry) error {
	if node != nil && node.Guid != "" {
		if eff := effectiveNodeKey(node); eff != node.Guid {
			if sub, ok := trees[node.Guid]; ok {
				delete(trees, node.Guid)
				if existing, ok := trees[eff]; ok {
					for email, ips := range sub {
						existing[email] = append(existing[email], ips...)
					}
				} else {
					trees[eff] = sub
				}
			}
		}
	}
	for guid, perEmail := range trees {
		if err := upsertNodeClientIps(guid, perEmail); err != nil {
			return err
		}
	}
	return nil
}

// GetClientIpsByGuid returns this panel's full attribution subtree (guid -> email
// -> fresh IPs), dropping stale entries. It is what the clientIpsByGuid endpoint
// serves to a parent panel.
func (s *InboundService) GetClientIpsByGuid() (map[string]map[string][]model.ClientIpEntry, error) {
	db := database.GetDB()
	var rows []model.NodeClientIp
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	cutoff := time.Now().Unix() - clientIpStaleAfterSeconds
	out := make(map[string]map[string][]model.ClientIpEntry)
	for _, row := range rows {
		if row.NodeGuid == "" || row.Email == "" || row.Ips == "" {
			continue
		}
		var entries []model.ClientIpEntry
		if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
			continue
		}
		fresh := mergeModelClientIpEntries(nil, entries, cutoff)
		if len(fresh) == 0 {
			continue
		}
		if out[row.NodeGuid] == nil {
			out[row.NodeGuid] = make(map[string][]model.ClientIpEntry)
		}
		out[row.NodeGuid][row.Email] = fresh
	}
	return out, nil
}

// GetClientIpNodeAttribution returns, for one client email, a map of IP -> the
// guid that most recently observed it (within the stale window). Used to label
// each IP in the panel with the node it is connecting to.
func (s *InboundService) GetClientIpNodeAttribution(email string) (map[string]string, error) {
	db := database.GetDB()
	var rows []model.NodeClientIp
	if err := db.Where("email = ?", email).Find(&rows).Error; err != nil {
		return nil, err
	}
	cutoff := time.Now().Unix() - clientIpStaleAfterSeconds
	ipGuid := make(map[string]string)
	ipTs := make(map[string]int64)
	for _, row := range rows {
		if row.NodeGuid == "" || row.Ips == "" {
			continue
		}
		var entries []model.ClientIpEntry
		if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
			continue
		}
		for _, e := range entries {
			if e.IP == "" || e.Timestamp < cutoff {
				continue
			}
			if cur, ok := ipTs[e.IP]; !ok || e.Timestamp > cur {
				ipTs[e.IP] = e.Timestamp
				ipGuid[e.IP] = row.NodeGuid
			}
		}
	}
	return ipGuid, nil
}

// ClientIpInfo is one IP shown in the panel's per-client IP log, labelled with
// the node it is connecting through ("" = this local panel).
type ClientIpInfo struct {
	IP   string `json:"ip"`
	Time string `json:"time"`
	Node string `json:"node"`
}

// GetClientIpsWithNodes returns a client's recorded IPs (from the flat
// inbound_client_ips display set) annotated with the node each IP is on, using
// the per-node attribution table. Local IPs (and any IP without attribution)
// carry an empty Node.
func (s *InboundService) GetClientIpsWithNodes(email string) ([]ClientIpInfo, error) {
	raw, err := s.GetInboundClientIps(email)
	if err != nil || raw == "" {
		// Record-not-found (or empty) is "no IPs", not an error for the UI.
		return []ClientIpInfo{}, nil
	}

	var entries []model.ClientIpEntry
	if jerr := json.Unmarshal([]byte(raw), &entries); jerr != nil || len(entries) == 0 {
		// Legacy shape: a plain JSON array of IP strings.
		var oldIps []string
		if json.Unmarshal([]byte(raw), &oldIps) == nil {
			entries = entries[:0]
			for _, ip := range oldIps {
				entries = append(entries, model.ClientIpEntry{IP: ip})
			}
		}
	}
	if len(entries) == 0 {
		return []ClientIpInfo{}, nil
	}

	attr, _ := s.GetClientIpNodeAttribution(email)
	guidName := s.nodeGuidNameMap()
	localGuid, _ := (&SettingService{}).GetPanelGuid()

	out := make([]ClientIpInfo, 0, len(entries))
	for _, e := range entries {
		if e.IP == "" {
			continue
		}
		info := ClientIpInfo{IP: e.IP}
		if e.Timestamp > 0 {
			info.Time = time.Unix(e.Timestamp, 0).Local().Format("2006-01-02 15:04:05")
		}
		if guid, ok := attr[e.IP]; ok && guid != "" && guid != localGuid {
			info.Node = guidName[guid]
		}
		out = append(out, info)
	}
	return out, nil
}

// nodeGuidNameMap maps each known node's attribution key to its display name,
// keyed by effectiveNodeGuid so a cloned node's IPs (stored under its node-unique
// key) still resolve to the right name instead of colliding under a shared GUID.
func (s *InboundService) nodeGuidNameMap() map[string]string {
	db := database.GetDB()
	var nodes []model.Node
	if err := db.Model(&model.Node{}).Find(&nodes).Error; err != nil {
		return map[string]string{}
	}
	ptrs := make([]*model.Node, len(nodes))
	for i := range nodes {
		ptrs[i] = &nodes[i]
	}
	selfGuid, _ := (&SettingService{}).GetPanelGuid()
	ambiguous := ambiguousNodeGuids(ptrs, selfGuid)
	m := make(map[string]string, len(nodes))
	for i := range nodes {
		m[effectiveNodeGuid(&nodes[i], ambiguous)] = nodes[i].Name
	}
	return m
}

// DeleteNodeClientIpsByGuid removes all attribution rows for a guid (e.g. when a
// node is deleted) so its IPs stop being reported and counted.
func (s *InboundService) DeleteNodeClientIpsByGuid(guid string) error {
	if guid == "" {
		return nil
	}
	db := database.GetDB()
	return db.Where("node_guid = ?", guid).Delete(&model.NodeClientIp{}).Error
}
