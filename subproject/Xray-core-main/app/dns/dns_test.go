package dns_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/miekg/dns"
	"github.com/xtls/xray-core/app/dispatcher"
	. "github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/policy"
	"github.com/xtls/xray-core/app/proxyman"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	feature_dns "github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/proxy/freedom"
	"github.com/xtls/xray-core/testing/servers/udp"
)

type staticHandler struct{}

func (*staticHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	ans := new(dns.Msg)
	ans.Id = r.Id

	var clientIP net.IP

	opt := r.IsEdns0()
	if opt != nil {
		for _, o := range opt.Option {
			if o.Option() == dns.EDNS0SUBNET {
				subnet := o.(*dns.EDNS0_SUBNET)
				clientIP = subnet.Address
			}
		}
	}

	for _, q := range r.Question {
		switch {
		case q.Name == "google.com." && q.Qtype == dns.TypeA:
			if clientIP == nil {
				rr, _ := dns.NewRR("google.com. IN A 8.8.8.8")
				ans.Answer = append(ans.Answer, rr)
			} else {
				rr, _ := dns.NewRR("google.com. IN A 8.8.4.4")
				ans.Answer = append(ans.Answer, rr)
			}

		case q.Name == "api.google.com." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("api.google.com. IN A 8.8.7.7")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "v2.api.google.com." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("v2.api.google.com. IN A 8.8.7.8")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "facebook.com." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("facebook.com. IN A 9.9.9.9")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "ipv6.google.com." && q.Qtype == dns.TypeA:
			rr, err := dns.NewRR("ipv6.google.com. IN A 8.8.8.7")
			common.Must(err)
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "ipv6.google.com." && q.Qtype == dns.TypeAAAA:
			rr, err := dns.NewRR("ipv6.google.com. IN AAAA 2001:4860:4860::8888")
			common.Must(err)
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "notexist.google.com." && q.Qtype == dns.TypeAAAA:
			ans.MsgHdr.Rcode = dns.RcodeNameError

		case q.Name == "notexist.google.com." && q.Qtype == dns.TypeA:
			ans.MsgHdr.Rcode = dns.RcodeNameError

		case q.Name == "hostname." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("hostname. IN A 127.0.0.1")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "hostname.local." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("hostname.local. IN A 127.0.0.1")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "hostname.localdomain." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("hostname.localdomain. IN A 127.0.0.1")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "localhost." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("localhost. IN A 127.0.0.2")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "localhost-a." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("localhost-a. IN A 127.0.0.3")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "localhost-b." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("localhost-b. IN A 127.0.0.4")
			ans.Answer = append(ans.Answer, rr)

		case q.Name == "Mijia\\ Cloud." && q.Qtype == dns.TypeA:
			rr, _ := dns.NewRR("Mijia\\ Cloud. IN A 127.0.0.1")
			ans.Answer = append(ans.Answer, rr)
		}
	}
	w.WriteMsg(ans)
}

func TestUDPServerSubnet(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}
	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
					},
				},
				ClientIp: []byte{7, 8, 9, 10},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	ips, _, err := client.LookupIP("google.com", feature_dns.IPOption{
		IPv4Enable: true,
		IPv6Enable: true,
		FakeEnable: false,
	})
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}

	if r := cmp.Diff(ips, []net.IP{{8, 8, 4, 4}}); r != "" {
		t.Fatal(r)
	}
}

func TestUDPServer(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	{
		ips, _, err := client.LookupIP("google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 8}}); r != "" {
			t.Fatal(r)
		}
	}

	{
		ips, _, err := client.LookupIP("facebook.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{9, 9, 9, 9}}); r != "" {
			t.Fatal(r)
		}
	}

	{
		_, _, err := client.LookupIP("notexist.google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err == nil {
			t.Fatal("nil error")
		}
		if r := feature_dns.RCodeFromError(err); r != uint16(dns.RcodeNameError) {
			t.Fatal("expected NameError, but got ", r)
		}
	}

	{
		ips, _, err := client.LookupIP("ipv4only.google.com", feature_dns.IPOption{
			IPv4Enable: false,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if !errors.AllEqual(feature_dns.ErrEmptyResponse, errors.Cause(err)) {
			t.Fatal("error: ", err)
		}
		if len(ips) != 0 {
			t.Fatal("ips: ", ips)
		}
	}

	dnsServer.Shutdown()

	{
		ips, _, err := client.LookupIP("google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 8}}); r != "" {
			t.Fatal(r)
		}
	}
}

func TestPrioritizedDomain(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: 9999, /* unreachable */
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							{
								Type:   DomainMatchingType_Full,
								Domain: "google.com",
							},
						},
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	startTime := time.Now()

	{
		ips, _, err := client.LookupIP("google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 8}}); r != "" {
			t.Fatal(r)
		}
	}

	endTime := time.Now()
	if startTime.After(endTime.Add(time.Second * 2)) {
		t.Error("DNS query doesn't finish in 2 seconds.")
	}
}

func TestUDPServerIPv6(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)
	{
		ips, _, err := client.LookupIP("ipv6.google.com", feature_dns.IPOption{
			IPv4Enable: false,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{32, 1, 72, 96, 72, 96, 0, 0, 0, 0, 0, 0, 0, 0, 136, 136}}); r != "" {
			t.Fatal(r)
		}
	}
}

func TestStaticHostDomain(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
					},
				},
				StaticHosts: []*Config_HostMapping{
					{
						Type:          DomainMatchingType_Full,
						Domain:        "example.com",
						ProxiedDomain: "google.com",
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	{
		ips, _, err := client.LookupIP("example.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 8}}); r != "" {
			t.Fatal(r)
		}
	}

	dnsServer.Shutdown()
}

func TestIPMatch(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					// private dns, not match
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						ExpectedGeoip: []*router.GeoIP{
							{
								CountryCode: "local",
								Cidr: []*router.CIDR{
									{
										// inner ip, will not match
										Ip:     []byte{192, 168, 11, 1},
										Prefix: 32,
									},
								},
							},
						},
					},
					// second dns, match ip
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						ExpectedGeoip: []*router.GeoIP{
							{
								CountryCode: "test",
								Cidr: []*router.CIDR{
									{
										Ip:     []byte{8, 8, 8, 8},
										Prefix: 32,
									},
								},
							},
							{
								CountryCode: "test",
								Cidr: []*router.CIDR{
									{
										Ip:     []byte{8, 8, 8, 4},
										Prefix: 32,
									},
								},
							},
						},
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	startTime := time.Now()

	{
		ips, _, err := client.LookupIP("google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 8}}); r != "" {
			t.Fatal(r)
		}
	}

	endTime := time.Now()
	if startTime.After(endTime.Add(time.Second * 2)) {
		t.Error("DNS query doesn't finish in 2 seconds.")
	}
}

func TestLocalDomain(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: 9999, /* unreachable */
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							// Equivalent of dotless:localhost
							{Type: DomainMatchingType_Regex, Domain: "^[^.]*localhost[^.]*$"},
						},
						ExpectedGeoip: []*router.GeoIP{
							{ // Will match localhost, localhost-a and localhost-b,
								CountryCode: "local",
								Cidr: []*router.CIDR{
									{Ip: []byte{127, 0, 0, 2}, Prefix: 32},
									{Ip: []byte{127, 0, 0, 3}, Prefix: 32},
									{Ip: []byte{127, 0, 0, 4}, Prefix: 32},
								},
							},
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							// Equivalent of dotless: and domain:local
							{Type: DomainMatchingType_Regex, Domain: "^[^.]*$"},
							{Type: DomainMatchingType_Subdomain, Domain: "local"},
							{Type: DomainMatchingType_Subdomain, Domain: "localdomain"},
						},
					},
				},
				StaticHosts: []*Config_HostMapping{
					{
						Type:   DomainMatchingType_Full,
						Domain: "hostnamestatic",
						Ip:     [][]byte{{127, 0, 0, 53}},
					},
					{
						Type:          DomainMatchingType_Full,
						Domain:        "hostnamealias",
						ProxiedDomain: "hostname.localdomain",
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	startTime := time.Now()

	{ // Will match dotless:
		ips, _, err := client.LookupIP("hostname", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 1}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match domain:local
		ips, _, err := client.LookupIP("hostname.local", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 1}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match static ip
		ips, _, err := client.LookupIP("hostnamestatic", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 53}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match domain replacing
		ips, _, err := client.LookupIP("hostnamealias", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 1}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match dotless:localhost, but not expectedIPs: 127.0.0.2, 127.0.0.3, then matches at dotless:
		ips, _, err := client.LookupIP("localhost", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 2}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match dotless:localhost, and expectedIPs: 127.0.0.2, 127.0.0.3
		ips, _, err := client.LookupIP("localhost-a", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 3}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match dotless:localhost, and expectedIPs: 127.0.0.2, 127.0.0.3
		ips, _, err := client.LookupIP("localhost-b", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 4}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match dotless:
		ips, _, err := client.LookupIP("Mijia Cloud", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{127, 0, 0, 1}}); r != "" {
			t.Fatal(r)
		}
	}

	endTime := time.Now()
	if startTime.After(endTime.Add(time.Second * 2)) {
		t.Error("DNS query doesn't finish in 2 seconds.")
	}
}

func TestMultiMatchPrioritizedDomain(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&Config{
				NameServer: []*NameServer{
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: 9999, /* unreachable */
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							{
								Type:   DomainMatchingType_Subdomain,
								Domain: "google.com",
							},
						},
						ExpectedGeoip: []*router.GeoIP{
							{ // Will only match 8.8.8.8 and 8.8.4.4
								Cidr: []*router.CIDR{
									{Ip: []byte{8, 8, 8, 8}, Prefix: 32},
									{Ip: []byte{8, 8, 4, 4}, Prefix: 32},
								},
							},
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							{
								Type:   DomainMatchingType_Subdomain,
								Domain: "google.com",
							},
						},
						ExpectedGeoip: []*router.GeoIP{
							{ // Will match 8.8.8.8 and 8.8.8.7, etc
								Cidr: []*router.CIDR{
									{Ip: []byte{8, 8, 8, 7}, Prefix: 24},
								},
							},
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							{
								Type:   DomainMatchingType_Subdomain,
								Domain: "api.google.com",
							},
						},
						ExpectedGeoip: []*router.GeoIP{
							{ // Will only match 8.8.7.7 (api.google.com)
								Cidr: []*router.CIDR{
									{Ip: []byte{8, 8, 7, 7}, Prefix: 32},
								},
							},
						},
					},
					{
						Address: &net.Endpoint{
							Network: net.Network_UDP,
							Address: &net.IPOrDomain{
								Address: &net.IPOrDomain_Ip{
									Ip: []byte{127, 0, 0, 1},
								},
							},
							Port: uint32(port),
						},
						PrioritizedDomain: []*NameServer_PriorityDomain{
							{
								Type:   DomainMatchingType_Full,
								Domain: "v2.api.google.com",
							},
						},
						ExpectedGeoip: []*router.GeoIP{
							{ // Will only match 8.8.7.8 (v2.api.google.com)
								Cidr: []*router.CIDR{
									{Ip: []byte{8, 8, 7, 8}, Prefix: 32},
								},
							},
						},
					},
				},
			}),
			serial.ToTypedMessage(&dispatcher.Config{}),
			serial.ToTypedMessage(&proxyman.OutboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&freedom.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)

	client := v.GetFeature(feature_dns.ClientType()).(feature_dns.Client)

	startTime := time.Now()

	{ // Will match server 1,2 and server 1 returns expected ip
		ips, _, err := client.LookupIP("google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 8}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match server 1,2 and server 1 returns unexpected ip, then server 2 returns expected one
		ips, _, err := client.LookupIP("ipv6.google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: false,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 8, 7}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match server 3,1,2 and server 3 returns expected one
		ips, _, err := client.LookupIP("api.google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 7, 7}}); r != "" {
			t.Fatal(r)
		}
	}

	{ // Will match server 4,3,1,2 and server 4 returns expected one
		ips, _, err := client.LookupIP("v2.api.google.com", feature_dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		})
		if err != nil {
			t.Fatal("unexpected error: ", err)
		}

		if r := cmp.Diff(ips, []net.IP{{8, 8, 7, 8}}); r != "" {
			t.Fatal(r)
		}
	}

	endTime := time.Now()
	if startTime.After(endTime.Add(time.Second * 2)) {
		t.Error("DNS query doesn't finish in 2 seconds.")
	}
}
