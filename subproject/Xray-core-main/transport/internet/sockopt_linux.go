package internet

import (
	"context"
	"net"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/unix"
)

func bindAddr(fd uintptr, ip []byte, port uint32) error {
	setReuseAddr(fd)
	setReusePort(fd)

	var sockaddr syscall.Sockaddr

	switch len(ip) {
	case net.IPv4len:
		a4 := &syscall.SockaddrInet4{
			Port: int(port),
		}
		copy(a4.Addr[:], ip)
		sockaddr = a4
	case net.IPv6len:
		a6 := &syscall.SockaddrInet6{
			Port: int(port),
		}
		copy(a6.Addr[:], ip)
		sockaddr = a6
	default:
		return errors.New("unexpected length of ip")
	}

	return syscall.Bind(int(fd), sockaddr)
}

// applyOutboundSocketOptions applies socket options for outbound connection.
// note that unlike other part of Xray, this function needs network with speified network stack(tcp4/tcp6/udp4/udp6)
func applyOutboundSocketOptions(network string, address string, fd uintptr, config *SocketConfig) error {
	if config.Mark != 0 {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, int(config.Mark)); err != nil {
			return errors.New("failed to set SO_MARK").Base(err)
		}
	}

	if config.Interface != "" {
		if err := syscall.BindToDevice(int(fd), config.Interface); err != nil {
			return errors.New("failed to set Interface").Base(err)
		}
	}

	if isTCPSocket(network) {
		tfo := config.ParseTFOValue()
		if tfo > 0 {
			tfo = 1
		}
		if tfo >= 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.SOL_TCP, unix.TCP_FASTOPEN_CONNECT, tfo); err != nil {
				return errors.New("failed to set TCP_FASTOPEN_CONNECT", tfo).Base(err)
			}
		}

		if config.TcpCongestion != "" {
			if err := syscall.SetsockoptString(int(fd), syscall.SOL_TCP, syscall.TCP_CONGESTION, config.TcpCongestion); err != nil {
				return errors.New("failed to set TCP_CONGESTION", err)
			}
		}

		if config.TcpWindowClamp > 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_WINDOW_CLAMP, int(config.TcpWindowClamp)); err != nil {
				return errors.New("failed to set TCP_WINDOW_CLAMP", err)
			}
		}

		if config.TcpUserTimeout > 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(config.TcpUserTimeout)); err != nil {
				return errors.New("failed to set TCP_USER_TIMEOUT", err)
			}
		}

		if config.TcpMaxSeg > 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, unix.TCP_MAXSEG, int(config.TcpMaxSeg)); err != nil {
				return errors.New("failed to set TCP_MAXSEG", err)
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

	if config.Tproxy.IsEnabled() {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
			return errors.New("failed to set IP_TRANSPARENT").Base(err)
		}
	}

	return nil
}

// applyInboundSocketOptions applies socket options for inbound listener.
// note that unlike other part of Xray, this function needs network with speified network stack(tcp4/tcp6/udp4/udp6)
func applyInboundSocketOptions(network string, fd uintptr, config *SocketConfig) error {
	if config.Mark != 0 {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, int(config.Mark)); err != nil {
			return errors.New("failed to set SO_MARK").Base(err)
		}
	}
	if isTCPSocket(network) {
		tfo := config.ParseTFOValue()
		if tfo >= 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.SOL_TCP, unix.TCP_FASTOPEN, tfo); err != nil {
				return errors.New("failed to set TCP_FASTOPEN", tfo).Base(err)
			}
		}

		if config.TcpKeepAliveInterval > 0 || config.TcpKeepAliveIdle > 0 {
			if config.TcpKeepAliveInterval > 0 {
				if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_KEEPINTVL, int(config.TcpKeepAliveInterval)); err != nil {
					return errors.New("failed to set TCP_KEEPINTVL", err)
				}
			}
			if config.TcpKeepAliveIdle > 0 {
				if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_KEEPIDLE, int(config.TcpKeepAliveIdle)); err != nil {
					return errors.New("failed to set TCP_KEEPIDLE", err)
				}
			}
			if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 1); err != nil {
				return errors.New("failed to set SO_KEEPALIVE", err)
			}
		} else if config.TcpKeepAliveInterval < 0 || config.TcpKeepAliveIdle < 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE, 0); err != nil {
				return errors.New("failed to unset SO_KEEPALIVE", err)
			}
		}

		if config.TcpCongestion != "" {
			if err := syscall.SetsockoptString(int(fd), syscall.SOL_TCP, syscall.TCP_CONGESTION, config.TcpCongestion); err != nil {
				return errors.New("failed to set TCP_CONGESTION", err)
			}
		}

		if config.TcpWindowClamp > 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_WINDOW_CLAMP, int(config.TcpWindowClamp)); err != nil {
				return errors.New("failed to set TCP_WINDOW_CLAMP", err)
			}
		}

		if config.TcpUserTimeout > 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(config.TcpUserTimeout)); err != nil {
				return errors.New("failed to set TCP_USER_TIMEOUT", err)
			}
		}

		if config.TcpMaxSeg > 0 {
			if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, unix.TCP_MAXSEG, int(config.TcpMaxSeg)); err != nil {
				return errors.New("failed to set TCP_MAXSEG", err)
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
	}

	if config.Tproxy.IsEnabled() {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
			return errors.New("failed to set IP_TRANSPARENT").Base(err)
		}
	}

	if config.ReceiveOriginalDestAddress && isUDPSocket(network) {
		err1 := syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_RECVORIGDSTADDR, 1)
		err2 := syscall.SetsockoptInt(int(fd), syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
		if err1 != nil && err2 != nil {
			return err1
		}
	}

	if config.V6Only {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, syscall.IPV6_V6ONLY, 1); err != nil {
			return errors.New("failed to set IPV6_V6ONLY", err)
		}
	}

	return nil
}

func setReuseAddr(fd uintptr) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return errors.New("failed to set SO_REUSEADDR").Base(err).AtWarning()
	}
	return nil
}

func setReusePort(fd uintptr) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
		return errors.New("failed to set SO_REUSEPORT").Base(err).AtWarning()
	}
	return nil
}
