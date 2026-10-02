// Package version compares 3x-ui release versions ("v3.8.0" or "3.8.0").
package version

import (
	"strconv"
	"strings"
)

// Compare returns -1, 0 or 1 as a is older than, equal to or newer than b, and
// false when either side is not a plain MAJOR.MINOR.PATCH version.
func Compare(a, b string) (int, bool) {
	aParts, okA := parse(a)
	bParts, okB := parse(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range len(aParts) {
		if aParts[i] > bParts[i] {
			return 1, true
		}
		if aParts[i] < bParts[i] {
			return -1, true
		}
	}
	return 0, true
}

// Normalize strips surrounding space and a leading "v" from a version tag.
func Normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func parse(v string) ([3]int, bool) {
	var result [3]int
	parts := strings.Split(Normalize(v), ".")
	if len(parts) != 3 {
		return result, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return result, false
		}
		result[i] = n
	}
	return result, true
}
