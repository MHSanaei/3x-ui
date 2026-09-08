// Phase 3.6: per-client port-forwarding. A real Go listener bound to each
// forwarded external port relays into the peer's own tunnel-internal
// address via a direct gonet dial -- the mirror image of
// AttachTCPForwarder/AttachUDPHandler (which relay FROM the tunnel TO the
// real world), and this path's replacement for the retired kernel-module
// architecture's PostUp/PostDown iptables DNAT rules: there's no real OS
// network interface here for DNAT to rewrite packets on, the same root
// reason Phase 3.5's IPv6 alias mechanism couldn't reuse NDP-proxy either.
//
// Deliberately dials straight into the gVisor stack rather than relaying
// through Xray's own SOCKS5 inbound the way the outbound direction does
// (relay.go): Xray runs as a genuinely separate OS process
// (internal/xray/process.go), so it has no visibility into this process's
// private, in-memory netstack at all -- a tunnel-internal address like
// 10.8.1.5:8080 has no route from Xray's own freedom outbound; only code
// holding the actual *stack.Stack can reach it. Accepted consequence:
// forwarded-port bytes don't appear in Xray's per-email stats/quota
// counters. This undercounts, it doesn't bypass enforcement -- a
// depleted/disabled client's peer is dropped from the interface's peer list
// entirely by DesiredAmneziaWGInstances, which tears its forwards down too
// as a side effect of Reconcile's own diff below.
package amneziawgnet

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// portForwardProto distinguishes the two sockets a single forwarded port
// needs -- ForwardedPorts has no per-port protocol selector (matches the
// retired DNAT implementation's own unconditional-TCP+UDP contract), so
// every port gets both.
type portForwardProto uint8

const (
	tcpForward portForwardProto = iota
	udpForward
)

// portForwardKey identifies one listener: a specific peer's specific port on
// a specific protocol. Two different peers (even on the same inbound)
// forwarding the same port number get two independent listeners under two
// independent keys -- a same-port collision surfaces as an ordinary bind
// failure on whichever one opens second, not something actively prevented
// here (see the migration plan's Phase 3.6 notes).
type portForwardKey struct {
	email string
	port  int
	proto portForwardProto
}

// portForwardTargetFunc resolves a peer's current tunnel-internal target
// address by email, re-checked on every new connection/session rather than
// captured once at listen time -- so a peer re-IP takes effect for the next
// connection with zero listener churn (see Reconcile's own comment on
// this). false means the peer has no resolvable target right now (removed,
// or its AllowedIPs/ForwardedPorts changed): the caller drops the
// connection/packet, and Reconcile will close the now-undesired listener
// shortly after, if it hasn't already.
type portForwardTargetFunc func(email string) (netip.Addr, bool)

// portForwardListener is the common handle both listenPortForwardTCP and
// listenPortForwardUDP return, so PortForwardSet can hold either behind one
// map value type without a type switch.
type portForwardListener interface {
	Close()
}

// PortForwardSet owns every open port-forward listener for one embedded
// AmneziaWG interface (one per amneziawgnet managed entry -- see
// manager.go). Unlike v6alias.go's stateless desired/diff/apply functions,
// this holds live Go resources (net.Listener/net.PacketConn) that must be
// explicitly closed -- there's no OS-level idempotent recreate the way
// `ip addr add` has -- so Reconcile diffs against its own live listeners
// map directly instead of a remembered prior Instance.
type PortForwardSet struct {
	gstack    *stack.Stack
	inboundID int

	mu          sync.Mutex
	peerTargets map[string]netip.Addr
	listeners   map[portForwardKey]portForwardListener
}

// NewPortForwardSet creates an empty supervisor for one embedded interface's
// stack. Call Reconcile to actually open any listeners.
func NewPortForwardSet(gstack *stack.Stack, inboundID int) *PortForwardSet {
	return &PortForwardSet{
		gstack:      gstack,
		inboundID:   inboundID,
		peerTargets: map[string]netip.Addr{},
		listeners:   map[portForwardKey]portForwardListener{},
	}
}

// desiredPeerTargets resolves each peer's tunnel-internal target address:
// the first IPv4 AllowedIPs entry, falling back to the first IPv6 entry only
// when no v4 entry exists and the instance has IPv6 enabled (mirrors
// desiredV6Aliases' own gating in v6alias.go -- no v6 route exists on the
// stack otherwise). A peer with no resolvable address at all (neither
// family, or an unparseable entry) is simply absent from the result.
func desiredPeerTargets(inst amneziawg.Instance) map[string]netip.Addr {
	out := map[string]netip.Addr{}
	for _, p := range inst.Peers {
		if p.Email == "" {
			continue
		}
		raw := amneziawg.FirstIPv4(p.AllowedIPs)
		if raw == "" && inst.IPv6Enabled {
			raw = amneziawg.FirstIPv6(p.AllowedIPs)
		}
		if raw == "" {
			continue
		}
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			continue
		}
		out[p.Email] = addr
	}
	return out
}

// desiredPortForwardKeys returns the full set of listener keys inst wants
// right now: one tcpForward and one udpForward key per port in every peer's
// ForwardedPorts spec, for every peer that also has a resolvable target
// (see desiredPeerTargets) -- a key never exists without a target, so
// Reconcile can always resolve one for any key it opens.
func desiredPortForwardKeys(inst amneziawg.Instance) map[portForwardKey]struct{} {
	out := map[portForwardKey]struct{}{}
	targets := desiredPeerTargets(inst)
	for _, p := range inst.Peers {
		if p.Email == "" || p.ForwardedPorts == "" {
			continue
		}
		if _, ok := targets[p.Email]; !ok {
			continue
		}
		for _, port := range amneziawg.ExpandForwardedPorts(p.ForwardedPorts) {
			out[portForwardKey{email: p.Email, port: port, proto: tcpForward}] = struct{}{}
			out[portForwardKey{email: p.Email, port: port, proto: udpForward}] = struct{}{}
		}
	}
	return out
}

// Reconcile brings the supervisor's open listeners in line with what inst
// currently wants: closes anything no longer desired, opens anything newly
// desired, leaves everything else untouched. Never returns an error --
// matches applyV6Aliases' contract exactly: one listener failing to bind
// only narrows that specific forward, never a reason to fail the whole
// reconcile.
func (s *PortForwardSet) Reconcile(inst amneziawg.Instance) {
	wantTargets := desiredPeerTargets(inst)
	wantKeys := desiredPortForwardKeys(inst)

	s.mu.Lock()
	s.peerTargets = wantTargets

	var toClose []portForwardListener
	for key, ln := range s.listeners {
		if _, ok := wantKeys[key]; ok {
			continue
		}
		toClose = append(toClose, ln)
		delete(s.listeners, key)
	}
	var toOpen []portForwardKey
	for key := range wantKeys {
		if _, ok := s.listeners[key]; ok {
			continue
		}
		toOpen = append(toOpen, key)
	}
	s.mu.Unlock()

	// Outside the lock: closing/opening real sockets shouldn't block a
	// concurrent targetFor lookup from an in-flight connection on some
	// other, unaffected listener.
	for _, ln := range toClose {
		ln.Close()
	}
	for _, key := range toOpen {
		ln := openPortForwardListener(s.gstack, s.inboundID, key, s.targetFor)
		if ln == nil {
			continue
		}
		s.mu.Lock()
		s.listeners[key] = ln
		s.mu.Unlock()
	}
}

// targetFor implements portForwardTargetFunc against the supervisor's
// current peerTargets snapshot.
func (s *PortForwardSet) targetFor(email string) (netip.Addr, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addr, ok := s.peerTargets[email]
	return addr, ok
}

// Close tears down every open listener. Call when the owning Device is
// closed (or rebuilt -- see manager.go's ensureLocked, which always
// constructs a fresh PortForwardSet alongside a fresh Device.Stack, the
// same reason it also rebuilds udpRelay from scratch rather than reusing
// one bound to a discarded stack).
func (s *PortForwardSet) Close() {
	s.mu.Lock()
	listeners := s.listeners
	s.listeners = map[portForwardKey]portForwardListener{}
	s.mu.Unlock()
	for _, ln := range listeners {
		ln.Close()
	}
}

// openPortForwardListener dispatches to the protocol-specific opener and
// normalizes its result to a real nil interface value on failure -- a
// (*tcpForwardListener)(nil) (or *udpForwardListener(nil)) wrapped directly
// into the portForwardListener interface would be a non-nil interface
// holding a nil pointer, Go's classic trap, so the concrete pointer is
// checked before it's ever assigned into the interface-typed return.
func openPortForwardListener(gstack *stack.Stack, inboundID int, key portForwardKey, target portForwardTargetFunc) portForwardListener {
	switch key.proto {
	case tcpForward:
		if ln := listenPortForwardTCP(gstack, inboundID, key, target); ln != nil {
			return ln
		}
	case udpForward:
		if ln := listenPortForwardUDP(gstack, inboundID, key, target); ln != nil {
			return ln
		}
	}
	return nil
}

const portForwardDialTimeout = 10 * time.Second

// tunnelNetwork returns the gVisor network protocol number matching addr's
// address family, for dialing toward it inside the embedded stack.
func tunnelNetwork(addr netip.Addr) tcpip.NetworkProtocolNumber {
	if addr.Is4() {
		return ipv4.ProtocolNumber
	}
	return ipv6.ProtocolNumber
}

// tunnelFullAddress builds the tcpip.FullAddress a gonet dial needs to
// reach addr:port inside the embedded stack -- NIC 1, matching
// createNetTUNWithStack's own CreateNIC(1, ...) (this package's stack only
// ever registers one NIC, and WriteUDPReply's WriteRawPacket already
// addresses it explicitly the same way elsewhere in this package, rather
// than relying on NIC 0's route-table auto-selection).
func tunnelFullAddress(addr netip.Addr, port int) tcpip.FullAddress {
	return tcpip.FullAddress{NIC: 1, Addr: tcpip.AddrFromSlice(addr.AsSlice()), Port: uint16(port)}
}

// tcpForwardListener is one open host-facing TCP listener for a single
// portForwardKey.
type tcpForwardListener struct {
	ln      net.Listener
	closing chan struct{}
}

// listenPortForwardTCP opens a host-facing TCP listener on key.port and
// starts relaying accepted connections into the tunnel toward
// target(key.email). A bind failure (most commonly EADDRINUSE, whether from
// an unrelated process or another AmneziaWG peer/inbound that already
// claimed the same port) is logged and returns nil; Reconcile treats a nil
// result as "not open this round" and retries on every future Reconcile
// call for as long as the key stays desired.
func listenPortForwardTCP(gstack *stack.Stack, inboundID int, key portForwardKey, target portForwardTargetFunc) *tcpForwardListener {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf(":%d", key.port))
	if err != nil {
		logger.Warningf("amneziawgnet: port-forward: inbound %d peer %q: listen tcp :%d: %v", inboundID, key.email, key.port, err)
		return nil
	}
	l := &tcpForwardListener{ln: ln, closing: make(chan struct{})}
	logger.Infof("amneziawgnet: port-forward: inbound %d peer %q: listening tcp :%d", inboundID, key.email, key.port)
	go l.acceptLoop(gstack, inboundID, key, target)
	return l
}

func (l *tcpForwardListener) acceptLoop(gstack *stack.Stack, inboundID int, key portForwardKey, target portForwardTargetFunc) {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			select {
			case <-l.closing:
				return // intentional shutdown, not a real accept error
			default:
			}
			logger.Warningf("amneziawgnet: port-forward: inbound %d peer %q: accept tcp :%d: %v", inboundID, key.email, key.port, err)
			return
		}
		go relayTCPForward(gstack, conn, inboundID, key, target)
	}
}

func relayTCPForward(gstack *stack.Stack, conn net.Conn, inboundID int, key portForwardKey, target portForwardTargetFunc) {
	defer conn.Close()
	addr, ok := target(key.email)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), portForwardDialTimeout)
	defer cancel()
	tunnelConn, err := gonet.DialContextTCP(ctx, gstack, tunnelFullAddress(addr, key.port), tunnelNetwork(addr))
	if err != nil {
		logger.Warningf("amneziawgnet: port-forward: inbound %d peer %q: dial tunnel %s:%d: %v", inboundID, key.email, addr, key.port, err)
		return
	}
	defer tunnelConn.Close()

	pipeBothWays(conn, tunnelConn)
}

// Close stops accepting new connections. Already-relaying connections are
// left to finish on their own -- there's no shared state to tear down early
// for, and an abrupt cut would just look like a network error to whichever
// external client was mid-transfer.
func (l *tcpForwardListener) Close() {
	close(l.closing)
	l.ln.Close()
}
