package common

import "strings"

// EnsureURLScheme prepends https:// to a URL that carries no scheme, so
// subscription apps and browsers don't resolve it relative to the panel's own
// domain (e.g. "t.me/support" turning into "https://panel.example/t.me/support").
// Values with an explicit scheme (https://, tg://, mailto:, tel:) and empty
// strings pass through untouched.
func EnsureURLScheme(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") ||
		strings.HasPrefix(trimmed, "mailto:") ||
		strings.HasPrefix(trimmed, "tel:") {
		return trimmed
	}
	return "https://" + trimmed
}
