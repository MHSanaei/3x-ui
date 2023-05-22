//go:build windows
// +build windows

package sys

import (
	"errors"

	"github.com/shirou/gopsutil/v3/net"
)

func GetConnectionCount(proto string) (int, error) {
	if proto != "tcp" && proto != "udp" {
		return 0, errors.New("invalid protocol")
	}

	stats, err := net.Connections(proto)
	if err != nil {
		return 0, err
	}
	return len(stats), nil
}

func GetTCPCount() (int, error) {
	return GetConnectionCount("tcp")
}

func GetUDPCount() (int, error) {
	return GetConnectionCount("udp")
}
