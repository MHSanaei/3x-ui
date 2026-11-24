package service

import (
	stdnet "net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func readNetworkDefaultRoutes(interfaces []stdnet.Interface) ([]networkDefaultRoute, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: unix.RT_TABLE_MAIN}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, err
	}
	return networkRoutesFromNetlink(routes, interfaces), nil
}

func networkRoutesFromNetlink(routes []netlink.Route, interfaces []stdnet.Interface) []networkDefaultRoute {
	names := make(map[int]string, len(interfaces))
	for _, iface := range interfaces {
		names[iface.Index] = iface.Name
	}
	var defaults []networkDefaultRoute
	for _, route := range routes {
		if route.Family != unix.AF_INET && route.Family != unix.AF_INET6 {
			continue
		}
		if route.Table != unix.RT_TABLE_MAIN || route.Type != unix.RTN_UNICAST || route.Flags&unix.RTNH_F_LINKDOWN != 0 {
			continue
		}
		if route.Dst != nil {
			ones, bits := route.Dst.Mask.Size()
			if ones != 0 || bits == 0 || !route.Dst.IP.IsUnspecified() {
				continue
			}
		}
		family := 4
		if route.Family == unix.AF_INET6 {
			family = 6
		}
		add := func(index, flags int) {
			if flags&(unix.RTNH_F_DEAD|unix.RTNH_F_LINKDOWN) == 0 && names[index] != "" {
				defaults = append(defaults, networkDefaultRoute{names[index], family, uint32(route.Priority)})
			}
		}
		if len(route.MultiPath) == 0 {
			add(route.LinkIndex, route.Flags)
		} else {
			for _, hop := range route.MultiPath {
				if hop != nil {
					add(hop.LinkIndex, hop.Flags)
				}
			}
		}
	}
	return defaults
}
