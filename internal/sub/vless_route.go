package sub

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// xray reads the route from UUID bytes 6-7 (net.PortFromBytes) and masks them to
// zero before auth, so baking a 0-65535 value into the 3rd group routes without
// breaking the user match. Empty/invalid/non-UUID input is returned unchanged.
func applyVlessRoute(id, route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return id
	}
	n, err := strconv.Atoi(route)
	if err != nil || n < 0 || n > 65535 {
		return id
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return id
	}
	u[6] = byte(n >> 8)
	u[7] = byte(n)
	return u.String()
}

func hostVlessRoute(ep map[string]any) string {
	v, _ := ep["vlessRoute"].(string)
	return v
}
