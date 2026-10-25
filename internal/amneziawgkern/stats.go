package amneziawgkern

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PeerStat is one row of `awg show <iface> dump`.
type PeerStat struct {
	PublicKey string
	// Endpoint is "ip:port", or "(none)" while the peer has never been seen.
	Endpoint string
	// LatestHandshake is a unix timestamp; 0 means never.
	LatestHandshake int64
	// RxBytes is what the server received from the peer -- the client's
	// upload. TxBytes is what the server sent -- the client's download.
	RxBytes int64
	TxBytes int64
}

// Delta is one peer's traffic since the previous poll, already corrected for
// counter resets.
type Delta struct {
	PublicKey string
	Up        int64
	Down      int64
}

// Dump reads the live per-peer counters for one interface.
func Dump(iface string) ([]PeerStat, error) {
	out, err := run("", "awg", "show", iface, "dump")
	if err != nil {
		return nil, fmt.Errorf("amneziawgkern: awg show %s dump: %w (%s)", iface, err, out)
	}
	return ParseDump(out)
}

// ParseDump parses the tab-separated output from `awg show <iface> dump`.
func ParseDump(out string) ([]PeerStat, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, nil
	}

	var stats []PeerStat
	// lines[0] is the interface itself (private key, public key, listen port,
	// fwmark) and carries no per-peer counters.
	for _, line := range lines[1:] {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 7 {
			continue
		}
		handshake, err := strconv.ParseInt(f[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amneziawgkern: parsing handshake %q: %w", f[4], err)
		}
		rx, err := strconv.ParseInt(f[5], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amneziawgkern: parsing rx %q: %w", f[5], err)
		}
		tx, err := strconv.ParseInt(f[6], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("amneziawgkern: parsing tx %q: %w", f[6], err)
		}
		stats = append(stats, PeerStat{
			PublicKey:       f[0],
			Endpoint:        f[2],
			LatestHandshake: handshake,
			RxBytes:         rx,
			TxBytes:         tx,
		})
	}
	return stats, nil
}

type counters struct {
	rx, tx int64
}

type endpointSighting struct {
	addr string
	seen time.Time
}

// Collector converts kernel counters to traffic deltas and tracks recent peer
// endpoints; adopted interfaces are primed once before deltas resume.
type Collector struct {
	mu        sync.Mutex
	last      map[string]counters
	endpoints map[string][]endpointSighting
	// emails maps iface/publicKey to the owning client, so observations can be
	// reported against the identity the rest of the panel keys on.
	emails map[string]string
	// handshakes is each peer's latest handshake, the only online signal a
	// kernel interface exposes.
	handshakes map[string]int64
}

func NewCollector() *Collector {
	return &Collector{
		last:       map[string]counters{},
		endpoints:  map[string][]endpointSighting{},
		emails:     map[string]string{},
		handshakes: map[string]int64{},
	}
}

func key(iface, publicKey string) string { return iface + "/" + publicKey }

// Delta records the current counters and returns what changed since the last call.
func (c *Collector) Delta(iface string, stats []PeerStat) []Delta {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]Delta, 0, len(stats))
	for _, s := range stats {
		k := key(iface, s.PublicKey)
		prev, seen := c.last[k]
		up, down := s.RxBytes, s.TxBytes
		if seen {
			up = s.RxBytes - prev.rx
			down = s.TxBytes - prev.tx
			if up < 0 {
				up = s.RxBytes
			}
			if down < 0 {
				down = s.TxBytes
			}
		}
		c.last[k] = counters{rx: s.RxBytes, tx: s.TxBytes}
		if up == 0 && down == 0 {
			continue
		}
		out = append(out, Delta{PublicKey: s.PublicKey, Up: up, Down: down})
	}
	return out
}

// Prime sets an adopted interface's current counters as the starting point.
func (c *Collector) Prime(iface string, stats []PeerStat) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := iface + "/"
	for k := range c.last {
		if strings.HasPrefix(k, prefix) {
			delete(c.last, k)
		}
	}
	for _, s := range stats {
		c.last[key(iface, s.PublicKey)] = counters{rx: s.RxBytes, tx: s.TxBytes}
	}
}

// ObserveEndpoints tracks recent source addresses used by kernel-mode peers.
func (c *Collector) ObserveEndpoints(iface string, stats []PeerStat, now time.Time, window time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := now.Add(-window)
	prefix := iface + "/"
	for k, sightings := range c.endpoints {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		kept := sightings[:0]
		for _, sighting := range sightings {
			if !sighting.seen.Before(cutoff) {
				kept = append(kept, sighting)
			}
		}
		if len(kept) == 0 {
			delete(c.endpoints, k)
		} else {
			c.endpoints[k] = kept
		}
	}
	for _, s := range stats {
		addr := endpointHost(s.Endpoint)
		if addr == "" || s.LatestHandshake <= 0 {
			continue
		}
		seen := time.Unix(s.LatestHandshake, 0)
		if seen.After(now) || seen.Before(cutoff) {
			continue
		}
		k := key(iface, s.PublicKey)
		kept := c.endpoints[k][:0]
		found := false
		for _, e := range c.endpoints[k] {
			if e.addr == addr {
				if seen.After(e.seen) {
					e.seen = seen
				}
				found = true
			}
			kept = append(kept, e)
		}
		if !found {
			kept = append(kept, endpointSighting{addr: addr, seen: seen})
		}
		if len(kept) == 0 {
			delete(c.endpoints, k)
		} else {
			c.endpoints[k] = kept
		}
	}
}

// DistinctEndpoints reports how many different addresses a peer has connected
// from inside the window last passed to ObserveEndpoints.
func (c *Collector) DistinctEndpoints(iface, publicKey string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.endpoints[key(iface, publicKey)])
}

// EndpointsFor returns the tracked addresses for a peer, for surfacing in the
// UI next to the limit that acted on them.
func (c *Collector) EndpointsFor(iface, publicKey string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	seen := c.endpoints[key(iface, publicKey)]
	out := make([]string, 0, len(seen))
	for _, e := range seen {
		out = append(out, e.addr)
	}
	return out
}

// Forget drops all state for an interface that is going away, so a later
// inbound reusing the name cannot inherit stale counters or sightings.
func (c *Collector) Forget(iface string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := iface + "/"
	for k := range c.last {
		if strings.HasPrefix(k, prefix) {
			delete(c.last, k)
		}
	}
	for k := range c.endpoints {
		if strings.HasPrefix(k, prefix) {
			delete(c.endpoints, k)
		}
	}
	for k := range c.emails {
		if strings.HasPrefix(k, prefix) {
			delete(c.emails, k)
		}
	}
	for k := range c.handshakes {
		if strings.HasPrefix(k, prefix) {
			delete(c.handshakes, k)
		}
	}
}

// endpointHost strips the port so a peer behind a NAT that renumbers its
// source port on every rekey is not mistaken for a peer in a new location.
func endpointHost(endpoint string) string {
	e := strings.TrimSpace(endpoint)
	if e == "" || e == "(none)" {
		return ""
	}
	if i := strings.LastIndex(e, "]:"); i >= 0 { // [v6]:port
		return e[:i+1]
	}
	if i := strings.LastIndex(e, ":"); i >= 0 && strings.Count(e, ":") == 1 {
		return e[:i]
	}
	return e
}

var (
	collectorOnce sync.Once
	collector     *Collector
)

// GetCollector returns the process-wide counter/endpoint collector.
func GetCollector() *Collector {
	collectorOnce.Do(func() { collector = NewCollector() })
	return collector
}

// EmailFor returns the client a peer key was last recorded against, so a peer
// that just left the desired set is still charged for its final counters.
func (c *Collector) EmailFor(iface, publicKey string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.emails[key(iface, publicKey)]
}

// SetEmails records which client each peer key belongs to, so observations can
// be reported against the identity the rest of the panel uses.
func (c *Collector) SetEmails(iface string, byPublicKey map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	active := make(map[string]string, len(byPublicKey))
	for pk, email := range byPublicKey {
		if pk != "" && email != "" {
			active[key(iface, pk)] = email
		}
	}
	prefix := iface + "/"
	for k := range c.last {
		if strings.HasPrefix(k, prefix) {
			if _, ok := active[k]; !ok {
				delete(c.last, k)
			}
		}
	}
	for k := range c.endpoints {
		if strings.HasPrefix(k, prefix) {
			if _, ok := active[k]; !ok {
				delete(c.endpoints, k)
			}
		}
	}
	for k := range c.handshakes {
		if strings.HasPrefix(k, prefix) {
			if _, ok := active[k]; !ok {
				delete(c.handshakes, k)
			}
		}
	}
	if c.emails == nil {
		c.emails = map[string]string{}
	}
	for k := range c.emails {
		if strings.HasPrefix(k, prefix) {
			if _, ok := active[k]; !ok {
				delete(c.emails, k)
			}
		}
	}
	for k, email := range active {
		c.emails[k] = email
	}
}

// ClientIPs returns recent peer endpoints as email -> address -> last-seen Unix timestamp.
func (c *Collector) ClientIPs() map[string]map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := map[string]map[string]int64{}
	for k, sightings := range c.endpoints {
		email := c.emails[k]
		if email == "" {
			continue
		}
		for _, s := range sightings {
			addr := strings.Trim(s.addr, "[]")
			if _, ok := out[email]; !ok {
				out[email] = map[string]int64{}
			}
			if prev, seen := out[email][addr]; !seen || s.seen.Unix() > prev {
				out[email][addr] = s.seen.Unix()
			}
		}
	}
	return out
}

// onlineHandshakeAge is how recently a peer must have completed a handshake to count as online.
const onlineHandshakeAge = 3 * time.Minute

// ObserveHandshakes records each peer's latest handshake so online status can be derived.
func (c *Collector) ObserveHandshakes(iface string, stats []PeerStat) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.handshakes == nil {
		c.handshakes = map[string]int64{}
	}
	for _, s := range stats {
		if s.LatestHandshake > 0 {
			c.handshakes[key(iface, s.PublicKey)] = s.LatestHandshake
		}
	}
}

// OnlineEmails returns the clients whose peers handshook recently enough to be
// considered connected.
func (c *Collector) OnlineEmails(now time.Time) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := now.Add(-onlineHandshakeAge).Unix()
	seen := make(map[string]struct{}, len(c.handshakes))
	out := make([]string, 0, len(c.handshakes))
	for k, ts := range c.handshakes {
		if ts < cutoff {
			continue
		}
		email := c.emails[k]
		if email == "" {
			continue
		}
		if _, dup := seen[email]; dup {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	return out
}
