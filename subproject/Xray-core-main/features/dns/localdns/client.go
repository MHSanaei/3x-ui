package localdns

import (
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/dns"
)

// Client is an implementation of dns.Client, which queries localhost for DNS.
type Client struct{}

// Type implements common.HasType.
func (*Client) Type() interface{} {
	return dns.ClientType()
}

// Start implements common.Runnable.
func (*Client) Start() error { return nil }

// Close implements common.Closable.
func (*Client) Close() error { return nil }

// LookupIP implements Client.
func (*Client) LookupIP(host string, option dns.IPOption) ([]net.IP, uint32, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, 0, err
	}
	parsedIPs := make([]net.IP, 0, len(ips))
	ipv4 := make([]net.IP, 0, len(ips))
	ipv6 := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		parsed := net.IPAddress(ip)
		if parsed == nil {
			continue
		}
		parsedIP := parsed.IP()
		parsedIPs = append(parsedIPs, parsedIP)

		if len(parsedIP) == net.IPv4len {
			ipv4 = append(ipv4, parsedIP)
		} else {
			ipv6 = append(ipv6, parsedIP)
		}
	}

	switch {
	case option.IPv4Enable && option.IPv6Enable:
		if len(parsedIPs) > 0 {
			return parsedIPs, dns.DefaultTTL, nil
		}
	case option.IPv4Enable:
		if len(ipv4) > 0 {
			return ipv4, dns.DefaultTTL, nil
		}
	case option.IPv6Enable:
		if len(ipv6) > 0 {
			return ipv6, dns.DefaultTTL, nil
		}
	}
	return nil, 0, dns.ErrEmptyResponse
}

// New create a new dns.Client that queries localhost for DNS.
func New() *Client {
	return &Client{}
}
