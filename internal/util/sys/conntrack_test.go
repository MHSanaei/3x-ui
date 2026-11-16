package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConntrackUsage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOST_PROC", root)
	dir := filepath.Join(root, "sys/net/netfilter")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, count, max string
		wantErr          bool
	}{
		{"valid", "6144\n", "7680\n", false},
		{"count exceeds limit", "8000", "7680", false},
		{"invalid count", "invalid", "7680", true},
		{"invalid limit", "1", "invalid", true},
		{"zero limit", "1", "0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for name, value := range map[string]string{"nf_conntrack_count": tc.count, "nf_conntrack_max": tc.max} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			count, max, err := GetConntrackUsage()
			if (err != nil) != tc.wantErr {
				t.Fatalf("GetConntrackUsage() error = %v, want error %v", err, tc.wantErr)
			}
			if tc.name == "valid" && (count != 6144 || max != 7680) {
				t.Fatalf("got %d/%d, want 6144/7680", count, max)
			}
		})
	}
	if err := os.Remove(filepath.Join(dir, "nf_conntrack_count")); err != nil {
		t.Fatal(err)
	}
	count, max, err := GetConntrackUsage()
	if err == nil || count != 0 || max != 0 {
		t.Fatalf("missing files returned %d/%d, %v", count, max, err)
	}
}
