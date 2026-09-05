//go:build linux

package amneziawgnet

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func createKernelLink(ifaceName string, mtu int, addresses []string) error {
	la := netlink.NewLinkAttrs()
	la.Name = ifaceName
	la.MTU = mtu

	link := &netlink.GenericLink{
		LinkAttrs: la,
		LinkType:  "amneziawg",
	}

	if err := netlink.LinkAdd(link); err != nil {
		if !strings.Contains(err.Error(), "exists") {
			return fmt.Errorf("netlink LinkAdd %s (type amneziawg): %w", ifaceName, err)
		}
	}

	curLink, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("netlink LinkByName %s: %w", ifaceName, err)
	}

	for _, addrStr := range addresses {
		addr, err := netlink.ParseAddr(addrStr)
		if err != nil {
			continue
		}
		_ = netlink.AddrAdd(curLink, addr)
	}

	if err := netlink.LinkSetUp(curLink); err != nil {
		return fmt.Errorf("netlink LinkSetUp %s: %w", ifaceName, err)
	}

	return nil
}

func deleteKernelLink(ifaceName string) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil
	}
	return netlink.LinkDel(link)
}

func applyKernelUAPI(ifaceName string, uapiConfig string) error {
	socketPath := fmt.Sprintf("/var/run/wireguard/%s.sock", ifaceName)
	var dialer net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "set=1\n%s\n\n", uapiConfig); err != nil {
		return err
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.HasPrefix(line, "errno=0") {
		return nil
	}
	return fmt.Errorf("kernel uapi error: %s", strings.TrimSpace(line))
}

func setupKernelRouting(ifaceName string, inboundID int, addresses []string) error {
	tableID := 10000 + inboundID
	rule := netlink.NewRule()
	rule.IifName = ifaceName
	rule.Table = tableID

	_ = netlink.RuleDel(rule)
	if err := netlink.RuleAdd(rule); err != nil {
		return fmt.Errorf("netlink RuleAdd for %s: %w", ifaceName, err)
	}

	loLink, err := netlink.LinkByName("lo")
	if err == nil {
		route := &netlink.Route{
			LinkIndex: loLink.Attrs().Index,
			Table:     tableID,
			Scope:     netlink.SCOPE_HOST,
			Type:      unix.RTN_LOCAL,
			Dst:       &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
		}
		_ = netlink.RouteAdd(route)
	}

	return nil
}

func teardownKernelRouting(ifaceName string, inboundID int) error {
	tableID := 10000 + inboundID
	rule := netlink.NewRule()
	rule.IifName = ifaceName
	rule.Table = tableID
	_ = netlink.RuleDel(rule)
	return deleteKernelLink(ifaceName)
}

func readKernelUAPIDump(ifaceName string) (string, error) {
	socketPath := fmt.Sprintf("/var/run/wireguard/%s.sock", ifaceName)
	if _, err := os.Stat(socketPath); err != nil {
		return "", err
	}
	var dialer net.Dialer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "get=1\n\n"); err != nil {
		return "", err
	}
	var sb strings.Builder
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if line == "\n" || line == "errno=0\n" {
			break
		}
		sb.WriteString(line)
	}
	return sb.String(), nil
}
