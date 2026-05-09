// Package runtime abstracts the live xray engine that an inbound's
// configuration is shipped to. Two implementations exist: Local talks
// to the panel's own xray via gRPC (the original behaviour); Remote
// talks to another 3x-ui panel's HTTP API as a managed Node.
//
// InboundService picks a Runtime per-inbound based on Inbound.NodeID.
// The point of the abstraction is to keep `if node != nil` checks out
// of the service code as Phase 2/3 features (traffic sync, subscription
// per-node) build on top.
package runtime

import (
	"context"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// Runtime is the live-engine adapter for one inbound's worth of
// operations. Implementations must be safe for concurrent use — the
// service layer does not synchronise calls.
type Runtime interface {
	// Name identifies the adapter in logs ("local", "node:<name>").
	Name() string

	// AddInbound deploys an inbound to the engine. The Tag field on ib
	// is treated as the source of truth for identifying the inbound on
	// the remote side; Local ignores it.
	AddInbound(ctx context.Context, ib *model.Inbound) error

	// DelInbound removes the inbound identified by ib.Tag.
	DelInbound(ctx context.Context, ib *model.Inbound) error

	// UpdateInbound replaces the existing inbound with newIb. oldIb
	// carries the previous config so the adapter can compute a minimal
	// diff (Local: drop+add by tag; Remote: HTTP update by remote-id).
	UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error

	// AddUser hot-adds a client to the inbound identified by ib.Tag.
	// userMap matches the shape that xray.XrayAPI.AddUser already takes
	// — keys: email, id, password, auth, security, flow, cipher.
	AddUser(ctx context.Context, ib *model.Inbound, userMap map[string]any) error

	// RemoveUser hot-removes the client by email from ib's inbound.
	RemoveUser(ctx context.Context, ib *model.Inbound, email string) error

	// RestartXray asks the engine to fully restart. For Local this just
	// flips the SetToNeedRestart flag and lets the cron pick it up; for
	// Remote it issues an HTTP POST to /panel/api/server/restartXrayService.
	RestartXray(ctx context.Context) error

	// ResetClientTraffic zeros the up/down counters for one client on the
	// engine. Local: no-op — the central DB UPDATE that runs before this
	// call is sufficient, and xray's gRPC stats counter resets on the next
	// poll. Remote: HTTP POST so the next traffic sync doesn't pull the
	// pre-reset absolute back from the node.
	ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error

	// ResetInboundClientTraffics zeros every client of one inbound. Same
	// Local/Remote split as ResetClientTraffic.
	ResetInboundClientTraffics(ctx context.Context, ib *model.Inbound) error

	// ResetAllTraffics zeros every inbound counter on the engine. Used by
	// the panel-wide "reset all traffic" action; called once per affected
	// node so that nodes with no inbounds for the current panel are skipped.
	ResetAllTraffics(ctx context.Context) error
}
