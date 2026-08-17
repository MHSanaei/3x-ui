package quiccapture

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

const (
	captureTimeout = 5 * time.Second
	// Comfortably covers a bare Initial reply (typically ~1200-1500 bytes);
	// any coalesced Handshake-level packet after it is simply never reached
	// (ParseServerReply stops at the Initial packet's own declared Length),
	// so there's no need to size this for the full first flight.
	captureReadBufferSize = 4096
	// Real servers split their reply across two datagrams (an ACK-only
	// Initial packet, then a second one carrying CRYPTO) rather than
	// coalescing both into one -- observed live against
	// cloudflare-quic.com, unlike Google/Facebook/Cloudflare's main site.
	// One extra read, still inside the same overall timeout, is enough to
	// cover it without turning this into a general reassembly loop.
	maxCaptureReads = 3
)

// Result mirrors i1Generators.ts's I1GenResult shape (chain + a human-
// readable label) so the frontend can treat a live capture identically to a
// locally-generated profile once it comes back over the API.
type Result struct {
	Chain string
	Label string
}

// Capture builds a real QUICv1 Initial ClientHello for host, sends it to
// host:443 over UDP, and turns the real server's Initial reply into a CPS
// chain (see ParseServerReply). host is validated and resolved through the
// same netsafe guard REALITY's own live scan target uses
// (internal/web/service/reality_scan.go) -- rejected if it resolves to a
// private/loopback/link-local address, so an authenticated admin can't turn
// this into a UDP probe against the panel's own internal network.
func Capture(ctx context.Context, host string) (*Result, error) {
	normalized, err := netsafe.NormalizeHost(host)
	if err != nil {
		return nil, err
	}

	dcid := make([]byte, 8)
	scid := make([]byte, 8)
	helloRandom := make([]byte, helloRandLen)
	for _, b := range [][]byte{dcid, scid, helloRandom} {
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("quiccapture: generating probe randomness: %w", err)
		}
	}

	probe, err := buildClientProbe(normalized, dcid, scid, helloRandom)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, captureTimeout)
	defer cancel()

	conn, err := netsafe.SSRFGuardedDialContext(ctx, "udp", net.JoinHostPort(normalized, "443"))
	if err != nil {
		return nil, fmt.Errorf("quiccapture: connecting to %s:443: %w", normalized, err)
	}
	defer conn.Close()

	if _, err := conn.Write(probe.packet); err != nil {
		return nil, fmt.Errorf("quiccapture: sending probe to %s: %w", normalized, err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(deadline)
	}
	buf := make([]byte, captureReadBufferSize)
	var lastErr error
	for attempt := 0; attempt < maxCaptureReads; attempt++ {
		n, err := conn.Read(buf)
		if err != nil {
			if lastErr != nil {
				return nil, fmt.Errorf("quiccapture: no further reply from %s:443 after an ACK-only datagram: %w", normalized, err)
			}
			return nil, fmt.Errorf("quiccapture: no reply from %s:443 -- it may not speak QUIC, or blocked the probe: %w", normalized, err)
		}

		chain, err := ParseServerReply(buf[:n], dcid)
		if err == nil {
			return &Result{Chain: chain, Label: fmt.Sprintf("quic-live(%s)", normalized)}, nil
		}
		if !errors.Is(err, ErrNoCryptoFrameYet) {
			return nil, err
		}
		lastErr = err // this datagram was ACK-only; try reading one more
	}
	return nil, fmt.Errorf("quiccapture: %s never sent a CRYPTO frame within %d datagrams: %w", normalized, maxCaptureReads, lastErr)
}
