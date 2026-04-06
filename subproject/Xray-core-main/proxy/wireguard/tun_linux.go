//go:build linux && !android

package wireguard

import (
	"context"
	goerrors "errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/vishvananda/netlink"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/transport/internet"
	"golang.zx2c4.com/wireguard/tun"
)

type deviceNet struct {
	tunnel
	dialer *net.Dialer
	lc     *net.ListenConfig

	handle    *netlink.Handle
	linkAddrs []netlink.Addr
	routes    []*netlink.Route
	rules     []*netlink.Rule
}

var (
	tableIndex int = 10230
	mu         sync.Mutex
)

func allocateIPv6TableIndex() int {
	mu.Lock()
	defer mu.Unlock()

	if tableIndex > 10230 {
		errors.LogInfo(context.Background(), "allocate new ipv6 table index: ", tableIndex)
	}
	currentIndex := tableIndex
	tableIndex++
	return currentIndex
}

func newDeviceNet(interfaceName string) *deviceNet {
	dialer := &net.Dialer{}
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := syscall.BindToDevice(int(fd), interfaceName); err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to bind to device")
			}
		})
	}
	lc := &net.ListenConfig{}
	lc.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if err := syscall.BindToDevice(int(fd), interfaceName); err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to bind to device")
			}
		})
	}
	return &deviceNet{dialer: dialer, lc: lc}
}

func (d *deviceNet) DialContextTCPAddrPort(ctx context.Context, addr netip.AddrPort) (
	net.Conn, error,
) {
	return d.dialer.DialContext(ctx, "tcp", addr.String())
}

func (d *deviceNet) DialUDPAddrPort(laddr, raddr netip.AddrPort) (net.Conn, error) {
	var conn net.PacketConn
	var err error
	if raddr.Addr().Is4() {
		conn, err = d.lc.ListenPacket(context.Background(), "udp4", ":0")
	} else {
		conn, err = d.lc.ListenPacket(context.Background(), "udp6", ":0")
	}
	if err != nil {
		return nil, err
	}
	return &internet.PacketConnWrapper{
		PacketConn: conn,
		Dest: &net.UDPAddr{
			IP:   raddr.Addr().AsSlice(),
			Port: int(raddr.Port()),
		},
	}, nil
}

func (d *deviceNet) Close() (err error) {
	var errs []error
	for _, rule := range d.rules {
		if err = d.handle.RuleDel(rule); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete rule: %w", err))
		}
	}
	for _, route := range d.routes {
		if err = d.handle.RouteDel(route); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete route: %w", err))
		}
	}
	if err = d.tunnel.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close tunnel: %w", err))
	}
	if d.handle != nil {
		d.handle.Close()
		d.handle = nil
	}
	if len(errs) == 0 {
		return nil
	}
	return goerrors.Join(errs...)
}

func createKernelTun(localAddresses []netip.Addr, mtu int, handler promiscuousModeHandler) (t Tunnel, err error) {
	if handler != nil {
		return nil, errors.New("TODO: support promiscuous mode")
	}

	var v4, v6 *netip.Addr
	for _, prefixes := range localAddresses {
		if v4 == nil && prefixes.Is4() {
			x := prefixes
			v4 = &x
		}
		if v6 == nil && prefixes.Is6() {
			x := prefixes
			v6 = &x
		}
	}

	writeSysctlZero := func(path string) error {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte("0"), 0o644)
	}

	// system configs.
	if v4 != nil {
		if err = writeSysctlZero("/proc/sys/net/ipv4/conf/all/rp_filter"); err != nil {
			return nil, fmt.Errorf("failed to disable ipv4 rp_filter for all: %w", err)
		}
	}
	if v6 != nil {
		if err = writeSysctlZero("/proc/sys/net/ipv6/conf/all/disable_ipv6"); err != nil {
			return nil, fmt.Errorf("failed to enable ipv6: %w", err)
		}
		if err = writeSysctlZero("/proc/sys/net/ipv6/conf/all/rp_filter"); err != nil {
			return nil, fmt.Errorf("failed to disable ipv6 rp_filter for all: %w", err)
		}
	}

	n := CalculateInterfaceName("wg")
	wgt, err := tun.CreateTUN(n, mtu)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = wgt.Close()
		}
	}()

	// disable linux rp_filter for tunnel device to avoid packet drop.
	// the operation require root privilege on container require '--privileged' flag.
	if v4 != nil {
		if err = writeSysctlZero("/proc/sys/net/ipv4/conf/" + n + "/rp_filter"); err != nil {
			return nil, fmt.Errorf("failed to disable ipv4 rp_filter for tunnel: %w", err)
		}
	}
	if v6 != nil {
		if err = writeSysctlZero("/proc/sys/net/ipv6/conf/" + n + "/rp_filter"); err != nil {
			return nil, fmt.Errorf("failed to disable ipv6 rp_filter for tunnel: %w", err)
		}
	}

	ipv6TableIndex := allocateIPv6TableIndex()
	if v6 != nil {
		r := &netlink.Route{Table: ipv6TableIndex}
		for {
			routeList, fErr := netlink.RouteListFiltered(netlink.FAMILY_V6, r, netlink.RT_FILTER_TABLE)
			if len(routeList) == 0 || fErr != nil {
				break
			}
			ipv6TableIndex--
			if ipv6TableIndex < 0 {
				return nil, fmt.Errorf("failed to find available ipv6 table index")
			}
		}
	}

	out := newDeviceNet(n)
	out.handle, err = netlink.NewHandle()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = out.Close()
		}
	}()

	l, err := netlink.LinkByName(n)
	if err != nil {
		return nil, err
	}

	if v4 != nil {
		addr := netlink.Addr{
			IPNet: &net.IPNet{
				IP:   v4.AsSlice(),
				Mask: net.CIDRMask(v4.BitLen(), v4.BitLen()),
			},
		}
		out.linkAddrs = append(out.linkAddrs, addr)
	}
	if v6 != nil {
		addr := netlink.Addr{
			IPNet: &net.IPNet{
				IP:   v6.AsSlice(),
				Mask: net.CIDRMask(v6.BitLen(), v6.BitLen()),
			},
		}
		out.linkAddrs = append(out.linkAddrs, addr)

		rt := &netlink.Route{
			LinkIndex: l.Attrs().Index,
			Dst: &net.IPNet{
				IP:   net.IPv6zero,
				Mask: net.CIDRMask(0, 128),
			},
			Table: ipv6TableIndex,
		}
		out.routes = append(out.routes, rt)

		r := netlink.NewRule()
		r.Table, r.Family, r.Src = ipv6TableIndex, unix.AF_INET6, addr.IPNet
		out.rules = append(out.rules, r)
		r = netlink.NewRule()
		r.Table, r.Family, r.OifName = ipv6TableIndex, unix.AF_INET6, n
		out.rules = append(out.rules, r)
	}

	for _, addr := range out.linkAddrs {
		if err = out.handle.AddrAdd(l, &addr); err != nil {
			return nil, fmt.Errorf("failed to add address %s to %s: %w", addr, n, err)
		}
	}
	if err = out.handle.LinkSetMTU(l, mtu); err != nil {
		return nil, err
	}
	if err = out.handle.LinkSetUp(l); err != nil {
		return nil, err
	}

	for _, route := range out.routes {
		if err = out.handle.RouteAdd(route); err != nil {
			return nil, fmt.Errorf("failed to add route %s: %w", route, err)
		}
	}
	for _, rule := range out.rules {
		if err = out.handle.RuleAdd(rule); err != nil {
			return nil, fmt.Errorf("failed to add rule %s: %w", rule, err)
		}
	}
	out.tun = wgt
	return out, nil
}

func KernelTunSupported() (bool, error) {
	var hdr unix.CapUserHeader
	hdr.Version = unix.LINUX_CAPABILITY_VERSION_3
	hdr.Pid = 0 // 0 means current process

	var data unix.CapUserData
	if err := unix.Capget(&hdr, &data); err != nil {
		return false, fmt.Errorf("failed to get capabilities: %v", err)
	}

	return (data.Effective & (1 << unix.CAP_NET_ADMIN)) != 0, nil
}
