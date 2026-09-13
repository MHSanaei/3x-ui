package tuic

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// A QUIC flow the sidecar has not touched for this long is forgotten; QUIC's
// own max_idle_time (15s by default) closes the session well before that.
const relayFlowIdle = 2 * time.Minute

const (
	relaySocketBuffer = 4 << 20
	maxRelayFlows     = 4096
)

// udpRelay owns an inbound's public UDP port and counts the bytes it forwards to
// the sidecar on loopback: tuic-server has no stats API and /proc/io stays at 0.
type udpRelay struct {
	public    *net.UDPConn
	upstream  *net.UDPAddr
	idle      time.Duration
	maxFlows  int
	up        atomic.Int64
	down      atomic.Int64
	mu        sync.Mutex
	flows     map[string]*relayFlow
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

type relayFlow struct {
	conn     *net.UDPConn
	client   *net.UDPAddr
	lastSeen atomic.Int64
}

func startUDPRelay(bind string, upstream *net.UDPAddr, idle time.Duration) (*udpRelay, error) {
	addr, err := net.ResolveUDPAddr("udp", bind)
	if err != nil {
		return nil, err
	}
	public, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	_ = public.SetReadBuffer(relaySocketBuffer)
	_ = public.SetWriteBuffer(relaySocketBuffer)
	r := &udpRelay{
		public:   public,
		upstream: upstream,
		idle:     idle,
		maxFlows: maxRelayFlows,
		flows:    make(map[string]*relayFlow),
		done:     make(chan struct{}),
	}
	r.wg.Add(2)
	go r.serve()
	go r.sweep()
	return r, nil
}

func freeLoopbackUDPPort() (int, error) {
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return 0, err
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port, nil
}

func (r *udpRelay) LocalAddr() net.Addr {
	return r.public.LocalAddr()
}

// CollectTraffic returns the client-to-sidecar and sidecar-to-client bytes
// relayed since the previous call.
func (r *udpRelay) CollectTraffic() (up, down int64) {
	return r.up.Swap(0), r.down.Swap(0)
}

func (r *udpRelay) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		close(r.done)
		_ = r.public.Close()
		r.mu.Lock()
		for key, f := range r.flows {
			_ = f.conn.Close()
			delete(r.flows, key)
		}
		r.mu.Unlock()
		r.wg.Wait()
	})
}

func (r *udpRelay) serve() {
	defer r.wg.Done()
	buf := make([]byte, 65535)
	for {
		n, client, err := r.public.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		flow, err := r.flowFor(client)
		if err != nil {
			continue
		}
		if _, err := flow.conn.Write(buf[:n]); err == nil {
			r.up.Add(int64(n))
		}
	}
}

func (r *udpRelay) flowFor(client *net.UDPAddr) (*relayFlow, error) {
	key := client.String()
	now := time.Now().UnixMilli()
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.done:
		return nil, net.ErrClosed
	default:
	}
	if f, ok := r.flows[key]; ok {
		f.lastSeen.Store(now)
		return f, nil
	}
	if len(r.flows) >= r.maxFlows {
		r.evictLeastRecentLocked()
	}
	conn, err := net.DialUDP("udp", nil, r.upstream)
	if err != nil {
		return nil, err
	}
	_ = conn.SetReadBuffer(relaySocketBuffer)
	_ = conn.SetWriteBuffer(relaySocketBuffer)
	f := &relayFlow{conn: conn, client: client}
	f.lastSeen.Store(now)
	r.flows[key] = f
	r.wg.Add(1)
	go r.pump(f)
	return f, nil
}

// Refusing a newcomer at the cap let 4096 junk datagrams lock every new client
// out until the sweep; the flow last seen longest ago is the junk one.
func (r *udpRelay) evictLeastRecentLocked() {
	var oldestKey string
	oldest := int64(-1)
	for key, f := range r.flows {
		if seen := f.lastSeen.Load(); oldest < 0 || seen < oldest {
			oldest, oldestKey = seen, key
		}
	}
	if f, ok := r.flows[oldestKey]; ok {
		_ = f.conn.Close()
		delete(r.flows, oldestKey)
	}
}

func (r *udpRelay) pump(f *relayFlow) {
	defer r.wg.Done()
	buf := make([]byte, 65535)
	for {
		n, err := f.conn.Read(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// ICMP unreachable while the sidecar restarts: drop it, keep the flow.
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if _, err := r.public.WriteToUDP(buf[:n], f.client); err == nil {
			r.down.Add(int64(n))
		}
		f.lastSeen.Store(time.Now().UnixMilli())
	}
}

func (r *udpRelay) sweep() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.idle / 2)
	defer ticker.Stop()
	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-r.idle).UnixMilli()
			r.mu.Lock()
			for key, f := range r.flows {
				if f.lastSeen.Load() < cutoff {
					_ = f.conn.Close()
					delete(r.flows, key)
				}
			}
			r.mu.Unlock()
		}
	}
}
