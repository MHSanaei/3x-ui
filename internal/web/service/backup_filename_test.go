package service

import (
	"regexp"
	"testing"
)

// getDb (controller) only accepts a Content-Disposition filename matching this
// pattern, so every sanitizeBackupHost output must satisfy it.
var backupFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)

func TestSanitizeBackupHost(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"domain", "panel.example.com", "panel.example.com"},
		{"ipv4", "203.0.113.5", "203.0.113.5"},
		{"ipv6", "2001:db8::1", "2001-db8--1"},
		{"ipv6 bracketed", "[fe80::1]", "fe80--1"},
		{"domain with port", "example.com:8443", "example.com-8443"},
		{"trims edge dots and dashes", "-.example.com.-", "example.com"},
		{"empty falls back", "", "x-ui"},
		{"all invalid falls back", ":::", "x-ui"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeBackupHost(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeBackupHost(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !backupFilenameRegex.MatchString(got) {
				t.Errorf("sanitizeBackupHost(%q) = %q, not a valid download filename", tc.in, got)
			}
		})
	}
}
