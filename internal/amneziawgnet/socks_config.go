package amneziawgnet

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
)

// SOCKSBasePort is the first loopback port used for an AmneziaWG inbound's
// own Xray SOCKS5 relay inbound (see relay.go/SocksInboundSettings). Its own
// range, distinct from amneziawg.EgressBasePort (63100, the kernel-module
// path's TPROXY bridge port) so the two can never collide even if both
// happen to be reachable during a transition.
const SOCKSBasePort = 65100

// SOCKSPortForInbound returns the loopback port of one AmneziaWG inbound's
// own Xray SOCKS5 relay inbound, derived deterministically from its id so
// the config-generation code (which builds the inbound) and the relay code
// (which dials it) never have to agree on a runtime-negotiated value --
// mirrors amneziawg.EgressPortForInbound's own reasoning exactly.
func SOCKSPortForInbound(inboundID int) int {
	return SOCKSBasePort + inboundID
}

var (
	socksPasswordOnce sync.Once
	socksPassword     string
)

// SocksPassword returns the process-wide password used to authenticate into
// every AmneziaWG SOCKS5 relay inbound, generating and caching it once
// (lazily, on first use) rather than persisting it anywhere: this traffic
// never leaves loopback, both the config generator (SocksInboundSettings'
// caller) and the relay dialer (SocksRelay/UDPRelay) live in this same
// process, and Xray's own generated config is already rebuilt from scratch
// on every reconcile -- there is nothing for a stored value to survive
// across that a fresh one wouldn't equally satisfy. Not a real secret (see
// SocksRelay's own doc comment); this only needs to be unpredictable enough
// that nothing outside this process could plausibly guess it and dial in
// over loopback.
func SocksPassword() string {
	socksPasswordOnce.Do(func() {
		var b [24]byte
		if _, err := rand.Read(b[:]); err != nil {
			// crypto/rand failing is effectively unrecoverable for a
			// process that generates real WireGuard keys elsewhere too;
			// a fixed fallback keeps this from panicking outright.
			socksPassword = fmt.Sprintf("amneziawgnet-fallback-%x", b)
			return
		}
		socksPassword = base64.RawURLEncoding.EncodeToString(b[:])
	})
	return socksPassword
}
