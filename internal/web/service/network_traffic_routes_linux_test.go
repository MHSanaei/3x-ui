package service

import (
	"net"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestNetworkDefaultRoutesExcludeNonDefaultsAndDeadNextHops(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("192.0.2.0/24")
	_, ipv6Default, _ := net.ParseCIDR("::/0")
	label := 100
	routes := []netlink.Route{
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_MPLS, MPLSDst: &label, LinkIndex: 2},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 1, Priority: 20},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET6, Dst: ipv6Default, LinkIndex: 3, Priority: 10},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET, Dst: subnet, LinkIndex: 2},
		{Table: 100, Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 2},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_BLACKHOLE, Family: unix.AF_INET, LinkIndex: 2},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 2, Flags: unix.RTNH_F_LINKDOWN},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET, Priority: 30, MultiPath: []*netlink.NexthopInfo{{LinkIndex: 2, Flags: unix.RTNH_F_DEAD}, {LinkIndex: 3}}},
	}
	want := []networkDefaultRoute{{"eth0", 4, 20}, {"ens3", 6, 10}, {"ens3", 4, 30}}
	if got := networkRoutesFromNetlink(routes, networkTestInterfaces()); !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults=%v, want %v", got, want)
	}
}

func TestNetworkDefaultRoutePreservesUnsignedPriority(t *testing.T) {
	// The netlink library's int holds a signed priority on 32-bit hosts.
	routes := []netlink.Route{
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 1, Priority: -1},
		{Table: unix.RT_TABLE_MAIN, Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 3, Priority: 10},
	}
	got, err := chooseNetworkDefaultRoute(networkRoutesFromNetlink(routes, networkTestInterfaces()), networkTestInterfaces())
	if err != nil || got != "ens3" {
		t.Fatalf("default interface=%q, %v; want ens3", got, err)
	}
}
