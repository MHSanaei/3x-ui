package tuic

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const (
	relayOpenTimeout     = 10 * time.Second
	maxPendingRelayOpens = 256
)

var ErrRelayQueueFull = errors.New("tuic relay: pending connection limit reached")

var relayGates [relayPortSlots]atomic.Pointer[RelayGate]

type RelayGate struct {
	mu       sync.Mutex
	writer   chan struct{}
	changed  chan struct{}
	paused   bool
	ready    bool
	owner    int
	accounts map[string]struct{}
	active   int
	waiting  int
}

// ManagedRelayGate uses the same bounded slots as the loopback relay ports.
// The owner check prevents a reused slot from forwarding through a deleted inbound.
func ManagedRelayGate(port int) *RelayGate {
	index := port - SOCKSBasePort - 1
	if index < 0 || index >= len(relayGates) {
		return nil
	}
	if gate := relayGates[index].Load(); gate != nil {
		return gate
	}
	gate := &RelayGate{writer: make(chan struct{}, 1), changed: make(chan struct{})}
	if relayGates[index].CompareAndSwap(nil, gate) {
		return gate
	}
	return relayGates[index].Load()
}

func (g *RelayGate) signalLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

func (g *RelayGate) acquire(ctx context.Context, owner int, email string) (func(), error) {
	g.mu.Lock()
	if g.waiting >= maxPendingRelayOpens {
		g.mu.Unlock()
		return nil, ErrRelayQueueFull
	}
	g.waiting++
	defer func() {
		g.waiting--
		g.mu.Unlock()
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, admitted := g.accounts[email]
		if !g.paused && g.ready && g.owner == owner && admitted {
			g.active++
			return func() {
				g.mu.Lock()
				g.active--
				g.signalLocked()
				g.mu.Unlock()
			}, nil
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
		}
		g.mu.Lock()
	}
}

type RelayUpdate struct{ gate *RelayGate }

func (g *RelayGate) ReadyFor(owner int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ready && g.owner == owner
}

func (g *RelayGate) Invalidate(owner int) {
	g.mu.Lock()
	g.owner, g.ready, g.accounts = owner, false, nil
	g.signalLocked()
	g.mu.Unlock()
}

// BeginUpdate blocks new handshakes and drains only local SOCKS authentication.
// Existing TCP streams and UDP associations do not hold this gate.
func (g *RelayGate) BeginUpdate(ctx context.Context) (*RelayUpdate, error) {
	select {
	case g.writer <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	g.mu.Lock()
	g.paused = true
	g.signalLocked()
	for {
		if err := ctx.Err(); err != nil {
			g.paused = false
			g.signalLocked()
			g.mu.Unlock()
			<-g.writer
			return nil, err
		}
		if g.active == 0 {
			break
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
		}
		g.mu.Lock()
	}
	g.mu.Unlock()
	return &RelayUpdate{gate: g}, nil
}

func (u *RelayUpdate) Finish(owner int, emails []string, available bool) {
	g := u.gate
	g.mu.Lock()
	g.owner, g.ready = owner, available
	g.accounts = make(map[string]struct{}, len(emails))
	for _, email := range emails {
		g.accounts[email] = struct{}{}
	}
	g.paused = false
	g.signalLocked()
	g.mu.Unlock()
	<-g.writer
}

func (u *RelayUpdate) Cancel() {
	g := u.gate
	g.mu.Lock()
	g.paused = false
	g.signalLocked()
	g.mu.Unlock()
	<-g.writer
}
