package common

import (
	"strconv"
	"unicode/utf8"
)

// FormatTraffic formats traffic bytes into human-readable units (B, KB, MB, GB, TB, PB).
func FormatTraffic(trafficBytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	unitIndex := 0
	size := float64(trafficBytes)

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	return strconv.FormatFloat(size, 'f', 2, 64) + units[unitIndex]
}

// maxStatusTextBytes bounds an error text a third party shapes before it is stored as a
// row's status; legitimate messages are a line or two.
const maxStatusTextBytes = 1024

// TruncateStatusText cuts s to maxStatusTextBytes on a UTF-8 boundary.
func TruncateStatusText(s string) string {
	if len(s) <= maxStatusTextBytes {
		return s
	}
	cut := maxStatusTextBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "... (truncated)"
}
