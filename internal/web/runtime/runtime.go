package runtime

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// ExternalLinkSync is the link set one node's clients receive, already resolved
// by the master: a node holds no groups, so scopes are flattened before the
// push and the node only has to materialize rows.
type ExternalLinkSync struct {
	Links   []ExternalLinkSyncLink   `json:"links"`
	Clients []ExternalLinkSyncClient `json:"clients"`
}

// ExternalLinkSyncLink is one library row the node stores, keyed by kind+value.
type ExternalLinkSyncLink struct {
	Kind       string            `json:"kind"`
	Value      string            `json:"value"`
	Remark     string            `json:"remark,omitempty"`
	NamePrefix string            `json:"namePrefix,omitempty"`
	Enable     bool              `json:"enable"`
	ExpiryTime int64             `json:"expiryTime,omitempty"`
	SortIndex  int               `json:"sortIndex"`
	UserAgent  string            `json:"userAgent,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	CacheTtl   int               `json:"cacheTtl,omitempty"`
}

// ExternalLinkSyncClient is one client's effective links in served order.
type ExternalLinkSyncClient struct {
	Email string                       `json:"email"`
	Links []ExternalLinkSyncAssignment `json:"links"`
}

// ExternalLinkSyncAssignment is a scope's resolved overrides over a library
// row; expiry 0 means never, matching what the master would have served.
type ExternalLinkSyncAssignment struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Enable     bool   `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
	Remark     string `json:"remark,omitempty"`
	NamePrefix string `json:"namePrefix,omitempty"`
	SortIndex  int    `json:"sortIndex"`
}

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

	// DeleteClient removes the client identified by email entirely from the
	// runtime's own store: on Remote it hits the node's full-delete endpoint
	// (record, attachments, traffic), unlike DeleteUser which only detaches
	// from one inbound and leaves the node's client record behind. Local has
	// no client store of its own, so it is a no-op there.
	DeleteClient(ctx context.Context, email string) error

	// PushExternalLinks hands a Remote the panel's resolved link set so a
	// subscription served off that node carries the same library. Local is a
	// no-op: the panel's own store is already the source.
	PushExternalLinks(ctx context.Context, sync ExternalLinkSync) error

	RestartXray(ctx context.Context) error

	ResetClientTraffic(ctx context.Context, ib *model.Inbound, email string) error
	ResetInboundTraffic(ctx context.Context, ib *model.Inbound) error
	ResetAllTraffics(ctx context.Context) error
}
