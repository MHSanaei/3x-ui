package internet

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"golang.org/x/sys/unix"
)

const (
	// TCP_FASTOPEN_SERVER is the value to enable TCP fast open on darwin for server connections.
	TCP_FASTOPEN_SERVER = 0x01
	// TCP_FASTOPEN_CLIENT is the value to enable TCP fast open on darwin for client connections.
	TCP_FASTOPEN_CLIENT = 0x02 // nolint: revive,stylecheck
	// syscall.TCP_KEEPINTVL is missing on some darwin architectures.
	sysTCP_KEEPINTVL = 0x101 // nolint: revive,stylecheck
)

const (
	PfOut       = 2
	IOCOut      = 0x40000000
	IOCIn       = 0x80000000
	IOCInOut    = IOCIn | IOCOut
	IOCPARMMask = 0x1FFF
	LEN         = 4*16 + 4*4 + 4*1
	// #define	_IOC(inout,group,num,len) (inout | ((len & IOCPARMMask) << 16) | ((group) << 8) | (num))
	// #define	_IOWR(g,n,t)	_IOC(IOCInOut,	(g), (n), sizeof(t))
	// #define DIOCNATLOOK		_IOWR('D', 23, struct pfioc_natlook)
	DIOCNATLOOK = IOCInOut | ((LEN & IOCPARMMask) << 16) | ('D' << 8) | 23
)

// OriginalDst uses ioctl to read original destination from /dev/pf
func OriginalDst(la, ra net.Addr) (net.IP, int, error) {
	f, err := os.Open("/dev/pf")
	if err != nil {
		return net.IP{}, -1, errors.New("failed to open device /dev/pf").Base(err)
	}
	defer f.Close()
	fd := f.Fd()
	nl := struct { // struct pfioc_natlook
		saddr, daddr, rsaddr, rdaddr       [16]byte
		sxport, dxport, rsxport, rdxport   [4]byte
		af, proto, protoVariant, direction uint8
	}{
		af:        syscall.AF_INET,
		proto:     syscall.IPPROTO_TCP,
		direction: PfOut,
	}
	var raIP, laIP net.IP
	var raPort, laPort int
	switch la.(type) {
	case *net.TCPAddr:
		raIP = ra.(*net.TCPAddr).IP
		laIP = la.(*net.TCPAddr).IP
		raPort = ra.(*net.TCPAddr).Port
		laPort = la.(*net.TCPAddr).Port
	case *net.UDPAddr:
		raIP = ra.(*net.UDPAddr).IP
		laIP = la.(*net.UDPAddr).IP
		raPort = ra.(*net.UDPAddr).Port
		laPort = la.(*net.UDPAddr).Port
	}
	if raIP.To4() != nil {
		if laIP.IsUnspecified() {
			laIP = net.ParseIP("127.0.0.1")
		}
		copy(nl.saddr[:net.IPv4len], raIP.To4())
		copy(nl.daddr[:net.IPv4len], laIP.To4())
	}
	if raIP.To16() != nil && raIP.To4() == nil {
		if laIP.IsUnspecified() {
			laIP = net.ParseIP("::1")
		}
		copy(nl.saddr[:], raIP)
		copy(nl.daddr[:], laIP)
	}
	nl.sxport[0], nl.sxport[1] = byte(raPort>>8), byte(raPort)
	nl.dxport[0], nl.dxport[1] = byte(laPort>>8), byte(laPort)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, DIOCNATLOOK, uintptr(unsafe.Pointer(&nl))); errno != 0 {
		return net.IP{}, -1, os.NewSyscallError("ioctl", err)
	}

	odPort := nl.rdxport
	var odIP net.IP
	switch nl.af {
	case syscall.AF_INET:
		odIP = make(net.IP, net.IPv4len)
		copy(odIP, nl.rdaddr[:net.IPv4len])
	case syscall.AF_INET6:
		odIP = make(net.IP, net.IPv6len)
		copy(odIP, nl.rdaddr[:])
	}
	return odIP, int(net.PortFromBytes(odPort[:2])), nil
}

func applyOutboundSocketOptions(network string, address string, fd uintptr, config *SocketConfig) error {
	if isTCPSocket(network) {
		tfo := config.ParseTFOValue()
		if tfo > 0 {
			tfo = TCP_FASTOPEN_CLIENT
		}
		if tfo >= 0 {
			if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, tfo); err != nil {
				return err
			}
		}

		if config.TcpKeepAliveIdle > 0 || config.TcpKeepAliveInterval > 0 {
			if config.TcpKeepAliveIdle > 0 {
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPALIVE, int(config.TcpKeepAliveInterval)); err != nil {
					return errors.New("failed to set TCP_KEEPINTVL", err)
				}
			}
			if config.TcpKeepAliveInterval > 0 {
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, sysTCP_KEEPINTVL, int(config.TcpKeepAliveIdle)); err != nil {
					return errors.New("failed to set TCP_KEEPIDLE", err)
				}
			}
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1); err != nil {
				return errors.New("failed to set SO_KEEPALIVE", err)
			}
		} else if config.TcpKeepAliveInterval < 0 || config.TcpKeepAliveIdle < 0 {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_KEEPALIVE, 0); err != nil {
				return errors.New("failed to unset SO_KEEPALIVE", err)
			}
		}
	}

	if config.Interface != "" {
		iface, err := net.InterfaceByName(config.Interface)

		if err != nil {
			return errors.New("failed to get interface ", config.Interface).Base(err)
		}
		if network == "tcp6" || network == "udp6" {
			if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, iface.Index); err != nil {
				return errors.New("failed to set IPV6_BOUND_IF").Base(err)
			}
		} else {
			if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index); err != nil {
				return errors.New("failed to set IP_BOUND_IF").Base(err)
			}
		}
	}

	if len(config.CustomSockopt) > 0 {
		for _, custom := range config.CustomSockopt {
			if custom.System != "" && custom.System != runtime.GOOS {
				errors.LogDebug(context.Background(), "CustomSockopt system not match: ", "want ", custom.System, " got ", runtime.GOOS)
				continue
			}
			// Skip unwanted network type
			// network might be tcp4 or tcp6
			// use HasPrefix so that "tcp" can match tcp4/6 with "tcp" if user want to control all tcp (udp is also the same)
			// if it is empty, strings.HasPrefix will always return true to make it apply for all networks
			if !strings.HasPrefix(network, custom.Network) {
				continue
			}
			var level = 0x6 // default TCP
			var opt int
			if len(custom.Opt) == 0 {
				return errors.New("No opt!")
			} else {
				opt, _ = strconv.Atoi(custom.Opt)
			}
			if custom.Level != "" {
				level, _ = strconv.Atoi(custom.Level)
			}
			if custom.Type == "int" {
				value, _ := strconv.Atoi(custom.Value)
				if err := syscall.SetsockoptInt(int(fd), level, opt, value); err != nil {
					return errors.New("failed to set CustomSockoptInt", opt, value, err)
				}
			} else if custom.Type == "str" {
				if err := syscall.SetsockoptString(int(fd), level, opt, custom.Value); err != nil {
					return errors.New("failed to set CustomSockoptString", opt, custom.Value, err)
				}
			} else {
				return errors.New("unknown CustomSockopt type:", custom.Type)
			}
		}
	}

	return nil
}

func applyInboundSocketOptions(network string, fd uintptr, config *SocketConfig) error {
	if isTCPSocket(network) {
		tfo := config.ParseTFOValue()
		if tfo > 0 {
			tfo = TCP_FASTOPEN_SERVER
		}
		if tfo >= 0 {
			if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, tfo); err != nil {
				return err
			}
		}

		if config.TcpKeepAliveIdle > 0 || config.TcpKeepAliveInterval > 0 {
			if config.TcpKeepAliveIdle > 0 {
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPALIVE, int(config.TcpKeepAliveInterval)); err != nil {
					return errors.New("failed to set TCP_KEEPINTVL", err)
				}
			}
			if config.TcpKeepAliveInterval > 0 {
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, sysTCP_KEEPINTVL, int(config.TcpKeepAliveIdle)); err != nil {
					return errors.New("failed to set TCP_KEEPIDLE", err)
				}
			}
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1); err != nil {
				return errors.New("failed to set SO_KEEPALIVE", err)
			}
		} else if config.TcpKeepAliveInterval < 0 || config.TcpKeepAliveIdle < 0 {
			if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_KEEPALIVE, 0); err != nil {
				return errors.New("failed to unset SO_KEEPALIVE", err)
			}
		}
	}

	if config.Interface != "" {
		iface, err := net.InterfaceByName(config.Interface)

		if err != nil {
			return errors.New("failed to get interface ", config.Interface).Base(err)
		}
		if network == "tcp6" || network == "udp6" {
			if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, iface.Index); err != nil {
				return errors.New("failed to set IPV6_BOUND_IF").Base(err)
			}
		} else {
			if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index); err != nil {
				return errors.New("failed to set IP_BOUND_IF").Base(err)
			}
		}
	}

	if config.V6Only {
		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, 1); err != nil {
			return errors.New("failed to set IPV6_V6ONLY").Base(err)
		}
	}

	if len(config.CustomSockopt) > 0 {
		for _, custom := range config.CustomSockopt {
			if custom.System != "" && custom.System != runtime.GOOS {
				errors.LogDebug(context.Background(), "CustomSockopt system not match: ", "want ", custom.System, " got ", runtime.GOOS)
				continue
			}
			// Skip unwanted network type
			// network might be tcp4 or tcp6
			// use HasPrefix so that "tcp" can match tcp4/6 with "tcp" if user want to control all tcp (udp is also the same)
			// if it is empty, strings.HasPrefix will always return true to make it apply for all networks
			if !strings.HasPrefix(network, custom.Network) {
				continue
			}
			var level = 0x6 // default TCP
			var opt int
			if len(custom.Opt) == 0 {
				return errors.New("No opt!")
			} else {
				opt, _ = strconv.Atoi(custom.Opt)
			}
			if custom.Level != "" {
				level, _ = strconv.Atoi(custom.Level)
			}
			if custom.Type == "int" {
				value, _ := strconv.Atoi(custom.Value)
				if err := syscall.SetsockoptInt(int(fd), level, opt, value); err != nil {
					return errors.New("failed to set CustomSockoptInt", opt, value, err)
				}
			} else if custom.Type == "str" {
				if err := syscall.SetsockoptString(int(fd), level, opt, custom.Value); err != nil {
					return errors.New("failed to set CustomSockoptString", opt, custom.Value, err)
				}
			} else {
				return errors.New("unknown CustomSockopt type:", custom.Type)
			}
		}
	}

	return nil
}

func bindAddr(fd uintptr, address []byte, port uint32) error {
	setReuseAddr(fd)
	setReusePort(fd)

	var sockaddr unix.Sockaddr

	switch len(address) {
	case net.IPv4len:
		a4 := &unix.SockaddrInet4{
			Port: int(port),
		}
		copy(a4.Addr[:], address)
		sockaddr = a4
	case net.IPv6len:
		a6 := &unix.SockaddrInet6{
			Port: int(port),
		}
		copy(a6.Addr[:], address)
		sockaddr = a6
	default:
		return errors.New("unexpected length of ip")
	}

	return unix.Bind(int(fd), sockaddr)
}

func setReuseAddr(fd uintptr) error {
	if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		return errors.New("failed to set SO_REUSEADDR").Base(err).AtWarning()
	}
	return nil
}

func setReusePort(fd uintptr) error {
	if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		return errors.New("failed to set SO_REUSEPORT").Base(err).AtWarning()
	}
	return nil
}
