package xdns

import (
	"bytes"
	"context"
	"encoding/binary"
	go_errors "errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)

const (
	idleTimeout      = 10 * time.Second
	responseTTL      = 60
	maxResponseDelay = 1 * time.Second
)

var (
	maxUDPPayload     = 1280 - 40 - 8
	maxEncodedPayload = computeMaxEncodedPayload(maxUDPPayload)
)

func clientIDToAddr(clientID [8]byte) *net.UDPAddr {
	ip := make(net.IP, 16)

	copy(ip, []byte{0xfd, 0x00, 0, 0, 0, 0, 0, 0})
	copy(ip[8:], clientID[:])

	return &net.UDPAddr{
		IP: ip,
	}
}

type record struct {
	Resp *Message
	Addr net.Addr
	// ClientID [8]byte
	ClientAddr net.Addr
}

type queue struct {
	last  time.Time
	queue chan []byte
	stash chan []byte
}

type xdnsConnServer struct {
	net.PacketConn

	domain Name

	ch            chan *record
	readQueue     chan *packet
	writeQueueMap map[string]*queue

	closed bool
	mutex  sync.Mutex
}

func NewConnServer(c *Config, raw net.PacketConn) (net.PacketConn, error) {
	domain, err := ParseName(c.Domain)
	if err != nil {
		return nil, err
	}

	conn := &xdnsConnServer{
		PacketConn: raw,

		domain: domain,

		ch:            make(chan *record, 500),
		readQueue:     make(chan *packet, 512),
		writeQueueMap: make(map[string]*queue),
	}

	go conn.clean()
	go conn.recvLoop()
	go conn.sendLoop()

	return conn, nil
}

func (c *xdnsConnServer) clean() {
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
				close(q.stash)
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

func (c *xdnsConnServer) ensureQueue(addr net.Addr) *queue {
	if c.closed {
		return nil
	}

	q, ok := c.writeQueueMap[addr.String()]
	if !ok {
		q = &queue{
			queue: make(chan []byte, 512),
			stash: make(chan []byte, 1),
		}
		c.writeQueueMap[addr.String()] = q
	}
	q.last = time.Now()

	return q
}

func (c *xdnsConnServer) stash(queue *queue, p []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return
	}

	select {
	case queue.stash <- p:
	default:
	}
}

func (c *xdnsConnServer) recvLoop() {
	var buf [finalmask.UDPSize]byte

	for {
		if c.closed {
			break
		}

		n, addr, err := c.PacketConn.ReadFrom(buf[:])
		if err != nil || n == 0 {
			if go_errors.Is(err, net.ErrClosed) || go_errors.Is(err, io.EOF) {
				break
			}
			continue
		}

		query, err := MessageFromWireFormat(buf[:n])
		if err != nil {
			errors.LogDebug(context.Background(), addr, " xdns from wireformat err ", err)
			continue
		}

		resp, payload := responseFor(&query, c.domain)

		var clientID [8]byte
		n = copy(clientID[:], payload)
		payload = payload[n:]
		if n == len(clientID) {
			r := bytes.NewReader(payload)
			for {
				p, err := nextPacketServer(r)
				if err != nil {
					break
				}

				buf := make([]byte, len(p))
				copy(buf, p)
				select {
				case c.readQueue <- &packet{
					p:    buf,
					addr: clientIDToAddr(clientID),
				}:
				default:
					errors.LogDebug(context.Background(), addr, " ", clientID, " mask read err queue full")
				}
			}
		} else {
			if resp != nil && resp.Rcode() == RcodeNoError {
				resp.Flags |= RcodeNameError
			}
		}

		if resp != nil {
			select {
			case c.ch <- &record{resp, addr, clientIDToAddr(clientID)}:
			default:
				errors.LogDebug(context.Background(), addr, " ", clientID, " mask read err record queue full")
			}
		}
	}

	errors.LogDebug(context.Background(), "xdns closed")

	close(c.ch)
	close(c.readQueue)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.closed = true
	for key, q := range c.writeQueueMap {
		close(q.queue)
		close(q.stash)
		delete(c.writeQueueMap, key)
	}
}

func (c *xdnsConnServer) sendLoop() {
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

		if rec.Resp.Rcode() == RcodeNoError && len(rec.Resp.Question) == 1 {
			rec.Resp.Answer = []RR{
				{
					Name:  rec.Resp.Question[0].Name,
					Type:  rec.Resp.Question[0].Type,
					Class: rec.Resp.Question[0].Class,
					TTL:   responseTTL,
					Data:  nil,
				},
			}

			var payload bytes.Buffer
			limit := maxEncodedPayload
			timer := time.NewTimer(maxResponseDelay)

			for {
				c.mutex.Lock()
				q := c.ensureQueue(rec.ClientAddr)
				if q == nil {
					c.mutex.Unlock()
					return
				}
				c.mutex.Unlock()

				var p []byte

				select {
				case p = <-q.stash:
				default:
					select {
					case p = <-q.stash:
					case p = <-q.queue:
					default:
						select {
						case p = <-q.stash:
						case p = <-q.queue:
						case <-timer.C:
						case nextRec = <-c.ch:
						}
					}
				}

				timer.Reset(0)

				if len(p) == 0 {
					break
				}

				limit -= 2 + len(p)
				if payload.Len() > 0 && limit < 0 {
					c.stash(q, p)
					break
				}

				// if len(p) > 65535 {
				// 	panic(len(p))
				// }

				_ = binary.Write(&payload, binary.BigEndian, uint16(len(p)))
				payload.Write(p)
			}

			timer.Stop()
			rec.Resp.Answer[0].Data = EncodeRDataTXT(payload.Bytes())
		}

		buf, err := rec.Resp.WireFormat()
		if err != nil {
			errors.LogDebug(context.Background(), rec.Addr, " ", rec.ClientAddr, " xdns wireformat err ", err)
			continue
		}

		if len(buf) > maxUDPPayload {
			errors.LogDebug(context.Background(), rec.Addr, " ", rec.ClientAddr, " xdns truncate ", len(buf))
			buf = buf[:maxUDPPayload]
			buf[2] |= 0x02
		}

		if c.closed {
			return
		}

		_, err = c.PacketConn.WriteTo(buf, rec.Addr)
		if go_errors.Is(err, net.ErrClosed) || go_errors.Is(err, io.ErrClosedPipe) {
			c.closed = true
			break
		}
	}
}

func (c *xdnsConnServer) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
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

func (c *xdnsConnServer) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if len(p)+2 > maxEncodedPayload {
		errors.LogDebug(context.Background(), addr, " mask write err short write ", len(p), "+2 > ", maxEncodedPayload)
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

func (c *xdnsConnServer) Close() error {
	c.closed = true
	return c.PacketConn.Close()
}

func nextPacketServer(r *bytes.Reader) ([]byte, error) {
	eof := func(err error) error {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	for {
		prefix, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if prefix >= 224 {
			paddingLen := prefix - 224
			_, err := io.CopyN(io.Discard, r, int64(paddingLen))
			if err != nil {
				return nil, eof(err)
			}
		} else {
			p := make([]byte, int(prefix))
			_, err = io.ReadFull(r, p)
			return p, eof(err)
		}
	}
}

func responseFor(query *Message, domain Name) (*Message, []byte) {
	resp := &Message{
		ID:       query.ID,
		Flags:    0x8000,
		Question: query.Question,
	}

	if query.Flags&0x8000 != 0 {
		return nil, nil
	}

	payloadSize := 0
	for _, rr := range query.Additional {
		if rr.Type != RRTypeOPT {
			continue
		}
		if len(resp.Additional) != 0 {
			resp.Flags |= RcodeFormatError
			return resp, nil
		}
		resp.Additional = append(resp.Additional, RR{
			Name:  Name{},
			Type:  RRTypeOPT,
			Class: 4096,
			TTL:   0,
			Data:  []byte{},
		})
		additional := &resp.Additional[0]

		version := (rr.TTL >> 16) & 0xff
		if version != 0 {
			resp.Flags |= ExtendedRcodeBadVers & 0xf
			additional.TTL = (ExtendedRcodeBadVers >> 4) << 24
			return resp, nil
		}

		payloadSize = int(rr.Class)
	}
	if payloadSize < 512 {
		payloadSize = 512
	}

	if len(query.Question) != 1 {
		resp.Flags |= RcodeFormatError
		return resp, nil
	}
	question := query.Question[0]

	prefix, ok := question.Name.TrimSuffix(domain)
	if !ok {
		resp.Flags |= RcodeNameError
		return resp, nil
	}
	resp.Flags |= 0x0400

	if query.Opcode() != 0 {
		resp.Flags |= RcodeNotImplemented
		return resp, nil
	}

	if question.Type != RRTypeTXT {
		resp.Flags |= RcodeNameError
		return resp, nil
	}

	encoded := bytes.ToUpper(bytes.Join(prefix, nil))
	payload := make([]byte, base32Encoding.DecodedLen(len(encoded)))
	n, err := base32Encoding.Decode(payload, encoded)
	if err != nil {
		resp.Flags |= RcodeNameError
		return resp, nil
	}
	payload = payload[:n]

	if payloadSize < maxUDPPayload {
		resp.Flags |= RcodeFormatError
		return resp, nil
	}

	return resp, payload
}

func computeMaxEncodedPayload(limit int) int {
	maxLengthName, err := NewName([][]byte{
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
	})
	if err != nil {
		panic(err)
	}
	{
		n := 0
		for _, label := range maxLengthName {
			n += len(label) + 1
		}
		n += 1
		if n != 255 {
			panic("computeMaxEncodedPayload n != 255")
		}
	}

	queryLimit := uint16(limit)
	if int(queryLimit) != limit {
		queryLimit = 0xffff
	}
	query := &Message{
		Question: []Question{
			{
				Name:  maxLengthName,
				Type:  RRTypeTXT,
				Class: RRTypeTXT,
			},
		},

		Additional: []RR{
			{
				Name:  Name{},
				Type:  RRTypeOPT,
				Class: queryLimit,
				TTL:   0,
				Data:  []byte{},
			},
		},
	}
	resp, _ := responseFor(query, [][]byte{})

	resp.Answer = []RR{
		{
			Name:  query.Question[0].Name,
			Type:  query.Question[0].Type,
			Class: query.Question[0].Class,
			TTL:   responseTTL,
			Data:  nil,
		},
	}

	low := 0
	high := 32768
	for low+1 < high {
		mid := (low + high) / 2
		resp.Answer[0].Data = EncodeRDataTXT(make([]byte, mid))
		buf, err := resp.WireFormat()
		if err != nil {
			panic(err)
		}
		if len(buf) <= limit {
			low = mid
		} else {
			high = mid
		}
	}

	return low
}
