package service

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// NodeView is the browser/API read contract for nodes. Credentials are
// write-only: responses expose only whether a node has a token configured.
type NodeView struct {
	Id                  int      `json:"id" example:"1"`
	Name                string   `json:"name" example:"edge-1"`
	Remark              string   `json:"remark" example:"Primary edge"`
	Scheme              string   `json:"scheme" example:"https"`
	Address             string   `json:"address" example:"node.example.com"`
	Port                int      `json:"port" example:"2053"`
	BasePath            string   `json:"basePath" example:"/"`
	HasApiToken         bool     `json:"hasApiToken" example:"true"`
	Enable              bool     `json:"enable" example:"true"`
	AllowPrivateAddress bool     `json:"allowPrivateAddress" example:"false"`
	TlsVerifyMode       string   `json:"tlsVerifyMode" example:"verify"`
	PinnedCertSha256    string   `json:"pinnedCertSha256" example:""`
	InboundSyncMode     string   `json:"inboundSyncMode" example:"all"`
	InboundTags         []string `json:"inboundTags" example:"[\"in-443-tcp\"]"`
	OutboundTag         string   `json:"outboundTag" example:"direct"`
	Guid                string   `json:"guid" example:"node-guid"`
	Status              string   `json:"status" example:"online"`
	LastHeartbeat       int64    `json:"lastHeartbeat" example:"1700000000"`
	LatencyMs           int      `json:"latencyMs" example:"42"`
	XrayVersion         string   `json:"xrayVersion" example:"25.10.31"`
	PanelVersion        string   `json:"panelVersion" example:"v3.x.x"`
	CpuPct              float64  `json:"cpuPct" example:"12.5"`
	MemPct              float64  `json:"memPct" example:"45.2"`
	UptimeSecs          uint64   `json:"uptimeSecs" example:"86400"`
	NetUp               uint64   `json:"netUp" example:"2097152"`
	NetDown             uint64   `json:"netDown" example:"1048576"`
	LastError           string   `json:"lastError" example:""`
	XrayState           string   `json:"xrayState" example:"running"`
	XrayError           string   `json:"xrayError" example:""`
	ConfigDirty         bool     `json:"configDirty" example:"false"`
	ConfigDirtyAt       int64    `json:"configDirtyAt" example:"0"`
	InboundCount        int      `json:"inboundCount" example:"3"`
	ClientCount         int      `json:"clientCount" example:"25"`
	OnlineCount         int      `json:"onlineCount" example:"5"`
	ActiveCount         int      `json:"activeCount" example:"20"`
	DisabledCount       int      `json:"disabledCount" example:"2"`
	DepletedCount       int      `json:"depletedCount" example:"1"`
	ParentGuid          string   `json:"parentGuid,omitempty" example:""`
	Transitive          bool     `json:"transitive,omitempty" example:"false"`
	CreatedAt           int64    `json:"createdAt" example:"1700000000"`
	UpdatedAt           int64    `json:"updatedAt" example:"1700003600"`
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
			if create {
				return common.NewError("apiToken is required unless mtls is enabled")
			}
			r.ApiToken = nil
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
