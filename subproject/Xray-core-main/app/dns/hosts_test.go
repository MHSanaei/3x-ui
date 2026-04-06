package dns_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/dns"
)

func TestStaticHosts(t *testing.T) {
	pb := []*Config_HostMapping{
		{
			Type:          DomainMatchingType_Subdomain,
			Domain:        "lan",
			ProxiedDomain: "#3",
		},
		{
			Type:   DomainMatchingType_Full,
			Domain: "example.com",
			Ip: [][]byte{
				{1, 1, 1, 1},
			},
		},
		{
			Type:   DomainMatchingType_Full,
			Domain: "proxy.xray.com",
			Ip: [][]byte{
				{1, 2, 3, 4},
				{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			},
			ProxiedDomain: "another-proxy.xray.com",
		},
		{
			Type:          DomainMatchingType_Full,
			Domain:        "proxy2.xray.com",
			ProxiedDomain: "proxy.xray.com",
		},
		{
			Type:   DomainMatchingType_Subdomain,
			Domain: "example.cn",
			Ip: [][]byte{
				{2, 2, 2, 2},
			},
		},
		{
			Type:   DomainMatchingType_Subdomain,
			Domain: "baidu.com",
			Ip: [][]byte{
				{127, 0, 0, 1},
				{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			},
		},
	}

	hosts, err := NewStaticHosts(pb)
	common.Must(err)

	{
		_, err := hosts.Lookup("example.com.lan", dns.IPOption{})
		if dns.RCodeFromError(err) != 3 {
			t.Error(err)
		}
	}

	{
		ips, _ := hosts.Lookup("example.com", dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
		})
		if len(ips) != 1 {
			t.Error("expect 1 IP, but got ", len(ips))
		}
		if diff := cmp.Diff([]byte(ips[0].IP()), []byte{1, 1, 1, 1}); diff != "" {
			t.Error(diff)
		}
	}

	{
		domain, _ := hosts.Lookup("proxy.xray.com", dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: false,
		})
		if len(domain) != 1 {
			t.Error("expect 1 domain, but got ", len(domain))
		}
		if diff := cmp.Diff(domain[0].Domain(), "another-proxy.xray.com"); diff != "" {
			t.Error(diff)
		}
	}

	{
		domain, _ := hosts.Lookup("proxy2.xray.com", dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: false,
		})
		if len(domain) != 1 {
			t.Error("expect 1 domain, but got ", len(domain))
		}
		if diff := cmp.Diff(domain[0].Domain(), "another-proxy.xray.com"); diff != "" {
			t.Error(diff)
		}
	}

	{
		ips, _ := hosts.Lookup("www.example.cn", dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
		})
		if len(ips) != 1 {
			t.Error("expect 1 IP, but got ", len(ips))
		}
		if diff := cmp.Diff([]byte(ips[0].IP()), []byte{2, 2, 2, 2}); diff != "" {
			t.Error(diff)
		}
	}

	{
		ips, _ := hosts.Lookup("baidu.com", dns.IPOption{
			IPv4Enable: false,
			IPv6Enable: true,
		})
		if len(ips) != 1 {
			t.Error("expect 1 IP, but got ", len(ips))
		}
		if diff := cmp.Diff([]byte(ips[0].IP()), []byte(net.LocalHostIPv6.IP())); diff != "" {
			t.Error(diff)
		}
	}
}
func TestStaticHostsFromCache(t *testing.T) {
	sites := []*router.GeoSite{
		{
			CountryCode: "cloudflare-dns.com",
			Domain: []*router.Domain{
				{Type: router.Domain_Full, Value: "example.com"},
			},
		},
		{
			CountryCode: "geosite:cn",
			Domain: []*router.Domain{
				{Type: router.Domain_Domain, Value: "baidu.cn"},
			},
		},
	}
	deps := map[string][]string{
		"HOSTS": {"cloudflare-dns.com", "geosite:cn"},
	}
	hostIPs := map[string][]string{
		"cloudflare-dns.com": {"1.1.1.1"},
		"geosite:cn":         {"2.2.2.2"},
		"_ORDER":             {"cloudflare-dns.com", "geosite:cn"},
	}

	var buf bytes.Buffer
	err := router.SerializeGeoSiteList(sites, deps, hostIPs, &buf)
	common.Must(err)

	// Load matcher
	m, err := router.LoadGeoSiteMatcher(bytes.NewReader(buf.Bytes()), "HOSTS")
	common.Must(err)

	// Load hostIPs
	f := bytes.NewReader(buf.Bytes())
	hips, err := router.LoadGeoSiteHosts(f)
	common.Must(err)

	hosts, err := NewStaticHostsFromCache(m, hips)
	common.Must(err)

	{
		ips, _ := hosts.Lookup("example.com", dns.IPOption{IPv4Enable: true})
		if len(ips) != 1 || ips[0].String() != "1.1.1.1" {
			t.Error("failed to lookup example.com from cache")
		}
	}

	{
		ips, _ := hosts.Lookup("baidu.cn", dns.IPOption{IPv4Enable: true})
		if len(ips) != 1 || ips[0].String() != "2.2.2.2" {
			t.Error("failed to lookup baidu.cn from cache deps")
		}
	}
}
