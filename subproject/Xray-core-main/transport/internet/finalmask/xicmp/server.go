package xicmp

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/transport/internet/finalmask"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	idleTimeout      = 10 * time.Second
	maxResponseDelay = 1 * time.Second
)

type record struct {
	id          int
	seq         int
	needSeqByte bool
	seqByte     byte
	addr        net.Addr
}

type queue struct {
	last  time.Time
	queue chan []byte
}

type xicmpConnServer struct {
	conn     net.PacketConn
	icmpConn *icmp.PacketConn

	typ    icmp.Type
	proto  int
	config *Config

	ch            chan *record
	readQueue     chan *packet
	writeQueueMap map[string]*queue

	closed bool
	mutex  sync.Mutex
}

func NewConnServer(c *Config, raw net.PacketConn, level int) (net.PacketConn, error) {
	if level != 0 {
		return nil, errors.New("xicmp requires being at the outermost level")
	}

	network := "ip4:icmp"
	typ := icmp.Type(ipv4.ICMPTypeEchoReply)
	proto := 1
	if strings.Contains(c.Ip, ":") {
		network = "ip6:ipv6-icmp"
		typ = ipv6.ICMPTypeEchoReply
		proto = 58
	}

	icmpConn, err := icmp.ListenPacket(network, c.Ip)
	if err != nil {
		return nil, errors.New("xicmp listen err").Base(err)
	}

	conn := &xicmpConnServer{
		conn:     raw,
		icmpConn: icmpConn,

		typ:    typ,
		proto:  proto,
		config: c,

		ch:            make(chan *record, 500),
		readQueue:     make(chan *packet, 512),
		writeQueueMap: make(map[string]*queue),
	}

	go conn.clean()
	go conn.recvLoop()
	go conn.sendLoop()

	return conn, nil
}

func (c *xicmpConnServer) clean() {
	f := func() bool {
		c.mutex.Lock()
		defer c.mutex.Unlock()

		if c.closed {
			return true
		}

		now := time.Now()

		for key, q := range c.writeQueueMap {
			if now.Sub(q.last) >= idleTimeout {
				close(q.queue)
				delete(c.writeQueueMap, key)
			}
		}

		return false
	}

	for {
		time.Sleep(idleTimeout / 2)
		if f() {
			return
		}
	}
}

func (c *xicmpConnServer) ensureQueue(addr net.Addr) *queue {
	if c.closed {
		return nil
	}

	q, ok := c.writeQueueMap[addr.String()]
	if !ok {
		q = &queue{
			queue: make(chan []byte, 512),
		}
		c.writeQueueMap[addr.String()] = q
	}
	q.last = time.Now()

	return q
}

func (c *xicmpConnServer) encode(p []byte, id int, seq int, needSeqByte bool, seqByte byte) ([]byte, error) {
	data := p
	if needSeqByte {
		b2 := c.randUntil(seqByte)
		data = append([]byte{b2}, p...)
	}

	msg := icmp.Message{
		Type: c.typ,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: data,
		},
	}

	buf, err := msg.Marshal(nil)
	if err != nil {
		return nil, err
	}

	if len(buf) > finalmask.UDPSize {
		return nil, errors.New("xicmp len(buf) > finalmask.UDPSize")
	}

	return buf, nil
}

func (c *xicmpConnServer) randUntil(b1 byte) byte {
	b2 := byte(crypto.RandBetween(0, 255))
	for {
		if b2 != b1 {
			return b2
		}
		b2 = byte(crypto.RandBetween(0, 255))
	}
}

func (c *xicmpConnServer) recvLoop() {
	var buf [finalmask.UDPSize]byte

	for {
		if c.closed {
			break
		}

		n, addr, err := c.icmpConn.ReadFrom(buf[:])
		if err != nil {
			continue
		}

		msg, err := icmp.ParseMessage(c.proto, buf[:n])
		if err != nil {
			continue
		}

		if msg.Type != ipv4.ICMPTypeEcho && msg.Type != ipv6.ICMPTypeEchoRequest {
			continue
		}

		echo, ok := msg.Body.(*icmp.Echo)
		if !ok {
			continue
		}

		if c.config.Id != 0 && echo.ID != int(c.config.Id) {
			continue
		}

		needSeqByte := false
		var seqByte byte

		if len(echo.Data) > 0 {
			needSeqByte = true
			seqByte = echo.Data[0]

			buf := make([]byte, len(echo.Data))
			copy(buf, echo.Data)
			select {
			case c.readQueue <- &packet{
				p: buf,
				addr: &net.UDPAddr{
					IP:   addr.(*net.IPAddr).IP,
					Port: echo.ID,
				},
			}:
			default:
				errors.LogDebug(context.Background(), addr, " ", echo.ID, " ", echo.Seq, " mask read err queue full")
			}
		}

		select {
		case c.ch <- &record{
			id:          echo.ID,
			seq:         echo.Seq,
			needSeqByte: needSeqByte,
			seqByte:     seqByte,
			addr: &net.UDPAddr{
				IP:   addr.(*net.IPAddr).IP,
				Port: echo.ID,
			},
		}:
		default:
			errors.LogDebug(context.Background(), addr, " ", echo.ID, " ", echo.Seq, " mask read err record queue full")
		}
	}

	errors.LogDebug(context.Background(), "xicmp closed")

	close(c.ch)
	close(c.readQueue)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.closed = true
	for key, q := range c.writeQueueMap {
		close(q.queue)
		delete(c.writeQueueMap, key)
	}
}

func (c *xicmpConnServer) sendLoop() {
	var nextRec *record
	for {
		rec := nextRec
		nextRec = nil

		if rec == nil {
			var ok bool
			rec, ok = <-c.ch
			if !ok {
				break
			}
		}

		c.mutex.Lock()
		q := c.ensureQueue(rec.addr)
		if q == nil {
			c.mutex.Unlock()
			return
		}
		c.mutex.Unlock()

		var p []byte

		timer := time.NewTimer(maxResponseDelay)

		select {
		case p = <-q.queue:
		default:
			select {
			case p = <-q.queue:
			case <-timer.C:
			case nextRec = <-c.ch:
			}
		}

		timer.Stop()

		if len(p) == 0 {
			continue
		}

		buf, err := c.encode(p, rec.id, rec.seq, rec.needSeqByte, rec.seqByte)
		if err != nil {
			errors.LogDebug(context.Background(), rec.addr, " ", rec.id, " ", rec.seq, " xicmp wireformat err ", err)
			continue
		}

		if c.closed {
			return
		}

		_, err = c.icmpConn.WriteTo(buf, &net.IPAddr{IP: rec.addr.(*net.UDPAddr).IP})
		if err != nil {
			errors.LogDebug(context.Background(), rec.addr, " ", rec.id, " ", rec.seq, " xicmp writeto err ", err)
		}
	}
}

func (c *xicmpConnServer) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	packet, ok := <-c.readQueue
	if !ok {
		return 0, nil, net.ErrClosed
	}
	if len(p) < len(packet.p) {
		errors.LogDebug(context.Background(), packet.addr, " mask read err short buffer ", len(p), " ", len(packet.p))
		return 0, packet.addr, nil
	}
	copy(p, packet.p)
	return len(packet.p), packet.addr, nil
}

func (c *xicmpConnServer) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if len(p)+8+1 > finalmask.UDPSize {
		errors.LogDebug(context.Background(), addr, " mask write err short write ", len(p), "+8+1 > ", finalmask.UDPSize)
		return 0, nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	q := c.ensureQueue(addr)
	if q == nil {
		return 0, io.ErrClosedPipe
	}

	buf := make([]byte, len(p))
	copy(buf, p)

	select {
	case q.queue <- buf:
		return len(p), nil
	default:
		// errors.LogDebug(context.Background(), addr, " mask write err queue full")
		return 0, nil
	}
}

func (c *xicmpConnServer) Close() error {
	c.closed = true
	_ = c.icmpConn.Close()
	return c.conn.Close()
}

func (c *xicmpConnServer) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: c.icmpConn.LocalAddr().(*net.IPAddr).IP}
}

func (c *xicmpConnServer) SetDeadline(t time.Time) error {
	return c.icmpConn.SetDeadline(t)
}

func (c *xicmpConnServer) SetReadDeadline(t time.Time) error {
	return c.icmpConn.SetReadDeadline(t)
}

func (c *xicmpConnServer) SetWriteDeadline(t time.Time) error {
	return c.icmpConn.SetWriteDeadline(t)
}
