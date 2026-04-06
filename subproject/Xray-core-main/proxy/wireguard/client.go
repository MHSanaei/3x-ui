/*

Some of codes are copied from https://github.com/octeep/wireproxy, license below.

Copyright (c) 2022 Wind T.F. Wong <octeep@pm.me>

Permission to use, copy, modify, and distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

*/

package wireguard

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/dice"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
)

// Handler is an outbound connection that silently swallow the entire payload.
type Handler struct {
	conf          *DeviceConfig
	net           Tunnel
	bind          *netBindClient
	policyManager policy.Manager
	dns           dns.Client
	// cached configuration
	endpoints        []netip.Addr
	hasIPv4, hasIPv6 bool
	wgLock           sync.Mutex
}

// New creates a new wireguard handler.
func New(ctx context.Context, conf *DeviceConfig) (*Handler, error) {
	v := core.MustFromContext(ctx)

	endpoints, hasIPv4, hasIPv6, err := parseEndpoints(conf)
	if err != nil {
		return nil, err
	}

	d := v.GetFeature(dns.ClientType()).(dns.Client)
	return &Handler{
		conf:          conf,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
		dns:           d,
		endpoints:     endpoints,
		hasIPv4:       hasIPv4,
		hasIPv6:       hasIPv6,
	}, nil
}

func (h *Handler) Close() (err error) {
	go func() {
		h.wgLock.Lock()
		defer h.wgLock.Unlock()

		if h.net != nil {
			_ = h.net.Close()
			h.net = nil
		}
	}()

	return nil
}

func (h *Handler) processWireGuard(ctx context.Context, dialer internet.Dialer) (err error) {
	h.wgLock.Lock()
	defer h.wgLock.Unlock()

	if h.bind != nil && h.bind.dialer == dialer && h.net != nil {
		return nil
	}

	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Info,
		Content:  "switching dialer",
	})

	if h.net != nil {
		_ = h.net.Close()
		h.net = nil
	}
	if h.bind != nil {
		_ = h.bind.Close()
		h.bind = nil
	}

	// bind := conn.NewStdNetBind() // TODO: conn.Bind wrapper for dialer
	h.bind = &netBindClient{
		netBind: netBind{
			dns: h.dns,
			dnsOption: dns.IPOption{
				IPv4Enable: h.hasIPv4,
				IPv6Enable: h.hasIPv6,
			},
			workers:   int(h.conf.NumWorkers),
			readQueue: make(chan *netReadInfo),
		},
		ctx:      ctx,
		dialer:   dialer,
		reserved: h.conf.Reserved,
	}
	defer func() {
		if err != nil {
			h.bind.Close()
			h.bind = nil
		}
	}()

	h.net, err = h.makeVirtualTun()
	if err != nil {
		return errors.New("failed to create virtual tun interface").Base(err)
	}
	return nil
}

// Process implements OutboundHandler.Dispatch().
func (h *Handler) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	if !ob.Target.IsValid() {
		return errors.New("target not specified")
	}
	ob.Name = "wireguard"
	ob.CanSpliceCopy = 3

	if err := h.processWireGuard(ctx, dialer); err != nil {
		return err
	}

	// Destination of the inner request.
	destination := ob.Target
	command := protocol.RequestCommandTCP
	if destination.Network == net.Network_UDP {
		command = protocol.RequestCommandUDP
	}

	// resolve dns
	addr := destination.Address
	if addr.Family().IsDomain() {
		ips, _, err := h.dns.LookupIP(addr.Domain(), dns.IPOption{
			IPv4Enable: h.hasIPv4 && h.conf.preferIP4(),
			IPv6Enable: h.hasIPv6 && h.conf.preferIP6(),
		})
		{ // Resolve fallback
			if (len(ips) == 0 || err != nil) && h.conf.hasFallback() {
				ips, _, err = h.dns.LookupIP(addr.Domain(), dns.IPOption{
					IPv4Enable: h.hasIPv4 && h.conf.fallbackIP4(),
					IPv6Enable: h.hasIPv6 && h.conf.fallbackIP6(),
				})
			}
		}
		if err != nil {
			return errors.New("failed to lookup DNS").Base(err)
		} else if len(ips) == 0 {
			return dns.ErrEmptyResponse
		}
		addr = net.IPAddress(ips[dice.Roll(len(ips))])
	}

	var newCtx context.Context
	var newCancel context.CancelFunc
	if session.TimeoutOnlyFromContext(ctx) {
		newCtx, newCancel = context.WithCancel(context.Background())
	}

	p := h.policyManager.ForLevel(0)

	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, func() {
		cancel()
		if newCancel != nil {
			newCancel()
		}
	}, p.Timeouts.ConnectionIdle)
	addrPort := netip.AddrPortFrom(toNetIpAddr(addr), destination.Port.Value())

	var requestFunc func() error
	var responseFunc func() error

	if command == protocol.RequestCommandTCP {
		conn, err := h.net.DialContextTCPAddrPort(ctx, addrPort)
		if err != nil {
			return errors.New("failed to create TCP connection").Base(err)
		}
		defer conn.Close()

		requestFunc = func() error {
			defer timer.SetTimeout(p.Timeouts.DownlinkOnly)
			return buf.Copy(link.Reader, buf.NewWriter(conn), buf.UpdateActivity(timer))
		}
		responseFunc = func() error {
			defer timer.SetTimeout(p.Timeouts.UplinkOnly)
			return buf.Copy(buf.NewReader(conn), link.Writer, buf.UpdateActivity(timer))
		}
	} else if command == protocol.RequestCommandUDP {
		conn, err := h.net.DialUDPAddrPort(netip.AddrPort{}, addrPort)
		if err != nil {
			return errors.New("failed to create UDP connection").Base(err)
		}
		defer conn.Close()

		conn = &udpConnClient{
			Conn: conn,
			dest: destination,
		}

		requestFunc = func() error {
			defer timer.SetTimeout(p.Timeouts.DownlinkOnly)
			return buf.Copy(link.Reader, buf.NewWriter(conn), buf.UpdateActivity(timer))
		}
		responseFunc = func() error {
			defer timer.SetTimeout(p.Timeouts.UplinkOnly)
			return buf.Copy(buf.NewReader(conn), link.Writer, buf.UpdateActivity(timer))
		}
	}

	if newCtx != nil {
		ctx = newCtx
	}

	responseDonePost := task.OnSuccess(responseFunc, task.Close(link.Writer))
	if err := task.Run(ctx, requestFunc, responseDonePost); err != nil {
		common.Interrupt(link.Reader)
		common.Interrupt(link.Writer)
		return errors.New("connection ends").Base(err)
	}

	return nil
}

// creates a tun interface on netstack given a configuration
func (h *Handler) makeVirtualTun() (Tunnel, error) {
	t, err := h.conf.createTun()(h.endpoints, int(h.conf.Mtu), nil)
	if err != nil {
		return nil, err
	}

	h.bind.dnsOption.IPv4Enable = h.hasIPv4
	h.bind.dnsOption.IPv6Enable = h.hasIPv6

	if err = t.BuildDevice(h.createIPCRequest(), h.bind); err != nil {
		_ = t.Close()
		return nil, err
	}
	return t, nil
}

// serialize the config into an IPC request
func (h *Handler) createIPCRequest() string {
	var request strings.Builder

	request.WriteString(fmt.Sprintf("private_key=%s\n", h.conf.SecretKey))

	if !h.conf.IsClient {
		// placeholder, we'll handle actual port listening on Xray
		request.WriteString("listen_port=1337\n")
	}

	for _, peer := range h.conf.Peers {
		if peer.PublicKey != "" {
			request.WriteString(fmt.Sprintf("public_key=%s\n", peer.PublicKey))
		}

		if peer.PreSharedKey != "" {
			request.WriteString(fmt.Sprintf("preshared_key=%s\n", peer.PreSharedKey))
		}

		address, port, err := net.SplitHostPort(peer.Endpoint)
		if err != nil {
			errors.LogError(h.bind.ctx, "failed to split endpoint ", peer.Endpoint, " into address and port")
		}
		addr := net.ParseAddress(address)
		if addr.Family().IsDomain() {
			dialerIp := h.bind.dialer.DestIpAddress()
			if dialerIp != nil {
				addr = net.ParseAddress(dialerIp.String())
				errors.LogInfo(h.bind.ctx, "createIPCRequest use dialer dest ip: ", addr)
			} else {
				ips, _, err := h.dns.LookupIP(addr.Domain(), dns.IPOption{
					IPv4Enable: h.conf.preferIP4(),
					IPv6Enable: h.conf.preferIP6(),
				})
				{ // Resolve fallback
					if (len(ips) == 0 || err != nil) && h.conf.hasFallback() {
						ips, _, err = h.dns.LookupIP(addr.Domain(), dns.IPOption{
							IPv4Enable: h.conf.fallbackIP4(),
							IPv6Enable: h.conf.fallbackIP6(),
						})
					}
				}
				if err != nil {
					errors.LogInfoInner(h.bind.ctx, err, "createIPCRequest failed to lookup DNS")
				} else if len(ips) == 0 {
					errors.LogInfo(h.bind.ctx, "createIPCRequest empty lookup DNS")
				} else {
					addr = net.IPAddress(ips[dice.Roll(len(ips))])
				}
			}
		}

		if peer.Endpoint != "" {
			request.WriteString(fmt.Sprintf("endpoint=%s:%s\n", addr, port))
		}

		for _, ip := range peer.AllowedIps {
			request.WriteString(fmt.Sprintf("allowed_ip=%s\n", ip))
		}

		if peer.KeepAlive != 0 {
			request.WriteString(fmt.Sprintf("persistent_keepalive_interval=%d\n", peer.KeepAlive))
		}
	}

	return request.String()[:request.Len()]
}

type udpConnClient struct {
	net.Conn
	dest net.Destination
}

func (c *udpConnClient) ReadMultiBuffer() (buf.MultiBuffer, error) {
	b := buf.New()
	b.Resize(0, buf.Size)
	n, addr, err := c.Conn.(net.PacketConn).ReadFrom(b.Bytes())
	if err != nil {
		b.Release()
		return nil, err
	}
	if addr == nil { // should never hit
		addr = c.dest.RawNetAddr()
	}
	b.Resize(0, int32(n))

	b.UDP = &net.Destination{
		Address: net.IPAddress(addr.(*net.UDPAddr).IP),
		Port:    net.Port(addr.(*net.UDPAddr).Port),
		Network: net.Network_UDP,
	}

	return buf.MultiBuffer{b}, nil
}

func (c *udpConnClient) WriteMultiBuffer(mb buf.MultiBuffer) error {
	for i, b := range mb {
		dst := c.dest
		if b.UDP != nil {
			dst = *b.UDP
		}
		_, err := c.Conn.(net.PacketConn).WriteTo(b.Bytes(), dst.RawNetAddr())
		if err != nil {
			buf.ReleaseMulti(mb[i:])
			return err
		}
		b.Release()
	}
	return nil
}
