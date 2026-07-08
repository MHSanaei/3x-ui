package service

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"gorm.io/gorm"
)

// Short-lived tombstone of just-deleted client emails so that a node snapshot
// arriving between delete and node-side processing doesn't resurrect them.
var (
	recentlyDeletedMu sync.Mutex
	recentlyDeleted   = map[string]time.Time{}
)

const deleteTombstoneTTL = 90 * time.Second

var (
	inboundMutationLocksMu sync.Mutex
	inboundMutationLocks   = map[int]*sync.Mutex{}
)

func lockInbound(inboundId int) *sync.Mutex {
	inboundMutationLocksMu.Lock()
	defer inboundMutationLocksMu.Unlock()
	m, ok := inboundMutationLocks[inboundId]
	if !ok {
		m = &sync.Mutex{}
		inboundMutationLocks[inboundId] = m
	}
	m.Lock()
	return m
}

func compactOrphans(db *gorm.DB, clients []any) []any {
	if len(clients) == 0 {
		return clients
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if e, _ := cm["email"].(string); e != "" {
			emails = append(emails, e)
		}
	}
	if len(emails) == 0 {
		return clients
	}
	existing := make(map[string]struct{}, len(emails))
	const orphanChunk = 400
	for start := 0; start < len(emails); start += orphanChunk {
		end := min(start+orphanChunk, len(emails))
		var found []string
		if err := db.Model(&model.ClientRecord{}).Where("email IN ?", emails[start:end]).Pluck("email", &found).Error; err != nil {
			logger.Warning("compactOrphans pluck:", err)
			return clients
		}
		for _, e := range found {
			existing[e] = struct{}{}
		}
	}
	if len(existing) == len(emails) {
		return clients
	}
	out := make([]any, 0, len(existing))
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			out = append(out, c)
			continue
		}
		e, _ := cm["email"].(string)
		if e == "" {
			out = append(out, c)
			continue
		}
		if _, ok := existing[e]; ok {
			out = append(out, c)
		}
	}
	return out
}

func tombstoneClientEmail(email string) {
	if email == "" {
		return
	}
	recentlyDeletedMu.Lock()
	defer recentlyDeletedMu.Unlock()
	recentlyDeleted[email] = time.Now()
	cutoff := time.Now().Add(-deleteTombstoneTTL)
	for e, ts := range recentlyDeleted {
		if ts.Before(cutoff) {
			delete(recentlyDeleted, e)
		}
	}
}

func tombstoneClientEmails(emails []string) {
	if len(emails) == 0 {
		return
	}
	now := time.Now()
	cutoff := now.Add(-deleteTombstoneTTL)
	recentlyDeletedMu.Lock()
	defer recentlyDeletedMu.Unlock()
	for _, email := range emails {
		if email != "" {
			recentlyDeleted[email] = now
		}
	}
	for e, ts := range recentlyDeleted {
		if ts.Before(cutoff) {
			delete(recentlyDeleted, e)
		}
	}
}

func isClientEmailTombstoned(email string) bool {
	if email == "" {
		return false
	}
	recentlyDeletedMu.Lock()
	defer recentlyDeletedMu.Unlock()
	ts, ok := recentlyDeleted[email]
	if !ok {
		return false
	}
	if time.Since(ts) > deleteTombstoneTTL {
		delete(recentlyDeleted, email)
		return false
	}
	return true
}

// dedupeSettingsClients collapses duplicate same-email client entries inside a
// settings JSON blob, keeping the first occurrence. Node snapshots produced by
// builds without the addInboundClient duplicate guard can carry duplicates
// (#5770); adopting them verbatim would copy the duplication into the central
// inbound. Returns the filtered JSON and whether anything was removed.
func dedupeSettingsClients(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	clients, _ := parsed["clients"].([]any)
	if len(clients) < 2 {
		return settings, false
	}
	seen := make(map[string]struct{}, len(clients))
	kept := make([]any, 0, len(clients))
	for _, c := range clients {
		if cm, ok := c.(map[string]any); ok {
			if email, _ := cm["email"].(string); email != "" {
				key := strings.ToLower(email)
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
			}
		}
		kept = append(kept, c)
	}
	if len(kept) == len(clients) {
		return settings, false
	}
	parsed["clients"] = kept
	b, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(b), true
}

// stripTombstonedClients drops just-deleted client entries from a node
// snapshot's settings JSON so adopting a stale snapshot can't re-add them to
// the central inbound while the delete tombstone is live. Returns the filtered
// JSON and whether anything was removed.
func stripTombstonedClients(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	clients, _ := parsed["clients"].([]any)
	if len(clients) == 0 {
		return settings, false
	}
	kept := make([]any, 0, len(clients))
	for _, c := range clients {
		if cm, ok := c.(map[string]any); ok {
			if email, _ := cm["email"].(string); email != "" && isClientEmailTombstoned(email) {
				continue
			}
		}
		kept = append(kept, c)
	}
	if len(kept) == len(clients) {
		return settings, false
	}
	parsed["clients"] = kept
	b, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(b), true
}
