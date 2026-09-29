package service

import (
	"slices"
	"testing"
)

func TestJournalctlArgsBoundTheScanWindow(t *testing.T) {
	want := []string{"-u", "x-ui", "--no-pager", "--since", "30 days ago", "-n", "200", "-p", "err"}
	if got := journalctlArgs(200, "err"); !slices.Equal(got, want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
}
