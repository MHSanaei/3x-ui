//go:build linux
// +build linux

package tcp

import (
	"context"
	"syscall"
	"unsafe"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/stat"
)

const SO_ORIGINAL_DST = 80

func GetOriginalDestination(conn stat.Connection) (net.Destination, error) {
	sysrawconn, f := conn.(syscall.Conn)
	if !f {
		return net.Destination{}, errors.New("unable to get syscall.Conn")
	}
	rawConn, err := sysrawconn.SyscallConn()
	if err != nil {
		return net.Destination{}, errors.New("failed to get sys fd").Base(err)
	}
	var dest net.Destination
	err = rawConn.Control(func(fd uintptr) {
		level := syscall.IPPROTO_IP
		if conn.RemoteAddr().String()[0] == '[' {
			level = syscall.IPPROTO_IPV6
		}
		addr, err := syscall.GetsockoptIPv6MTUInfo(int(fd), level, SO_ORIGINAL_DST)
		if err != nil {
			errors.LogInfoInner(context.Background(), err, "failed to call getsockopt")
			return
		}
		ip := (*[4]byte)(unsafe.Pointer(&addr.Addr.Flowinfo))[:4]
		if level == syscall.IPPROTO_IPV6 {
			ip = addr.Addr.Addr[:]
		}
		port := (*[2]byte)(unsafe.Pointer(&addr.Addr.Port))[:2]
		dest = net.TCPDestination(net.IPAddress(ip), net.PortFromBytes(port))
	})
	if err != nil {
		return net.Destination{}, errors.New("failed to control connection").Base(err)
	}
	if !dest.IsValid() {
		return net.Destination{}, errors.New("failed to call getsockopt")
	}
	return dest, nil
}
