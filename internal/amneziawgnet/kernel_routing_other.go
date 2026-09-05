//go:build !linux

package amneziawgnet

import (
	"errors"
)

var errNotSupportedOnOS = errors.New("amneziawg kernel module is only supported on linux")

func createKernelLink(ifaceName string, mtu int, addresses []string) error {
	return errNotSupportedOnOS
}

func applyKernelUAPI(ifaceName string, uapiConfig string) error {
	return errNotSupportedOnOS
}

func setupKernelRouting(ifaceName string, inboundID int, addresses []string) error {
	return errNotSupportedOnOS
}

func teardownKernelRouting(ifaceName string, inboundID int) error {
	return nil
}

func readKernelUAPIDump(ifaceName string) (string, error) {
	return "", errNotSupportedOnOS
}
