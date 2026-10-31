package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

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
	m, ok := inboundMutationLocks[inboundId]
	if !ok {
		m = &sync.Mutex{}
		inboundMutationLocks[inboundId] = m
	}
	inboundMutationLocksMu.Unlock()
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

func withdrawClientTombstones(emails ...string) {
	if len(emails) == 0 {
		return
	}
	recentlyDeletedMu.Lock()
	defer recentlyDeletedMu.Unlock()
	for _, email := range emails {
		if email != "" {
			delete(recentlyDeleted, email)
		}
	}
}

// liftTombstonesOnCommit withdraws the delete tombstones of clients a serialized
// tx re-created once it commits; a rolled-back create leaves them standing.
func liftTombstonesOnCommit(tx *gorm.DB, created []*model.ClientRecord) {
	state := serializedTxOf(tx)
	if state == nil {
		return
	}
	for _, rec := range created {
		state.createdClientEmails = append(state.createdClientEmails, rec.Email)
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

// settingsClientEntry is what these rewrites read of a client entry. A node's blob may
// nest any value deeply, so entries stay raw text and only these fields are decoded.
type settingsClientEntry struct {
	Email      string          `json:"email"`
	SubID      json.RawMessage `json:"subId"`
	ExpiryTime json.RawMessage `json:"expiryTime"`
	Enable     json.RawMessage `json:"enable"`
}

// splitSettingsClients decodes a settings blob one level deep; ok is false when it has
// no client array to work on.
func splitSettingsClients(settings string) (members map[string]json.RawMessage, entries []json.RawMessage, ok bool) {
	if settings == "" || json.Unmarshal([]byte(settings), &members) != nil {
		return nil, nil, false
	}
	if json.Unmarshal(members["clients"], &entries) != nil {
		return nil, nil, false
	}
	return members, entries, true
}

func readSettingsClient(entry json.RawMessage) (settingsClientEntry, bool) {
	var c settingsClientEntry
	if len(entry) == 0 || entry[0] != '{' || json.Unmarshal(entry, &c) != nil {
		return c, false
	}
	return c, true
}

// joinSettingsClients writes the blob back compactly; indenting it again would cost the
// square of whatever depth the node nested into it.
func joinSettingsClients(settings string, members map[string]json.RawMessage, entries []json.RawMessage) (string, bool) {
	clients, err := json.Marshal(entries)
	if err != nil {
		return settings, false
	}
	members["clients"] = clients
	b, err := json.Marshal(members)
	if err != nil {
		return settings, false
	}
	return string(b), true
}

// dedupeSettingsClients collapses duplicate same-email client entries inside a
// settings JSON blob, keeping the first occurrence. Node snapshots produced by
// builds without the addInboundClient duplicate guard can carry duplicates
// (#5770); adopting them verbatim would copy the duplication into the central
// inbound. Returns the filtered JSON and whether anything was removed.
func dedupeSettingsClients(settings string) (string, bool) {
	members, entries, ok := splitSettingsClients(settings)
	if !ok || len(entries) < 2 {
		return settings, false
	}
	seen := make(map[string]struct{}, len(entries))
	kept := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		if c, ok := readSettingsClient(entry); ok && c.Email != "" {
			key := strings.ToLower(c.Email)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
		}
		kept = append(kept, entry)
	}
	if len(kept) == len(entries) {
		return settings, false
	}
	return joinSettingsClients(settings, members, kept)
}

// stripTombstonedClients drops just-deleted client entries from a node
// snapshot's settings JSON so adopting a stale snapshot can't re-add them to
// the central inbound while the delete tombstone is live. Returns the filtered
// JSON and whether anything was removed.
func stripTombstonedClients(settings string) (string, bool) {
	members, entries, ok := splitSettingsClients(settings)
	if !ok || len(entries) == 0 {
		return settings, false
	}
	kept := make([]json.RawMessage, 0, len(entries))
	for _, entry := range entries {
		if c, ok := readSettingsClient(entry); ok && c.Email != "" && isClientEmailTombstoned(c.Email) {
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == len(entries) {
		return settings, false
	}
	return joinSettingsClients(settings, members, kept)
}

// liftClientLifecycleInSettings rewrites adopted settings from master traffic
// so a lagging node blob cannot store pre-extension expiry/enable (#6228).
func liftClientLifecycleInSettings(settings string, trafficByEmail map[string]*xray.ClientTraffic) (string, bool) {
	if len(trafficByEmail) == 0 {
		return settings, false
	}
	members, entries, ok := splitSettingsClients(settings)
	if !ok || len(entries) == 0 {
		return settings, false
	}
	changed := false
	for i, entry := range entries {
		c, ok := readSettingsClient(entry)
		if !ok || c.Email == "" {
			continue
		}
		tr := trafficByEmail[c.Email]
		if tr == nil {
			continue
		}
		nodeExpiry, hasExpiry := jsonClientInt64(c.ExpiryTime)
		if !hasExpiry {
			continue
		}
		merged := mergeActivationExpiry(tr.ExpiryTime, nodeExpiry)
		var nodeEnable bool
		_ = json.Unmarshal(c.Enable, &nodeEnable)
		// tr is the already-merged master row, authoritative in both directions:
		// a lagging blob must not re-enable a disabled client either (#4917).
		if merged == nodeExpiry && nodeEnable == tr.Enable {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(entry, &fields) != nil {
			continue
		}
		fields["expiryTime"] = json.RawMessage(strconv.FormatInt(merged, 10))
		fields["enable"] = json.RawMessage(strconv.FormatBool(tr.Enable))
		rewritten, err := json.Marshal(fields)
		if err != nil {
			continue
		}
		entries[i] = rewritten
		changed = true
	}
	if !changed {
		return settings, false
	}
	return joinSettingsClients(settings, members, entries)
}

// settingsClientAbsoluteExpiries indexes the absolute (>0) expiryTime of every
// client in a settings blob. Never nil, so callers can cache it per inbound.
func settingsClientAbsoluteExpiries(settings string) map[string]int64 {
	out := map[string]int64{}
	_, entries, ok := splitSettingsClients(settings)
	if !ok {
		return out
	}
	for _, entry := range entries {
		c, ok := readSettingsClient(entry)
		if !ok || c.Email == "" {
			continue
		}
		if exp, has := jsonClientInt64(c.ExpiryTime); has && exp > 0 {
			out[c.Email] = exp
		}
	}
	return out
}

// jsonClientInt64 reads a JSON number, as encoding/json would into a float64.
func jsonClientInt64(raw json.RawMessage) (int64, bool) {
	var n float64
	if len(raw) == 0 || json.Unmarshal(raw, &n) != nil {
		return 0, false
	}
	return int64(n), true
}
