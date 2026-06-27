package service

import (
	"regexp"
	"testing"
	"time"
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

// dateSuffixRegex narrows backupFilenameRegex to the exact _YYYY-MM-DD_HHMMSS shape.
var dateSuffixRegex = regexp.MustCompile(`^_\d{4}-\d{2}-\d{2}_\d{6}$`)

func TestBackupDateSuffix(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"utc midnight", time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC), "_2026-06-27_000000"},
		{"end of year", time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC), "_2025-12-31_235959"},
		{"single digit month/day padded", time.Date(2026, 1, 5, 9, 4, 0, 0, time.UTC), "_2026-01-05_090400"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := backupDateSuffix(tc.now)
			if got != tc.want {
				t.Errorf("backupDateSuffix(%v) = %q, want %q", tc.now, got, tc.want)
			}
			if !dateSuffixRegex.MatchString(got) {
				t.Errorf("backupDateSuffix(%v) = %q, not a valid date suffix", tc.now, got)
			}
			if !backupFilenameRegex.MatchString(got) {
				t.Errorf("backupDateSuffix(%v) = %q, not a valid download filename char", tc.now, got)
			}
		})
	}
}
