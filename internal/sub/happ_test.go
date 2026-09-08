package sub

import "testing"

func TestIsHappClient(t *testing.T) {
	for _, tc := range []struct {
		userAgent string
		want      bool
	}{
		{"Happ/1.2.3 (iOS)", true},
		{"HAPP/2.0", true},
		{"Mozilla/5.0 Happ", true},
		{"v2rayN/6.23", false},
		{"", false},
		{"HappyClient/1.0", false}, // "happ" as a word, not a substring of something else
	} {
		if got := IsHappClient(tc.userAgent); got != tc.want {
			t.Errorf("IsHappClient(%q) = %v, want %v", tc.userAgent, got, tc.want)
		}
	}
}
