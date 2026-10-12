package model

import (
	"strconv"
	"strings"
)

var legacyClientIntFields = []string{"tgId", "limitIp", "totalGB", "expiryTime", "reset", "created_at", "updated_at"}

// NormalizeLegacyClientFields turns the string numbers older panels stored on a
// client object into integers; an empty or unparseable value is dropped.
func NormalizeLegacyClientFields(obj map[string]any) (changed bool) {
	for _, key := range legacyClientIntFields {
		s, ok := obj[key].(string)
		if !ok {
			continue
		}
		changed = true
		trimmed := strings.ReplaceAll(strings.TrimSpace(s), " ", "")
		if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			obj[key] = n
		} else {
			delete(obj, key)
		}
	}
	return changed
}
