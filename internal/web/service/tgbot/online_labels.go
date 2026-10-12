package tgbot

import "strings"

// onlineButtonLabel builds the label of one online-client button. tags are the
// inbound tags the client is currently attributed to (from the access log); a
// client connected to several inbounds lists every one of them, so the bot no
// longer reports only the first inbound the client happens to be attached to.
// remarkByTag maps a tag to its inbound remark; a tag without a remark is shown
// as-is. With no attribution, fallback (the legacy single-inbound remark, may be
// empty) is used.
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
