//go:build !linux

package xray

// KernelTunBlocked is always false off Linux: the core never puts WireGuard on a kernel TUN there.
func KernelTunBlocked() bool { return false }
