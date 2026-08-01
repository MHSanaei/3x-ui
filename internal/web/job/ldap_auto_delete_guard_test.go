package job

import "testing"

// A bind that succeeds with an unusable search returns (empty, nil); the only
// guard was `err != nil`, so that answer detached the entire inbound.
func TestAutoDeleteAllowed(t *testing.T) {
	cases := []struct {
		name     string
		previous int64
		fetched  int
		want     bool
	}{
		{"empty fetch on a fresh job is refused", 0, 0, false},
		{"empty fetch after a healthy sync is refused", 500, 0, false},
		{"full fetch on a fresh job is allowed", 0, 500, true},
		{"steady fetch is allowed", 500, 500, true},
		{"growth is allowed", 500, 900, true},
		{"shrink above the retention floor is allowed", 500, 250, true},
		{"shrink below the retention floor is refused", 500, 249, false},
		{"collapse to a single user is refused", 500, 1, false},
		{"single user with no history is allowed", 0, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			j := NewLdapSyncJob()
			j.lastFlagCount.Store(c.previous)
			if got := j.autoDeleteSafeForFetch(c.fetched); got != c.want {
				t.Fatalf("autoDeleteSafeForFetch(previous=%d, fetched=%d) = %v, want %v",
					c.previous, c.fetched, got, c.want)
			}
		})
	}
}
