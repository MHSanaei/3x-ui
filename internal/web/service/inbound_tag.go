package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// InboundTagChangeCheck is the pre-flight report for an inbound ID (tag)
// change: whether the target is usable and what a rename would touch, so the
// panel can show the impact before committing.
type InboundTagChangeCheck struct {
	Unchanged bool   `json:"unchanged" example:"false"`
	Valid     bool   `json:"valid" example:"true"`
	Reason    string `json:"reason"`
	// RoutingRefs is how many routing rules on this panel reference the
	// current tag; a rename rewrites them all.
	RoutingRefs int `json:"routingRefs" example:"2"`
	// NodeTagConflict reports a same-name inbound on the hosting node that the
	// new tag would collide with once the prefix is stripped.
	NodeTagConflict bool `json:"nodeTagConflict" example:"false"`
}

// CheckTagChange validates a prospective tag change for an inbound without
// touching anything. The caller supplies the target tag exactly as the panel
// would store it (n<nodeID>-prefixed for node-hosted inbounds).
func (s *InboundService) CheckTagChange(userID, inboundID int, newTag string) (*InboundTagChangeCheck, error) {
	check := &InboundTagChangeCheck{}
	inbound, err := s.GetInboundDetail(inboundID)
	if err != nil {
		return nil, err
	}
	if inbound.UserId != userID {
		return nil, fmt.Errorf("inbound %d not found", inboundID)
	}
	newTag = strings.TrimSpace(newTag)
	if newTag == "" {
		check.Reason = "blank"
		return check, nil
	}
	if len(newTag) > 128 {
		check.Reason = "too-long"
		return check, nil
	}

	refs, err := s.countRoutingRuleRefs(inbound.Tag)
	if err != nil {
		return nil, err
	}
	check.RoutingRefs = refs

	if newTag == inbound.Tag {
		check.Unchanged = true
		check.Valid = true
		return check, nil
	}

	taken, err := s.tagExists(newTag, inboundID)
	if err != nil {
		return nil, err
	}
	if taken {
		check.Reason = "in-use"
		return check, nil
	}

	if inbound.NodeID != nil {
		nodeID := *inbound.NodeID
		// selfBare is the name this inbound already carries on the node,
		// which the node's tag list always reports; exclude it so a rename
		// that keeps the bare part is not a self-conflict.
		selfBare := inbound.Tag
		if stripped, ok := strings.CutPrefix(inbound.Tag, nodeTagPrefix(&nodeID)); ok {
			selfBare = stripped
		}
		conflict, err := s.nodeTagConflict(nodeID, newTag, selfBare)
		if err != nil {
			// Fail safe: an unreachable node cannot prove the tag is free.
			check.Reason = "node-unreachable"
			return check, nil
		}
		if conflict {
			check.NodeTagConflict = true
			check.Reason = "node-tag-conflict"
			return check, nil
		}
	}

	check.Valid = true
	return check, nil
}

// countRoutingRuleRefs counts the panel's routing rules referencing tag, so
// the rename preview can say how much will be rewritten.
func (s *InboundService) countRoutingRuleRefs(tag string) (int, error) {
	if tag == "" {
		return 0, nil
	}
	template, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil {
		return 0, err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(UnwrapXrayTemplateConfig(template)), &cfg); err != nil {
		return 0, err
	}
	refs := 0
	for _, rule := range routingRulesFromCfg(cfg) {
		for _, t := range readInboundTags(rule["inboundTag"]) {
			if t == tag {
				refs++
				break
			}
		}
	}
	return refs, nil
}

// nodeTagConflict checks the hosting node's own tags for a collision with the
// new tag once the central n<id>- prefix is stripped, i.e. the name Xray on
// the node would carry. The node's remote ID table is only rebuilt on demand,
// so a single remote call against the node's list endpoint suffices.
func (s *InboundService) nodeTagConflict(nodeID int, newTag, selfBare string) (bool, error) {
	nodeService := &NodeService{}
	node, err := nodeService.GetById(nodeID)
	if err != nil {
		return false, err
	}
	if err := nodeService.normalize(node); err != nil {
		return false, err
	}
	prefix := nodeTagPrefix(&nodeID)
	bare := newTag
	if stripped, ok := strings.CutPrefix(newTag, prefix); ok {
		bare = stripped
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var (
		conflict  bool
		remoteErr error
	)
	nodeService.withOutboundBridge(nodeID, node.OutboundTag, func(proxyURL string) {
		remote := runtime.NewRemote(node, staticEgressResolver(proxyURL))
		tags, err := remote.ListRemoteTags(ctx)
		if err != nil {
			remoteErr = err
			return
		}
		for _, t := range tags {
			if t != selfBare && (t == newTag || t == bare) {
				conflict = true
				return
			}
		}
	})
	if remoteErr != nil {
		return false, remoteErr
	}
	return conflict, nil
}

// ChangeInboundTag renames an inbound's identifier (tag) by driving the
// proven UpdateInbound rename path, so routing rewrites, node re-push and the
// xray restart all happen exactly as they do for a form edit. It first runs
// the same pre-flight check the UI shows, and refuses an unchanged target.
func (s *InboundService) ChangeInboundTag(userID, inboundID int, newTag string) (bool, error) {
	check, err := s.CheckTagChange(userID, inboundID, newTag)
	if err != nil {
		return false, err
	}
	if !check.Valid {
		return false, common.NewErrorf("inbound id cannot be changed to %q (%s)", newTag, check.Reason)
	}
	if check.Unchanged {
		return false, nil
	}
	full, err := s.GetInboundDetail(inboundID)
	if err != nil {
		return false, err
	}
	if full.UserId != userID {
		return false, common.NewErrorf("inbound %d not found", inboundID)
	}
	full.Tag = strings.TrimSpace(newTag)
	_, needRestart, err := s.UpdateInbound(full)
	return needRestart, err
}
