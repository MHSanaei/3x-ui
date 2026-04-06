//go:build !linux || android

package wireguard

import (
	"errors"
	"net/netip"
)

func createKernelTun(localAddresses []netip.Addr, mtu int, handler promiscuousModeHandler) (t Tunnel, err error) {
	return nil, errors.New("not implemented")
}

func KernelTunSupported() (bool, error) {
	return false, nil
}
