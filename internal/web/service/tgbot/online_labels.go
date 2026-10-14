package tgbot

import "strings"

// onlineButtonLabel lists every inbound a client is attributed to; with no
// tags it uses fallback (the legacy single-inbound remark, may be empty).
func onlineButtonLabel(email string, tags []string, remarkByTag map[string]string, fallback string) string {
	names := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		name := strings.TrimSpace(remarkByTag[tag])
		if name == "" {
			name = tag
		}
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(names) == 0 {
		if fallback == "" {
			return email
		}
		return email + " - " + fallback
	}
	return email + " - " + strings.Join(names, ", ")
}
