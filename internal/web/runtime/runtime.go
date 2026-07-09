package runtime

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type Runtime interface {
	Name() string

	AddInbound(ctx context.Context, ib *model.Inbound) error
	DelInbound(ctx context.Context, ib *model.Inbound) error
	UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error

	AddUser(ctx context.Context, ib *model.Inbound, userMap map[string]any) error
	RemoveUser(ctx context.Context, ib *model.Inbound, email string) error

	// Per-client operations that route through the node's clients API on
	// Remote (instead of pushing the whole inbound) so the node applies
	// per-user xray API calls without a DelInbound+AddInbound cycle.
	UpdateUser(ctx context.Context, ib *model.Inbound, email string, payload model.Client) error
	DeleteUser(ctx context.Context, ib *model.Inbound, email string) error
	AddClient(ctx context.Context, ib *model.Inbound, client model.Client) error

	RestartXray(ctx context.Context) error

	ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error
	ResetInboundTraffic(ctx context.Context, ib *model.Inbound) error
	ResetAllTraffics(ctx context.Context) error

	// ReconcileInbound pushes ib only when its wire payload differs from the last
	// successful push, or when the node no longer reports the tag (existsOnNode
	// false) — a node that dropped/restarted must still be re-seeded. Returns
	// whether a push actually happened. This turns a full-fleet reconcile from
	// "send every inbound's full settings" into "send only what changed".
	ReconcileInbound(ctx context.Context, ib *model.Inbound, existsOnNode bool) (bool, error)
}