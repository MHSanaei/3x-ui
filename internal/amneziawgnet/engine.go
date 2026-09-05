package amneziawgnet

import (
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

// Engine is the common backend interface for AmneziaWG device operations.
type Engine interface {
	Name() string
	Ensure(d Desired) error
	Remove(inboundID int)
	StopAll()
	HasRunning() bool
	Diagnose(inboundID int, peers []amneziawg.Peer) Diagnostics
}
