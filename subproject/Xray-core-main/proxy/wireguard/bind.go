package wireguard

import (
	"context"
	gonet "net"
	"net/netip"
	"runtime"
	"strconv"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/transport/internet"
)

type netReadInfo struct {
	buff     *buf.Buffer
	endpoint conn.Endpoint
}

// reduce duplicated code
type netBind struct {
	dns       dns.Client
	dnsOption dns.IPOption

	workers   int
	readQueue chan *netReadInfo
	closedCh  chan struct{}
}

// SetMark implements conn.Bind
func (bind *netBind) SetMark(mark uint32) error {
	return nil
}

// ParseEndpoint implements conn.Bind
func (n *netBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	ipStr, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil, err
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	addr := net.ParseAddress(ipStr)
	if addr.Family() == net.AddressFamilyDomain {
		ips, _, err := n.dns.LookupIP(addr.Domain(), n.dnsOption)
		if err != nil {
			return nil, err
		} else if len(ips) == 0 {
			return nil, dns.ErrEmptyResponse
		}
		addr = net.IPAddress(ips[0])
	}

	dst := net.Destination{
		Address: addr,
		Port:    net.Port(portNum),
		Network: net.Network_UDP,
	}

	return &netEndpoint{
		dst: dst,
	}, nil
}

// BatchSize implements conn.Bind
func (bind *netBind) BatchSize() int {
	return 1
}

// Open implements conn.Bind
func (bind *netBind) Open(uport uint16) ([]conn.ReceiveFunc, uint16, error) {
	bind.closedCh = make(chan struct{})
	errors.LogDebug(context.Background(), "bind opened")

	fun := func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		select {
		case r := <-bind.readQueue:
			sizes[0], eps[0] = copy(bufs[0], r.buff.Bytes()), r.endpoint
			r.buff.Release()
			return 1, nil
		case <-bind.closedCh:
			errors.LogDebug(context.Background(), "recv func closed")
			return 0, gonet.ErrClosed
		}
	}
	workers := bind.workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers <= 0 {
		workers = 1
	}
	arr := make([]conn.ReceiveFunc, workers)
	for i := 0; i < workers; i++ {
		arr[i] = fun
	}

	return arr, uint16(uport), nil
}

// Close implements conn.Bind
func (bind *netBind) Close() error {
	errors.LogDebug(context.Background(), "bind closed")
	if bind.closedCh != nil {
		close(bind.closedCh)
	}
	return nil
}

type netBindClient struct {
	netBind

	ctx      context.Context
	dialer   internet.Dialer
	reserved []byte
}

func (bind *netBindClient) connectTo(endpoint *netEndpoint) error {
	c, err := bind.dialer.Dial(bind.ctx, endpoint.dst)
	if err != nil {
		return err
	}
	endpoint.conn = c

	go func() {
		for {
			buff := buf.NewWithSize(device.MaxMessageSize)
			n, err := buff.ReadFrom(c)

			if err != nil {
				buff.Release()
				endpoint.conn = nil
				c.Close()
				return
			}

			rawBytes := buff.Bytes()
			if n > 3 {
				rawBytes[1] = 0
				rawBytes[2] = 0
				rawBytes[3] = 0
			}

			select {
			case bind.readQueue <- &netReadInfo{
				buff:     buff,
				endpoint: endpoint,
			}:
			case <-bind.closedCh:
				buff.Release()
				endpoint.conn = nil
				c.Close()
				return
			}
		}
	}()

	return nil
}

func (bind *netBindClient) Send(buff [][]byte, endpoint conn.Endpoint) error {
	var err error

	nend, ok := endpoint.(*netEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}

	if nend.conn == nil {
		err = bind.connectTo(nend)
		if err != nil {
			return err
		}
	}

	for _, buff := range buff {
		if len(buff) > 3 && len(bind.reserved) == 3 {
			copy(buff[1:], bind.reserved)
		}
		if _, err = nend.conn.Write(buff); err != nil {
			return err
		}
	}
	return nil
}

type netBindServer struct {
	netBind
}

func (bind *netBindServer) Send(buff [][]byte, endpoint conn.Endpoint) error {
	var err error

	nend, ok := endpoint.(*netEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}

	if nend.conn == nil {
		errors.LogDebug(context.Background(), nend.dst.NetAddr(), " send on closed peer")
		return errors.New("peer closed")
	}

	for _, buff := range buff {
		if _, err = nend.conn.Write(buff); err != nil {
			return err
		}
	}

	return err
}

type netEndpoint struct {
	dst  net.Destination
	conn net.Conn
}

func (netEndpoint) ClearSrc() {}

func (e netEndpoint) DstIP() netip.Addr {
	return netip.Addr{}
}

func (e netEndpoint) SrcIP() netip.Addr {
	return netip.Addr{}
}

func (e netEndpoint) DstToBytes() []byte {
	var dat []byte
	if e.dst.Address.Family().IsIPv4() {
		dat = e.dst.Address.IP().To4()[:]
	} else {
		dat = e.dst.Address.IP().To16()[:]
	}
	dat = append(dat, byte(e.dst.Port), byte(e.dst.Port>>8))
	return dat
}

func (e netEndpoint) DstToString() string {
	return e.dst.NetAddr()
}

func (e netEndpoint) SrcToString() string {
	return ""
}

func toNetIpAddr(addr net.Address) netip.Addr {
	if addr.Family().IsIPv4() {
		ip := addr.IP()
		return netip.AddrFrom4([4]byte{ip[0], ip[1], ip[2], ip[3]})
	} else {
		ip := addr.IP()
		arr := [16]byte{}
		for i := 0; i < 16; i++ {
			arr[i] = ip[i]
		}
		return netip.AddrFrom16(arr)
	}
}
