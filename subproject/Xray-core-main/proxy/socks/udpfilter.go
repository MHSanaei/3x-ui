package socks

import (
	"net"
	"sync"
)

/*
In the sock implementation of * ray, UDP authentication is flawed and can be bypassed.
Tracking a UDP connection may be a bit troublesome.
Here is a simple solution.
We create a filter, add remote IP to the pool when it try to establish a UDP connection with auth.
And drop UDP packets from unauthorized IP.
After discussion, we believe it is not necessary to add a timeout mechanism to this filter.
*/

type UDPFilter struct {
	ips sync.Map
}

func (f *UDPFilter) Add(addr net.Addr) bool {
	ip, _, _ := net.SplitHostPort(addr.String())
	f.ips.Store(ip, true)
	return true
}

func (f *UDPFilter) Check(addr net.Addr) bool {
	ip, _, _ := net.SplitHostPort(addr.String())
	_, ok := f.ips.Load(ip)
	return ok
}
