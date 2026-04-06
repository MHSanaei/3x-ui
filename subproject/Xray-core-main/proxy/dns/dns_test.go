package dns_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/miekg/dns"
	"github.com/xtls/xray-core/app/dispatcher"
	dnsapp "github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/policy"
	"github.com/xtls/xray-core/app/proxyman"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	dns_proxy "github.com/xtls/xray-core/proxy/dns"
	"github.com/xtls/xray-core/proxy/dokodemo"
	"github.com/xtls/xray-core/testing/servers/tcp"
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
		}
	}
	w.WriteMsg(ans)
}

func TestUDPDNSTunnel(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
		UDPSize: 1200,
	}
	defer dnsServer.Shutdown()

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	serverPort := udp.PickPort()
	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&dnsapp.Config{
				NameServer: []*dnsapp.NameServer{
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
			serial.ToTypedMessage(&proxyman.InboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Inbound: []*core.InboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  net.NewIPOrDomain(net.LocalHostIP),
					Port:     uint32(port),
					Networks: []net.Network{net.Network_UDP},
				}),
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &net.PortList{Range: []*net.PortRange{net.SinglePortRange(serverPort)}},
					Listen:   net.NewIPOrDomain(net.LocalHostIP),
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&dns_proxy.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)
	common.Must(v.Start())
	defer v.Close()

	{
		m1 := new(dns.Msg)
		m1.Id = dns.Id()
		m1.RecursionDesired = true
		m1.Question = make([]dns.Question, 1)
		m1.Question[0] = dns.Question{Name: "google.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}

		c := new(dns.Client)
		in, _, err := c.Exchange(m1, "127.0.0.1:"+strconv.Itoa(int(serverPort)))
		common.Must(err)

		if len(in.Answer) != 1 {
			t.Fatal("len(answer): ", len(in.Answer))
		}

		rr, ok := in.Answer[0].(*dns.A)
		if !ok {
			t.Fatal("not A record")
		}
		if r := cmp.Diff(rr.A[:], net.IP{8, 8, 8, 8}); r != "" {
			t.Error(r)
		}
	}

	{
		m1 := new(dns.Msg)
		m1.Id = dns.Id()
		m1.RecursionDesired = true
		m1.Question = make([]dns.Question, 1)
		m1.Question[0] = dns.Question{Name: "ipv4only.google.com.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}

		c := new(dns.Client)
		c.Timeout = 10 * time.Second
		in, _, err := c.Exchange(m1, "127.0.0.1:"+strconv.Itoa(int(serverPort)))
		common.Must(err)

		if len(in.Answer) != 0 {
			t.Fatal("len(answer): ", len(in.Answer))
		}
	}

	{
		m1 := new(dns.Msg)
		m1.Id = dns.Id()
		m1.RecursionDesired = true
		m1.Question = make([]dns.Question, 1)
		m1.Question[0] = dns.Question{Name: "notexist.google.com.", Qtype: dns.TypeAAAA, Qclass: dns.ClassINET}

		c := new(dns.Client)
		in, _, err := c.Exchange(m1, "127.0.0.1:"+strconv.Itoa(int(serverPort)))
		common.Must(err)

		if in.Rcode != dns.RcodeNameError {
			t.Error("expected NameError, but got ", in.Rcode)
		}
	}
}

func TestTCPDNSTunnel(t *testing.T) {
	port := udp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "udp",
		Handler: &staticHandler{},
	}
	defer dnsServer.Shutdown()

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	serverPort := tcp.PickPort()
	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&dnsapp.Config{
				NameServer: []*dnsapp.NameServer{
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
			serial.ToTypedMessage(&proxyman.InboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Inbound: []*core.InboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  net.NewIPOrDomain(net.LocalHostIP),
					Port:     uint32(port),
					Networks: []net.Network{net.Network_TCP},
				}),
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &net.PortList{Range: []*net.PortRange{net.SinglePortRange(serverPort)}},
					Listen:   net.NewIPOrDomain(net.LocalHostIP),
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&dns_proxy.Config{}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)
	common.Must(v.Start())
	defer v.Close()

	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.RecursionDesired = true
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{Name: "google.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}

	c := &dns.Client{
		Net: "tcp",
	}
	in, _, err := c.Exchange(m1, "127.0.0.1:"+serverPort.String())
	common.Must(err)

	if len(in.Answer) != 1 {
		t.Fatal("len(answer): ", len(in.Answer))
	}

	rr, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("not A record")
	}
	if r := cmp.Diff(rr.A[:], net.IP{8, 8, 8, 8}); r != "" {
		t.Error(r)
	}
}

func TestUDP2TCPDNSTunnel(t *testing.T) {
	port := tcp.PickPort()

	dnsServer := dns.Server{
		Addr:    "127.0.0.1:" + port.String(),
		Net:     "tcp",
		Handler: &staticHandler{},
	}
	defer dnsServer.Shutdown()

	go dnsServer.ListenAndServe()
	time.Sleep(time.Second)

	serverPort := tcp.PickPort()
	config := &core.Config{
		App: []*serial.TypedMessage{
			serial.ToTypedMessage(&dnsapp.Config{
				NameServer: []*dnsapp.NameServer{
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
			serial.ToTypedMessage(&proxyman.InboundConfig{}),
			serial.ToTypedMessage(&policy.Config{}),
		},
		Inbound: []*core.InboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  net.NewIPOrDomain(net.LocalHostIP),
					Port:     uint32(port),
					Networks: []net.Network{net.Network_TCP},
				}),
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &net.PortList{Range: []*net.PortRange{net.SinglePortRange(serverPort)}},
					Listen:   net.NewIPOrDomain(net.LocalHostIP),
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&dns_proxy.Config{
					Server: &net.Endpoint{
						Network: net.Network_TCP,
					},
				}),
			},
		},
	}

	v, err := core.New(config)
	common.Must(err)
	common.Must(v.Start())
	defer v.Close()

	m1 := new(dns.Msg)
	m1.Id = dns.Id()
	m1.RecursionDesired = true
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{Name: "google.com.", Qtype: dns.TypeA, Qclass: dns.ClassINET}

	c := &dns.Client{
		Net: "tcp",
	}
	in, _, err := c.Exchange(m1, "127.0.0.1:"+serverPort.String())
	common.Must(err)

	if len(in.Answer) != 1 {
		t.Fatal("len(answer): ", len(in.Answer))
	}

	rr, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("not A record")
	}
	if r := cmp.Diff(rr.A[:], net.IP{8, 8, 8, 8}); r != "" {
		t.Error(r)
	}
}
