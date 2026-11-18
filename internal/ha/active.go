package ha

import "sync/atomic"

var active atomic.Pointer[Monitor]

// SetActive installs the process's role monitor; nil means HA is off.
func SetActive(m *Monitor) { active.Store(m) }

// Active returns the process's role monitor, or nil when HA is off.
func Active() *Monitor { return active.Load() }
