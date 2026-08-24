package common

import (
	"errors"
	"net/url"
	"strings"
)

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

// ParseRemoteRoutingURL classifies a routing settings value: one single-line
// absolute HTTPS URL is a remote source (canonicalized); anything else is inline.
func ParseRemoteRoutingURL(raw string) (string, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.ContainsAny(trimmed, "\r\n") {
		return "", false, nil
	}
	if !strings.HasPrefix(strings.ToLower(trimmed), "https://") {
		return "", false, nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return "", true, errors.New("must be an absolute HTTPS URL")
	}
	if u.User != nil {
		return "", true, errors.New("must not contain URL credentials")
	}
	u.Scheme = "https"
	u.Fragment = ""
	return u.String(), true, nil
}
