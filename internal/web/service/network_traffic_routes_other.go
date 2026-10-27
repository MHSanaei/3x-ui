//go:build !linux

package service

import (
	"fmt"
	"net"
)

func readNetworkDefaultRoutes(_ []net.Interface) ([]networkDefaultRoute, error) {
	return nil, fmt.Errorf("automatic network traffic interface selection is supported on Linux; choose legacy or manual mode")
}
