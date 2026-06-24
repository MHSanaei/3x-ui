package panel

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v2.9.4", "2.9.3", true},
		{"v2.10.0", "2.9.9", true},
		{"v2.9.3", "2.9.3", false},
		{"v2.9.2", "2.9.3", false},
		{"v3.0.0", "2.9.3", true},
	}

	for _, tc := range cases {
		if got := isNewerVersion(tc.latest, tc.current); got != tc.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestCompareVersionStringsRejectsUnexpectedFormats(t *testing.T) {
	if _, ok := compareVersionStrings("latest", "2.9.3"); ok {
		t.Fatal("expected non-semver latest tag to be rejected")
	}
	if _, ok := compareVersionStrings("v2.9", "2.9.3"); ok {
		t.Fatal("expected short version to be rejected")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/usr/bin/curl"); got != "'/usr/bin/curl'" {
		t.Fatalf("unexpected quote result: %s", got)
	}
	if got := shellQuote("/tmp/a'b"); got != "'/tmp/a'\\''b'" {
		t.Fatalf("unexpected quote result with single quote: %s", got)
	}
}

func TestExtractReleaseCommit(t *testing.T) {
	full := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	cases := []struct {
		name    string
		release service.Release
		want    string
	}{
		{
			name:    "from body marker",
			release: service.Release{Body: "Rolling build\n\ncommit=" + full + "\nbuilt=2026-06-24T00:00:00Z"},
			want:    full,
		},
		{
			name:    "body marker is case-insensitive and wins over target",
			release: service.Release{Body: "COMMIT=" + full, TargetCommitish: "deadbeef"},
			want:    full,
		},
		{
			name:    "fallback to target commit sha",
			release: service.Release{Body: "no marker here", TargetCommitish: full},
			want:    full,
		},
		{
			name:    "branch target is not a commit",
			release: service.Release{Body: "no marker", TargetCommitish: "main"},
			want:    "",
		},
	}
	for _, tc := range cases {
		if got := extractReleaseCommit(&tc.release); got != tc.want {
			t.Fatalf("%s: extractReleaseCommit = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCommitsEqual(t *testing.T) {
	full := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	cases := []struct {
		a, b string
		want bool
	}{
		{"1a2b3c4d", full, true},  // injected 8-char prefix matches full release sha
		{full, "1a2b3c4d", true},  // order independent
		{"1A2B3C4D", full, true},  // case insensitive
		{"deadbeef", full, false}, // different commit
		{"", full, false},         // empty current never matches
		{"1a2b3c4d", "", false},   // empty latest never matches
	}
	for _, tc := range cases {
		if got := commitsEqual(tc.a, tc.b); got != tc.want {
			t.Fatalf("commitsEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestShortCommit(t *testing.T) {
	if got := shortCommit("1a2b3c4d5e6f7a8b"); got != "1a2b3c4d" {
		t.Fatalf("shortCommit truncation = %q, want %q", got, "1a2b3c4d")
	}
	if got := shortCommit("abc"); got != "abc" {
		t.Fatalf("shortCommit short input = %q, want %q", got, "abc")
	}
}
