package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *InboundService) GetAllInboundClientIps() ([]model.InboundClientIps, error) {
	db := database.GetDB()
	var ips []model.InboundClientIps
	err := db.Model(&model.InboundClientIps{}).Find(&ips).Error
	return ips, err
}

// nodeHostedEmails is every client one node serves, its descendants' included.
// Per-node pushes are scoped to it so their cost tracks the node, not the fleet.
func nodeHostedEmails(db *gorm.DB, nodeID int) ([]string, error) {
	var emails []string
	err := db.Model(&model.NodeClientTraffic{}).Where("node_id = ?", nodeID).Pluck("email", &emails).Error
	return emails, err
}

// GetNodeInboundClientIps returns the IP rows of the clients nodeID hosts: a node's
// IP-limit job reads no other row, so pushing the rest only made it echo them back.
func (s *InboundService) GetNodeInboundClientIps(nodeID int) ([]model.InboundClientIps, error) {
	db := database.GetDB()
	emails, err := nodeHostedEmails(db, nodeID)
	if err != nil || len(emails) == 0 {
		return nil, err
	}
	var ips []model.InboundClientIps
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var page []model.InboundClientIps
		if err := db.Where("client_email IN ?", batch).Find(&page).Error; err != nil {
			return nil, err
		}
		ips = append(ips, page...)
	}
	return ips, nil
}

// Match the scanner's 30-minute retention so sync cannot resurrect pruned IPs.
const clientIpStaleAfterSeconds = int64(30 * 60)

// mergeClientIpEntries keeps each fresh IP's maximum timestamp.
func mergeClientIpEntries(old, incoming []model.ClientIpEntry, cutoff int64) []model.ClientIpEntry {
	ipMap := make(map[string]int64, len(old)+len(incoming))
	for _, e := range old {
		if e.Timestamp < cutoff {
			continue
		}
		ipMap[e.IP] = e.Timestamp
	}
	for _, e := range incoming {
		if e.Timestamp < cutoff {
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
	return out
}

// MergeInboundClientIps unions remote observations without losing concurrent scanner writes.
func (s *InboundService) MergeInboundClientIps(incomingIps []model.InboundClientIps) error {
	db := database.GetDB()
	var currentIps []model.InboundClientIps
	if err := db.Model(&model.InboundClientIps{}).Find(&currentIps).Error; err != nil {
		return err
	}

	currentMap := make(map[string]*model.InboundClientIps, len(currentIps))
	for i := range currentIps {
		currentMap[currentIps[i].ClientEmail] = &currentIps[i]
	}

	now := time.Now().Unix()
	cutoff := now - clientIpStaleAfterSeconds

	// All sync writers take row locks in email order to avoid PostgreSQL deadlocks.
	sort.Slice(incomingIps, func(i, j int) bool {
		return incomingIps[i].ClientEmail < incomingIps[j].ClientEmail
	})

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, incoming := range incomingIps {
		if incoming.ClientEmail == "" || incoming.Ips == "" {
			continue
		}

		var incomingEntries []model.ClientIpEntry
		_ = json.Unmarshal([]byte(incoming.Ips), &incomingEntries)

		current, exists := currentMap[incoming.ClientEmail]
		if !exists {
			// A new row must contain at least one fresh observation.
			fresh := mergeClientIpEntries(nil, incomingEntries, cutoff)
			if len(fresh) == 0 {
				continue
			}
			incoming.Ips, _ = model.MarshalClientIpEntries(fresh)
			// Remote primary keys belong to an independent database.
			incoming.Id = 0
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "client_email"}},
				DoNothing: true,
			}).Create(&incoming)
			if result.Error != nil {
				tx.Rollback()
				return result.Error
			}
			if result.RowsAffected == 1 {
				currentMap[incoming.ClientEmail] = &incoming
				continue
			}
			current = &model.InboundClientIps{}
			if err := tx.Where("client_email = ?", incoming.ClientEmail).First(current).Error; err != nil {
				tx.Rollback()
				return err
			}
			currentMap[incoming.ClientEmail] = current
		}

		// check_client_ip_job rewrites this blob too; an unconditional Update loses
		// whichever writer commits second and its remote IPs with it (#6587).
		if err := mergeExistingClientIps(tx, current, incomingEntries, cutoff); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ClientIpCasRetries bounds the re-reads after losing a compare-and-set. Running
// out is an error, so the caller retries on its next schedule instead of dropping IPs.
const ClientIpCasRetries = 8

// CasUpdateInboundClientIps writes newIps only while the row still holds expectedIps;
// updated=false with a nil error means another writer won and the caller must re-read.
func CasUpdateInboundClientIps(tx *gorm.DB, id int, expectedIps, newIps string) (updated bool, err error) {
	res := tx.Model(&model.InboundClientIps{}).
		Where("id = ? AND ips = ?", id, expectedIps).
		Update("ips", newIps)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// Seed the first CAS from the preload; only a competing write requires a re-read.
func mergeExistingClientIps(tx *gorm.DB, current *model.InboundClientIps, incoming []model.ClientIpEntry, cutoff int64) error {
	for attempt := range ClientIpCasRetries {
		var oldEntries []model.ClientIpEntry
		if current.Ips != "" {
			_ = json.Unmarshal([]byte(current.Ips), &oldEntries)
		}
		merged := mergeClientIpEntries(oldEntries, incoming, cutoff)
		mergedStr, _ := model.MarshalClientIpEntries(merged)
		if current.Ips == mergedStr {
			return nil
		}
		ok, err := CasUpdateInboundClientIps(tx, current.Id, current.Ips, mergedStr)
		if err != nil {
			return err
		}
		if ok {
			current.Ips = mergedStr
			return nil
		}
		if attempt+1 < ClientIpCasRetries {
			if err := tx.Where("id = ?", current.Id).First(current).Error; err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("inbound_client_ips id=%d: exhausted CAS retries merging client IPs", current.Id)
}

func (s *InboundService) UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error {
	// The caller only renames onto a free identity, so a row already sitting on
	// newEmail is stale tracking data — drop it instead of failing the edit.
	if oldEmail != newEmail {
		if err := tx.Where("client_email = ?", newEmail).Delete(model.InboundClientIps{}).Error; err != nil {
			return err
		}
	}
	return tx.Model(model.InboundClientIps{}).Where("client_email = ?", oldEmail).Update("client_email", newEmail).Error
}

func (s *InboundService) DelClientIPs(tx *gorm.DB, email string) error {
	return tx.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

func (s *InboundService) delClientIPsByEmails(tx *gorm.DB, emails []string) error {
	const chunk = 400
	for start := 0; start < len(emails); start += chunk {
		end := min(start+chunk, len(emails))
		if err := tx.Where("client_email IN ?", emails[start:end]).Delete(model.InboundClientIps{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *InboundService) GetInboundClientIps(clientEmail string) (string, error) {
	db := database.GetDB()
	InboundClientIps := &model.InboundClientIps{}
	err := db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps).Error
	if err != nil {
		return "", err
	}

	if InboundClientIps.Ips == "" {
		return "", nil
	}

	var ipsWithTime []model.ClientIpEntry
	err = json.Unmarshal([]byte(InboundClientIps.Ips), &ipsWithTime)

	// If successfully parsed as new format, return with timestamps
	if err == nil && len(ipsWithTime) > 0 {
		return InboundClientIps.Ips, nil
	}

	// Otherwise, assume it's old format (simple string array)
	// Try to parse as simple array and convert to new format
	var oldIps []string
	err = json.Unmarshal([]byte(InboundClientIps.Ips), &oldIps)
	if err == nil && len(oldIps) > 0 {
		// Convert old format to new format with current timestamp
		newIpsWithTime := make([]model.ClientIpEntry, len(oldIps))
		for i, ip := range oldIps {
			newIpsWithTime[i] = model.ClientIpEntry{IP: ip, Timestamp: time.Now().Unix()}
		}
		result, _ := json.Marshal(newIpsWithTime)
		return string(result), nil
	}

	// Return as-is if parsing fails
	return InboundClientIps.Ips, nil
}

func (s *InboundService) ClearClientIps(clientEmail string) error {
	db := database.GetDB()

	result := db.Model(model.InboundClientIps{}).
		Where("client_email = ?", clientEmail).
		Update("ips", "")
	err := result.Error
	if err != nil {
		return err
	}
	return nil
}

// PruneStaleClientIps enforces clientIpStaleAfterSeconds for rows the online
// scan no longer rewrites: an offline client's addresses must still expire.
func (s *InboundService) PruneStaleClientIps() error {
	db := database.GetDB()
	cutoff := time.Now().Unix() - clientIpStaleAfterSeconds

	var rows []model.InboundClientIps
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		var entries []model.ClientIpEntry
		if row.Ips != "" {
			// Legacy blobs without timestamps stay untouched; the next scan rewrites them.
			if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
				continue
			}
		}
		kept := mergeClientIpEntries(nil, entries, cutoff)
		if len(kept) == 0 {
			if err := db.Delete(&model.InboundClientIps{}, row.Id).Error; err != nil {
				return err
			}
			continue
		}
		if len(kept) == len(entries) {
			continue
		}
		encoded, _ := model.MarshalClientIpEntries(kept)
		if err := db.Model(&model.InboundClientIps{}).Where("id = ?", row.Id).Update("ips", encoded).Error; err != nil {
			return err
		}
	}
	return pruneStaleNodeClientIps(cutoff)
}
