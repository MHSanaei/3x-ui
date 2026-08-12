package naive

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
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
	switch strings.ToLower(parsed.Scheme) {
	case "https", "quic", "http":
	default:
		return fmt.Errorf("unsupported proxy scheme")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("proxy host is required")
	}
	if rawPort := parsed.Port(); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid proxy port")
		}
	}
	return nil
}

func ValidateVersion(version string) error {
	if !versionPattern.MatchString(strings.TrimSpace(version)) {
		return fmt.Errorf("invalid naive version")
	}
	return nil
}
