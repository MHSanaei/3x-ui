//go:build !linux

package mtproto

// killStrayMtgProcesses is a no-op off Linux. On Windows the kill-on-exit job
// object already terminates mtg together with the panel (see
// attachChildLifetime), so orphans do not arise there; other platforms are not
// a supported deployment target for the mtg sidecar.
func killStrayMtgProcesses(_ string) int { return 0 }
