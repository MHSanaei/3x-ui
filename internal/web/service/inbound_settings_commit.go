package service

import (
	"encoding/json"
	"reflect"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// commitInboundClientSettings writes a client op's edit (base → ib.Settings) onto
// the settings committed now: a traffic tick after the op's read must survive.
func commitInboundClientSettings(tx *gorm.DB, ib *model.Inbound, base string) error {
	var current []string
	if err := tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).Pluck("settings", &current).Error; err != nil {
		return err
	}
	if len(current) == 1 && current[0] != base {
		merged, err := rebaseClientSettings(base, ib.Settings, current[0])
		if err != nil {
			return err
		}
		ib.Settings = merged
	}
	return tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", ib.Settings).Error
}

// rebaseClientSettings three-way merges settings JSON: every key and client field
// ours left as base had it takes current's value; clients are matched by email.
func rebaseClientSettings(base, ours, current string) (string, error) {
	var baseM, oursM, curM map[string]any
	for _, p := range []struct {
		raw string
		dst *map[string]any
	}{{base, &baseM}, {ours, &oursM}, {current, &curM}} {
		if err := json.Unmarshal([]byte(p.raw), p.dst); err != nil {
			return "", err
		}
	}
	out := mergeFields(baseM, oursM, curM)
	baseClients, _ := baseM["clients"].([]any)
	oursClients, _ := oursM["clients"].([]any)
	curClients, _ := curM["clients"].([]any)
	if _, has := oursM["clients"]; has {
		out["clients"] = mergeClientLists(baseClients, oursClients, curClients)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func mergeFields(base, ours, current map[string]any) map[string]any {
	out := make(map[string]any, len(current)+len(ours))
	for k, v := range current {
		out[k] = v
	}
	keys := make(map[string]struct{}, len(base)+len(ours))
	for k := range base {
		keys[k] = struct{}{}
	}
	for k := range ours {
		keys[k] = struct{}{}
	}
	for k := range keys {
		bv, inBase := base[k]
		ov, inOurs := ours[k]
		if inBase == inOurs && reflect.DeepEqual(bv, ov) {
			continue
		}
		if inOurs {
			out[k] = ov
		} else {
			delete(out, k)
		}
	}
	return out
}

func clientEntryEmail(entry any) (map[string]any, string) {
	m, ok := entry.(map[string]any)
	if !ok {
		return nil, ""
	}
	email, _ := m["email"].(string)
	return m, email
}

func indexClientsByEmail(list []any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(list))
	for _, entry := range list {
		if m, email := clientEntryEmail(entry); email != "" {
			out[email] = m
		}
	}
	return out
}

// mergeClientLists keeps ours' order. A client ours removed stays removed; one a
// concurrent writer added or removed keeps that change unless ours edited it.
func mergeClientLists(base, ours, current []any) []any {
	baseBy := indexClientsByEmail(base)
	curBy := indexClientsByEmail(current)
	out := make([]any, 0, len(ours)+len(current))
	placed := make(map[string]struct{}, len(ours))
	for _, entry := range ours {
		o, email := clientEntryEmail(entry)
		if email == "" {
			out = append(out, entry)
			continue
		}
		placed[email] = struct{}{}
		b, inBase := baseBy[email]
		c, inCur := curBy[email]
		switch {
		case !inBase:
			out = append(out, o)
		case !inCur:
			if !reflect.DeepEqual(b, o) {
				out = append(out, o)
			}
		default:
			out = append(out, mergeFields(b, o, c))
		}
	}
	for _, entry := range current {
		c, email := clientEntryEmail(entry)
		if email == "" {
			continue
		}
		if _, done := placed[email]; done {
			continue
		}
		if _, inBase := baseBy[email]; !inBase {
			out = append(out, c)
		}
	}
	return out
}

// keepStoredClients puts the stored client list back into an inbound save's
// payload: clients change through the client endpoints, never this form.
func keepStoredClients(payload, stored string) string {
	var payloadM, storedM map[string]any
	if json.Unmarshal([]byte(payload), &payloadM) != nil || json.Unmarshal([]byte(stored), &storedM) != nil {
		return payload
	}
	storedClients, has := storedM["clients"]
	if reflect.DeepEqual(payloadM["clients"], storedClients) {
		return payload
	}
	if has {
		payloadM["clients"] = storedClients
	} else {
		delete(payloadM, "clients")
	}
	b, err := json.MarshalIndent(payloadM, "", "  ")
	if err != nil {
		return payload
	}
	return string(b)
}
