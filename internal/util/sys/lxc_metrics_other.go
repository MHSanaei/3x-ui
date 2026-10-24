//go:build !linux

package sys

import "time"

func (r *LXCReader) Read(now time.Time) (*LXCStats, error) { return nil, nil }
func (r *LXCReader) UseScopedHistory() bool                { return false }
