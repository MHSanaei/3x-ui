package naive

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	tagPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	versionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+\.\d+-\d+$`)
)

func ValidateTag(tag string) error {
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("invalid naive tag")
	}
	return nil
}

func ValidateProxyURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("proxy URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	switch parsed.Scheme {
	case "https", "quic", "http":
	default:
		return fmt.Errorf("unsupported proxy scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("proxy host is required")
	}
	return nil
}

func ValidateVersion(version string) error {
	if !versionPattern.MatchString(strings.TrimSpace(version)) {
		return fmt.Errorf("invalid naive version")
	}
	return nil
}
