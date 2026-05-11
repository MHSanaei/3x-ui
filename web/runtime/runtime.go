package runtime

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

type Runtime interface {
	Name() string

	AddInbound(ctx context.Context, ib *model.Inbound) error
	DelInbound(ctx context.Context, ib *model.Inbound) error
	UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error

	AddUser(ctx context.Context, ib *model.Inbound, userMap map[string]any) error
	RemoveUser(ctx context.Context, ib *model.Inbound, email string) error

	RestartXray(ctx context.Context) error

	ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error
	ResetInboundClientTraffics(ctx context.Context, ib *model.Inbound) error
	ResetAllTraffics(ctx context.Context) error
}
