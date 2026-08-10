package service

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg/quiccapture"
)

const quicCaptureTimeout = 8 * time.Second

// QuicCaptureResult is a live QUIC capture's CPS chain plus a human-readable
// label, ready to drop straight into an AmneziaWG inbound's I1-I5 field --
// the same shape the frontend's own locally-generated i1Generators.ts
// profiles return.
type QuicCaptureResult struct {
	Chain string `json:"chain" example:"<b 0x...><r 24><b 0x...><r 32><b 0x...><r 16>"`
	Label string `json:"label" example:"quic-live(example.com)"`
}

// CaptureAwgQuic sends a real QUICv1 Initial ClientHello to host:443 and
// turns the server's real Initial reply into a CPS chain (see
// internal/amneziawg/quiccapture). host is validated and SSRF-guarded the
// same way ScanRealityTarget's own live probe target is.
func (s *ServerService) CaptureAwgQuic(host string) (*QuicCaptureResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), quicCaptureTimeout)
	defer cancel()

	res, err := quiccapture.Capture(ctx, host)
	if err != nil {
		return nil, err
	}
	return &QuicCaptureResult{Chain: res.Chain, Label: res.Label}, nil
}
