package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var errOverrideTargetUnknown = errors.New("override target is not an outbound in the running config")

// balancerOverrides keeps the overrides set through the panel, because every
// routing reload rebuilds the core's balancers and drops them.
var balancerOverrides balancerOverrideSet

// balancerOverrideSet belongs to one core process: a restart clears overrides,
// so records made for an earlier process are never re-applied.
type balancerOverrideSet struct {
	mu      sync.Mutex
	process *xray.Process
	targets map[string]string
}

// set forces balancer tag to target in the running core and records it; an
// empty target clears the override.
func (s *balancerOverrideSet) set(process *xray.Process, api *xray.XrayAPI, tag, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	coreTarget := ""
	if target != "" {
		resolved, err := resolveOverrideTarget(process.GetConfig(), target)
		if err != nil {
			return err
		}
		coreTarget = resolved
	}
	if err := api.SetBalancerTarget(tag, coreTarget); err != nil {
		return err
	}
	if s.process != process {
		s.process = process
		s.targets = map[string]string{}
	}
	if target == "" {
		delete(s.targets, tag)
	} else {
		s.targets[tag] = target
	}
	return nil
}

// reapply restores the recorded overrides once cfg is live, dropping any whose
// balancer or target cfg no longer has.
func (s *balancerOverrideSet) reapply(process *xray.Process, api *xray.XrayAPI, cfg *xray.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != process {
		return
	}
	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		_ = json.Unmarshal(cfg.RouterConfig, &routing)
	}
	for tag, target := range s.targets {
		if !routingTagIsBalancer(routing, tag) {
			delete(s.targets, tag)
			continue
		}
		coreTarget, err := resolveOverrideTarget(cfg, target)
		if err != nil {
			logger.Warning("balancer [", tag, "] override dropped:", err)
			delete(s.targets, tag)
		}
		if err := api.SetBalancerTarget(tag, coreTarget); err != nil {
			logger.Warning("re-apply override of balancer [", tag, "] failed:", err)
		}
	}
}

// resolveOverrideTarget maps target to the outbound tag the core routes to: a
// balancer becomes the _bl_ loopback outbound whose rule feeds it.
func resolveOverrideTarget(cfg *xray.Config, target string) (string, error) {
	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			return "", err
		}
	}
	if loopback := balancerLoopbackTag(routing, target); loopback != "" && outboundTagExists(cfg.OutboundConfigs, loopback) {
		return loopback, nil
	}
	if outboundTagExists(cfg.OutboundConfigs, target) {
		return target, nil
	}
	return "", fmt.Errorf("%w: %q", errOverrideTargetUnknown, target)
}

func balancerLoopbackTag(routing map[string]any, balancerTag string) string {
	rules, _ := routing["rules"].([]any)
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok || rule["balancerTag"] != balancerTag {
			continue
		}
		inboundTags, _ := rule["inboundTag"].([]any)
		if len(inboundTags) == 0 {
			continue
		}
		if tag, ok := inboundTags[0].(string); ok && strings.HasPrefix(tag, "_bl_") {
			return tag
		}
	}
	return ""
}
