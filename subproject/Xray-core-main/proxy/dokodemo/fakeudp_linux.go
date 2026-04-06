//go:build linux
// +build linux

package dokodemo

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func FakeUDP(addr *net.UDPAddr, mark int) (net.PacketConn, error) {
	var af int
	var sockaddr syscall.Sockaddr

	if len(addr.IP) == 4 {
		af = syscall.AF_INET
		sockaddr = &syscall.SockaddrInet4{Port: addr.Port}
		copy(sockaddr.(*syscall.SockaddrInet4).Addr[:], addr.IP)
	} else {
		af = syscall.AF_INET6
		sockaddr = &syscall.SockaddrInet6{Port: addr.Port}
		copy(sockaddr.(*syscall.SockaddrInet6).Addr[:], addr.IP)
	}

	var fd int
	var err error

	if fd, err = syscall.Socket(af, syscall.SOCK_DGRAM, 0); err != nil {
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("socket open: %s", err)}
	}

	if mark != 0 {
		if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, mark); err != nil {
			syscall.Close(fd)
			return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("set socket option: SO_MARK: %s", err)}
		}
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		syscall.Close(fd)
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("set socket option: IP_TRANSPARENT: %s", err)}
	}

	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)

	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)

	if err = syscall.Bind(fd, sockaddr); err != nil {
		syscall.Close(fd)
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("socket bind: %s", err)}
	}

	fdFile := os.NewFile(uintptr(fd), fmt.Sprintf("net-udp-fake-%s", addr.String()))
	defer fdFile.Close()

	packetConn, err := net.FilePacketConn(fdFile)
	if err != nil {
		syscall.Close(fd)
		return nil, &net.OpError{Op: "fake", Err: fmt.Errorf("convert file descriptor to connection: %s", err)}
	}

	return packetConn, nil
}
