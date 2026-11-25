package sub

import "regexp"

var shadowrocketUserAgentRegex = regexp.MustCompile(`(?i)\bshadowrocket`)

// IsShadowrocketClient checks if the client user-agent identifies as Shadowrocket.
func IsShadowrocketClient(userAgent string) bool {
	return shadowrocketUserAgentRegex.MatchString(userAgent)
}
