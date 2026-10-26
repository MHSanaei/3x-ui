package amneziawgnet

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/time/rate"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

const (
	EchoMaxPacket      = 1200
	echoTimeout        = 3 * time.Second
	echoMaxPending     = 4096
	echoMaxPeerPending = 256
)

// EchoGate must reauthorize identity and commit native traffic before returning success.
type EchoGate func(inbound int, peer amneziawg.Peer, src, dst netip.Addr, up, down int64, expected string) (string, error)

type echoRequest struct {
	src, dst netip.Addr
	message  []byte
}

func echoChecksum(p []byte) uint16 {
	var sum uint32
	for len(p) >= 2 {
		sum += uint32(binary.BigEndian.Uint16(p))
		p = p[2:]
	}
	if len(p) > 0 {
		sum += uint32(p[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 65535) + (sum >> 16)
	}
	return ^uint16(sum)
}

func parseEcho(p []byte) (echoRequest, error) {
	if len(p) < 28 || len(p) > EchoMaxPacket || p[0] != 0x45 || p[9] != 1 || int(binary.BigEndian.Uint16(p[2:4])) != len(p) || binary.BigEndian.Uint16(p[6:8])&0xbfff != 0 || p[8] == 0 || echoChecksum(p[:20]) != 0 {
		return echoRequest{}, fmt.Errorf("invalid IPv4 Echo packet")
	}
	msg := p[20:]
	if msg[0] != 8 || msg[1] != 0 || echoChecksum(msg) != 0 {
		return echoRequest{}, fmt.Errorf("invalid Echo Request")
	}
	return echoRequest{src: netip.AddrFrom4([4]byte(p[12:16])), dst: netip.AddrFrom4([4]byte(p[16:20])), message: bytes.Clone(msg)}, nil
}

func echoReplyPacket(req echoRequest) []byte {
	p := make([]byte, 20+len(req.message))
	p[0] = 0x45
	p[8] = 64
	p[9] = 1
	binary.BigEndian.PutUint16(p[2:4], uint16(len(p)))
	src, dst := req.dst.As4(), req.src.As4()
	copy(p[12:16], src[:])
	copy(p[16:20], dst[:])
	copy(p[20:], req.message)
	p[20] = 0
	p[22] = 0
	p[23] = 0
	binary.BigEndian.PutUint16(p[22:24], echoChecksum(p[20:]))
	binary.BigEndian.PutUint16(p[10:12], echoChecksum(p[:20]))
	return p
}

type echoSocket interface {
	ReadFrom([]byte) (int, net.Addr, error)
	WriteTo([]byte, net.Addr) (int, error)
	Close() error
}

func openEchoSocket() (echoSocket, error) { return icmp.ListenPacket("ip4:icmp", "0.0.0.0") }

// EchoSocketAvailable probes capability without sending traffic or changing host privileges.
func EchoSocketAvailable() bool {
	s, err := openEchoSocket()
	if err != nil {
		return false
	}
	_ = s.Close()
	return true
}

type echoPending struct {
	request  echoRequest
	peer     amneziawg.Peer
	wire     []byte
	identity string
	expires  time.Time
}

type echoPeerBudget struct {
	limiter *rate.Limiter
	pending int
	last    time.Time
}

var echoQueueSlots = make(chan struct{}, 4096)

var echoGlobalBudget = struct {
	sync.Mutex
	limiter   *rate.Limiter
	pending   int
	peers     map[string]*echoPeerBudget
	lastSweep time.Time
}{limiter: rate.NewLimiter(1024, 2048), peers: map[string]*echoPeerBudget{}}

type echoRelay struct {
	queue      chan []byte
	workerDone chan struct{}
	mu         sync.Mutex
	inbound    int
	gate       EchoGate
	lookup     func(netip.Addr) (amneziawg.Peer, bool)
	emit       func([]byte) bool
	forbidden  func(netip.Addr) bool
	open       func() (echoSocket, error)
	socket     echoSocket
	pending    map[uint32]echoPending
	next       uint32
	closed     bool
	done       chan struct{}
	wg         sync.WaitGroup
}

func newEchoRelay(id int, gate EchoGate, lookup func(netip.Addr) (amneziawg.Peer, bool), emit func([]byte) bool, forbidden func(netip.Addr) bool) *echoRelay {
	r := &echoRelay{inbound: id, gate: gate, lookup: lookup, emit: emit, forbidden: forbidden, open: openEchoSocket, pending: map[uint32]echoPending{}, done: make(chan struct{}), queue: make(chan []byte, 512), workerDone: make(chan struct{})}
	var seed [4]byte
	if _, err := rand.Read(seed[:]); err == nil {
		r.next = binary.BigEndian.Uint32(seed[:])
	}
	r.wg.Add(1)
	go r.sweep()
	go r.work()
	return r
}

func (r *echoRelay) dropLocked(key uint32) {
	p, ok := r.pending[key]
	if !ok {
		return
	}
	delete(r.pending, key)
	echoGlobalBudget.Lock()
	if b := echoGlobalBudget.peers[p.peer.Email]; b != nil {
		b.pending--
	}
	echoGlobalBudget.pending--
	echoGlobalBudget.Unlock()
}

func (r *echoRelay) sweep() {
	defer r.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.done:
			return
		case now := <-ticker.C:
			r.mu.Lock()
			for k, p := range r.pending {
				if !now.Before(p.expires) {
					r.dropLocked(k)
				}
			}
			echoGlobalBudget.Lock()
			if now.Sub(echoGlobalBudget.lastSweep) >= time.Second {
				for k, b := range echoGlobalBudget.peers {
					if b.pending == 0 && now.Sub(b.last) > time.Minute {
						delete(echoGlobalBudget.peers, k)
					}
				}
				echoGlobalBudget.lastSweep = now
			}
			echoGlobalBudget.Unlock()
			r.mu.Unlock()
		}
	}
}

func (r *echoRelay) handle(packet []byte) {
	if len(packet) > EchoMaxPacket {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	select {
	case echoQueueSlots <- struct{}{}:
	default:
		return
	}
	select {
	case r.queue <- bytes.Clone(packet):
	default:
		<-echoQueueSlots
	}
}

func (r *echoRelay) work() {
	defer close(r.workerDone)
	defer func() {
		for {
			select {
			case <-r.queue:
				<-echoQueueSlots
			default:
				return
			}
		}
	}()
	for {
		select {
		case <-r.done:
			return
		case packet := <-r.queue:
			r.process(packet)
			<-echoQueueSlots
		}
	}
}

func (r *echoRelay) process(packet []byte) {
	req, err := parseEcho(packet)
	if err != nil || r.gate == nil || r.forbidden(req.dst) {
		return
	}
	peer, ok := r.lookup(req.src)
	if !ok {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	echoGlobalBudget.Lock()
	b := echoGlobalBudget.peers[peer.Email]
	if b == nil {
		if len(echoGlobalBudget.peers) >= 4096 {
			echoGlobalBudget.Unlock()
			r.mu.Unlock()
			return
		}
		b = &echoPeerBudget{limiter: rate.NewLimiter(128, 256)}
		echoGlobalBudget.peers[peer.Email] = b
	}
	b.last = time.Now()
	admitted := b.pending < echoMaxPeerPending && b.limiter.Allow() && echoGlobalBudget.pending < echoMaxPending && echoGlobalBudget.limiter.Allow()
	if admitted {
		echoGlobalBudget.pending++
		b.pending++
	}
	echoGlobalBudget.Unlock()
	if !admitted {
		r.mu.Unlock()
		return
	}

	r.next++
	for {
		if _, exists := r.pending[r.next]; !exists {
			break
		}
		r.next++
	}
	key := r.next
	wire := make([]byte, len(req.message)+16)
	copy(wire, req.message)
	binary.BigEndian.PutUint32(wire[4:8], key)
	if _, err := rand.Read(wire[len(wire)-16:]); err != nil {
		echoGlobalBudget.Lock()
		echoGlobalBudget.pending--
		b.pending--
		echoGlobalBudget.Unlock()
		r.mu.Unlock()
		return
	}
	wire[2] = 0
	wire[3] = 0
	binary.BigEndian.PutUint16(wire[2:4], echoChecksum(wire))
	r.pending[key] = echoPending{request: req, peer: peer, wire: wire, expires: time.Now().Add(echoTimeout)}
	r.mu.Unlock()

	identity, err := r.gate(r.inbound, peer, req.src, req.dst, int64(len(packet)), 0, "")
	r.mu.Lock()
	defer r.mu.Unlock()
	p, exists := r.pending[key]
	if !exists {
		return
	}
	if err != nil || r.closed || !time.Now().Before(p.expires) {
		r.dropLocked(key)
		return
	}
	current, ok := r.lookup(req.src)
	if !ok || current.PublicKey != peer.PublicKey || current.Email != peer.Email || r.forbidden(req.dst) {
		r.dropLocked(key)
		return
	}
	if r.socket == nil {
		socket, e := r.open()
		if e != nil {
			r.dropLocked(key)
			return
		}
		r.socket = socket
		r.wg.Add(1)
		go r.read(socket)
	}
	p.identity = identity
	r.pending[key] = p
	if n, e := r.socket.WriteTo(wire, &net.IPAddr{IP: net.IP(req.dst.AsSlice())}); e != nil || n != len(wire) {
		r.dropLocked(key)
	}
}

func (r *echoRelay) read(socket echoSocket) {
	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.socket == socket {
			_ = socket.Close()
			r.socket = nil
			for key := range r.pending {
				r.dropLocked(key)
			}
		}
	}()
	defer r.wg.Done()
	buf := make([]byte, 65536)
	for {
		n, addr, err := socket.ReadFrom(buf)
		if err != nil {
			return
		}
		ip, ok := addr.(*net.IPAddr)
		if !ok {
			continue
		}
		src, ok := netip.AddrFromSlice(ip.IP)
		if !ok {
			continue
		}
		r.receive(src.Unmap(), buf[:n])
	}
}

func (r *echoRelay) receive(src netip.Addr, msg []byte) {
	if len(msg) < 24 || msg[0] != 0 || msg[1] != 0 || echoChecksum(msg) != 0 {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	key := binary.BigEndian.Uint32(msg[4:8])
	p, ok := r.pending[key]
	if !ok || src != p.request.dst || !time.Now().Before(p.expires) {
		r.mu.Unlock()
		return
	}
	expected := bytes.Clone(p.wire)
	expected[0] = 0
	expected[2] = 0
	expected[3] = 0
	binary.BigEndian.PutUint16(expected[2:4], echoChecksum(expected))
	if !bytes.Equal(expected, msg) {
		r.mu.Unlock()
		return
	}
	r.dropLocked(key)
	r.mu.Unlock()
	current, ok := r.lookup(p.request.src)
	if !ok || current.PublicKey != p.peer.PublicKey || current.Email != p.peer.Email || r.forbidden(src) {
		return
	}
	reply := echoReplyPacket(p.request)
	if _, err := r.gate(r.inbound, p.peer, p.request.src, src, 0, int64(len(reply)), p.identity); err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.emit(reply)
	}
}

func (r *echoRelay) close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.done)
	if r.socket != nil {
		_ = r.socket.Close()
	}
	for k := range r.pending {
		r.dropLocked(k)
	}
	r.mu.Unlock()
	// Gates may be waiting behind a lifecycle writer; teardown must not wait for them.
}
