package service

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// NodeView is the browser/API read contract for nodes. Credentials are
// write-only: responses expose only whether a node has a token configured.
type NodeView struct {
	Id                  int      `json:"id"`
	Name                string   `json:"name"`
	Remark              string   `json:"remark"`
	Scheme              string   `json:"scheme"`
	Address             string   `json:"address"`
	Port                int      `json:"port"`
	BasePath            string   `json:"basePath"`
	HasApiToken         bool     `json:"hasApiToken"`
	Enable              bool     `json:"enable"`
	AllowPrivateAddress bool     `json:"allowPrivateAddress"`
	TlsVerifyMode       string   `json:"tlsVerifyMode"`
	PinnedCertSha256    string   `json:"pinnedCertSha256"`
	InboundSyncMode     string   `json:"inboundSyncMode"`
	InboundTags         []string `json:"inboundTags"`
	OutboundTag         string   `json:"outboundTag"`
	Guid                string   `json:"guid"`
	Status              string   `json:"status"`
	LastHeartbeat       int64    `json:"lastHeartbeat"`
	LatencyMs           int      `json:"latencyMs"`
	XrayVersion         string   `json:"xrayVersion"`
	PanelVersion        string   `json:"panelVersion"`
	CpuPct              float64  `json:"cpuPct"`
	MemPct              float64  `json:"memPct"`
	UptimeSecs          uint64   `json:"uptimeSecs"`
	NetUp               uint64   `json:"netUp"`
	NetDown             uint64   `json:"netDown"`
	LastError           string   `json:"lastError"`
	XrayState           string   `json:"xrayState"`
	XrayError           string   `json:"xrayError"`
	ConfigDirty         bool     `json:"configDirty"`
	ConfigDirtyAt       int64    `json:"configDirtyAt"`
	InboundCount        int      `json:"inboundCount"`
	ClientCount         int      `json:"clientCount"`
	OnlineCount         int      `json:"onlineCount"`
	ActiveCount         int      `json:"activeCount"`
	DisabledCount       int      `json:"disabledCount"`
	DepletedCount       int      `json:"depletedCount"`
	ParentGuid          string   `json:"parentGuid,omitempty"`
	Transitive          bool     `json:"transitive,omitempty"`
	CreatedAt           int64    `json:"createdAt"`
	UpdatedAt           int64    `json:"updatedAt"`
}

func toNodeView(n *model.Node) *NodeView {
	if n == nil {
		return nil
	}
	return &NodeView{
		Id:                  n.Id,
		Name:                n.Name,
		Remark:              n.Remark,
		Scheme:              n.Scheme,
		Address:             n.Address,
		Port:                n.Port,
		BasePath:            n.BasePath,
		HasApiToken:         n.ApiToken != "",
		Enable:              n.Enable,
		AllowPrivateAddress: n.AllowPrivateAddress,
		TlsVerifyMode:       n.TlsVerifyMode,
		PinnedCertSha256:    n.PinnedCertSha256,
		InboundSyncMode:     n.InboundSyncMode,
		InboundTags:         n.InboundTags,
		OutboundTag:         n.OutboundTag,
		Guid:                n.Guid,
		Status:              n.Status,
		LastHeartbeat:       n.LastHeartbeat,
		LatencyMs:           n.LatencyMs,
		XrayVersion:         n.XrayVersion,
		PanelVersion:        n.PanelVersion,
		CpuPct:              n.CpuPct,
		MemPct:              n.MemPct,
		UptimeSecs:          n.UptimeSecs,
		NetUp:               n.NetUp,
		NetDown:             n.NetDown,
		LastError:           n.LastError,
		XrayState:           n.XrayState,
		XrayError:           n.XrayError,
		ConfigDirty:         n.ConfigDirty,
		ConfigDirtyAt:       n.ConfigDirtyAt,
		InboundCount:        n.InboundCount,
		ClientCount:         n.ClientCount,
		OnlineCount:         n.OnlineCount,
		ActiveCount:         n.ActiveCount,
		DisabledCount:       n.DisabledCount,
		DepletedCount:       n.DepletedCount,
		ParentGuid:          n.ParentGuid,
		Transitive:          n.Transitive,
		CreatedAt:           n.CreatedAt,
		UpdatedAt:           n.UpdatedAt,
	}
}

func toNodeViews(nodes []*model.Node) []*NodeView {
	views := make([]*NodeView, 0, len(nodes))
	for _, node := range nodes {
		views = append(views, toNodeView(node))
	}
	return views
}

// NodeMutationRequest is the node write/probe contract. ApiToken is accepted
// only as input. On update, nil means keep the stored token; replacement and
// clearing are explicit and mutually exclusive.
type NodeMutationRequest struct {
	Id                  int      `json:"id" form:"id"`
	Name                string   `json:"name" form:"name" validate:"required"`
	Remark              string   `json:"remark" form:"remark"`
	Scheme              string   `json:"scheme" form:"scheme" validate:"omitempty,oneof=http https"`
	Address             string   `json:"address" form:"address" validate:"required"`
	Port                int      `json:"port" form:"port" validate:"gte=1,lte=65535"`
	BasePath            string   `json:"basePath" form:"basePath"`
	ApiToken            *string  `json:"apiToken,omitempty" form:"apiToken"`
	ClearApiToken       bool     `json:"clearApiToken,omitempty" form:"clearApiToken"`
	Enable              bool     `json:"enable" form:"enable"`
	AllowPrivateAddress bool     `json:"allowPrivateAddress" form:"allowPrivateAddress"`
	TlsVerifyMode       string   `json:"tlsVerifyMode" form:"tlsVerifyMode" validate:"omitempty,oneof=verify skip pin mtls"`
	PinnedCertSha256    string   `json:"pinnedCertSha256" form:"pinnedCertSha256"`
	InboundSyncMode     string   `json:"inboundSyncMode" form:"inboundSyncMode" validate:"omitempty,oneof=all selected"`
	InboundTags         []string `json:"inboundTags" form:"inboundTags"`
	OutboundTag         string   `json:"outboundTag" form:"outboundTag"`
}

func (r *NodeMutationRequest) validateCredentials(create bool) error {
	if r == nil {
		return common.NewError("node request is required")
	}
	if r.ApiToken != nil && r.ClearApiToken {
		return common.NewError("apiToken and clearApiToken are mutually exclusive")
	}
	if r.ApiToken != nil {
		*r.ApiToken = strings.TrimSpace(*r.ApiToken)
		if *r.ApiToken == "" {
			return common.NewError("apiToken must be omitted to keep it or cleared explicitly")
		}
	}
	if create {
		if r.ClearApiToken {
			return common.NewError("credentials cannot be cleared while creating a node")
		}
		if r.ApiToken == nil && r.TlsVerifyMode != "mtls" {
			return common.NewError("apiToken is required unless mtls is enabled")
		}
	}
	if r.ClearApiToken && r.Enable && r.TlsVerifyMode != "mtls" {
		return common.NewError("disable the node or enable mtls before clearing its apiToken")
	}
	return nil
}

func (r *NodeMutationRequest) toNode() *model.Node {
	n := &model.Node{
		Id:                  r.Id,
		Name:                r.Name,
		Remark:              r.Remark,
		Scheme:              r.Scheme,
		Address:             r.Address,
		Port:                r.Port,
		BasePath:            r.BasePath,
		Enable:              r.Enable,
		AllowPrivateAddress: r.AllowPrivateAddress,
		TlsVerifyMode:       r.TlsVerifyMode,
		PinnedCertSha256:    r.PinnedCertSha256,
		InboundSyncMode:     r.InboundSyncMode,
		InboundTags:         r.InboundTags,
		OutboundTag:         r.OutboundTag,
	}
	if r.ApiToken != nil {
		n.ApiToken = *r.ApiToken
	}
	return n
}
