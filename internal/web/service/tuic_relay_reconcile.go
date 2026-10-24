package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// Renames and port swaps share one rollback boundary; independent inbounds do not.
func groupTuicRelayChanges(changes []xray.InboundReplacement) [][]xray.InboundReplacement {
	parent := make([]int, len(changes))
	for i := range parent {
		parent[i] = i
	}
	root := func(i int) int {
		for parent[i] != i {
			i = parent[i]
		}
		return i
	}
	ports, owners := make(map[int]int), make(map[int]int)
	tags := make(map[string]int)
	for i, change := range changes {
		for _, inbound := range []*xray.InboundConfig{change.Previous, change.Next} {
			if inbound == nil {
				continue
			}
			if j, ok := ports[inbound.Port]; ok {
				parent[root(i)] = root(j)
			}
			ports[inbound.Port] = i
			if j, ok := tags[inbound.Tag]; ok {
				parent[root(i)] = root(j)
			}
			tags[inbound.Tag] = i
			if inbound.TuicInboundID != 0 {
				if j, ok := owners[inbound.TuicInboundID]; ok {
					parent[root(i)] = root(j)
				}
				owners[inbound.TuicInboundID] = i
			}
		}
	}
	var groups [][]xray.InboundReplacement
	indices := make(map[int]int)
	for i, change := range changes {
		key := root(i)
		index, exists := indices[key]
		if !exists {
			index = len(groups)
			indices[key] = index
			groups = append(groups, nil)
		}
		groups[index] = append(groups[index], change)
	}
	return groups
}

func relayBindsOverlap(a, b xray.InboundConfig) bool {
	return a.Tag == b.Tag || (a.Port > 0 && a.Port == b.Port &&
		len(rawBindConflicts(&xray.Config{InboundConfigs: []xray.InboundConfig{a, b}})) > 0)
}

func tuicRequiresOrdinaryRemoval(old *xray.Config, changes []xray.InboundReplacement) bool {
	for _, change := range changes {
		if change.Next == nil {
			continue
		}
		for _, inbound := range old.InboundConfigs {
			if !inbound.TuicRelay && relayBindsOverlap(inbound, *change.Next) {
				return true
			}
		}
	}
	return false
}

// A failed relay retains its last verified config. Dependent tag/port handoffs
// and their routing/outbounds wait too; unrelated changes can still converge.
func configWithDeferredTuic(old, desired *xray.Config, failed []xray.InboundReplacement) *xray.Config {
	effective := *desired
	previous, pendingTags := make(map[string]xray.InboundConfig), make(map[string]bool)
	for _, change := range failed {
		if change.Previous != nil {
			previous[change.Previous.Tag] = *change.Previous
		}
		if change.Next != nil {
			pendingTags[change.Next.Tag] = true
		}
	}
	changedTags := false
	dependentTags := make(map[string]bool)
	for _, change := range failed {
		if change.Previous == nil {
			continue
		}
		old := change.Previous
		nextTag := ""
		for _, other := range failed {
			if next := other.Next; next != nil && ((old.TuicInboundID != 0 && old.TuicInboundID == next.TuicInboundID) || (old.TuicInboundID == 0 && old.Port == next.Port)) {
				nextTag = next.Tag
				break
			}
		}
		if nextTag != old.Tag {
			changedTags = true
			dependentTags[old.Tag] = true
			if nextTag != "" {
				dependentTags[nextTag] = true
			}
		}
	}
	effective.InboundConfigs = make([]xray.InboundConfig, 0, len(desired.InboundConfigs))
	seen := make(map[string]bool)
	for _, inbound := range desired.InboundConfigs {
		if kept, ok := previous[inbound.Tag]; ok {
			inbound = kept
		} else if inbound.TuicRelay && pendingTags[inbound.Tag] {
			continue
		} else if !inbound.TuicRelay {
			blocked := false
			for _, kept := range previous {
				if relayBindsOverlap(inbound, kept) {
					blocked = true
					break
				}
			}
			if blocked {
				index := slices.IndexFunc(old.InboundConfigs, func(candidate xray.InboundConfig) bool { return candidate.Tag == inbound.Tag })
				if index < 0 {
					continue
				}
				inbound = old.InboundConfigs[index]
				changedTags = true
			}
		}
		effective.InboundConfigs = append(effective.InboundConfigs, inbound)
		seen[inbound.Tag] = true
	}
	for _, inbound := range old.InboundConfigs {
		if _, ok := previous[inbound.Tag]; ok && !seen[inbound.Tag] {
			effective.InboundConfigs = append(effective.InboundConfigs, inbound)
		}
	}
	if changedTags && routingRulesChanged(old.RouterConfig, desired.RouterConfig, dependentTags, false) {
		effective.RouterConfig = old.RouterConfig
		effective.OutboundConfigs = old.OutboundConfigs
		kept := effective.InboundConfigs[:0]
		for _, inbound := range effective.InboundConfigs {
			if !inbound.TuicRelay && routingRulesChanged(old.RouterConfig, desired.RouterConfig, map[string]bool{inbound.Tag: true}, true) {
				index := slices.IndexFunc(old.InboundConfigs, func(candidate xray.InboundConfig) bool { return candidate.Tag == inbound.Tag })
				if index < 0 {
					continue
				}
				inbound = old.InboundConfigs[index]
			}
			kept = append(kept, inbound)
		}
		effective.InboundConfigs = kept
	}
	return &effective
}

func routingRulesChanged(old, desired []byte, tags map[string]bool, includeGlobal bool) bool {
	matchingRules := func(raw []byte) ([]byte, error) {
		var router struct {
			Rules []map[string]any `json:"rules"`
		}
		if err := json.Unmarshal(raw, &router); err != nil {
			return nil, err
		}
		var matched []map[string]any
		for _, rule := range router.Rules {
			inbounds, _ := rule["inboundTag"].([]any)
			if includeGlobal && len(inbounds) == 0 {
				matched = append(matched, rule)
				continue
			}
			for _, inbound := range inbounds {
				tag, _ := inbound.(string)
				if tags[tag] {
					matched = append(matched, rule)
					break
				}
			}
		}
		return json.Marshal(matched)
	}
	before, err := matchingRules(old)
	if err != nil {
		return true
	}
	after, err := matchingRules(desired)
	return err != nil || !bytes.Equal(before, after)
}

type stagedTuicRelay struct {
	transaction *tuicRelayTransaction
	changes     []xray.InboundReplacement
	changed     bool
}

type tuicHotReconcile struct {
	effective *xray.Config
	staged    []*stagedTuicRelay
}

func (r *tuicHotReconcile) finish() {
	for _, group := range r.staged {
		group.transaction.finish()
	}
}

func (r *tuicHotReconcile) rollback(api *xray.XrayAPI) error {
	var failures error
	for _, group := range r.staged {
		if group.changed {
			failures = errors.Join(failures, group.transaction.rollback(api, group.changes))
			group.changed = false
		}
	}
	return failures
}

func reconcileTuicHotConfig(api *xray.XrayAPI, old, desired *xray.Config, changes []xray.InboundReplacement) (*tuicHotReconcile, error) {
	result := &tuicHotReconcile{}
	var failures error
	var deferred []xray.InboundReplacement
	for _, group := range groupTuicRelayChanges(changes) {
		transaction, err := stageTuicRelayGroup(api, group)
		if transaction != nil {
			result.staged = append(result.staged, &stagedTuicRelay{transaction: transaction, changes: group, changed: err == nil})
		}
		if err != nil {
			failures = errors.Join(failures, err)
			deferred = append(deferred, group...)
		}
	}
	effective := configWithDeferredTuic(old, desired, deferred)
	if !bytes.Equal(effective.RouterConfig, desired.RouterConfig) {
		for _, group := range result.staged {
			if !group.changed {
				continue
			}
			tags := make(map[string]bool)
			for _, change := range group.changes {
				for _, inbound := range []*xray.InboundConfig{change.Previous, change.Next} {
					if inbound != nil {
						tags[inbound.Tag] = true
					}
				}
			}
			if routingRulesChanged(old.RouterConfig, desired.RouterConfig, tags, true) {
				failures = errors.Join(failures, errors.New("TUIC relay update waits for dependent routing"), group.transaction.rollback(api, group.changes))
				group.changed = false
				deferred = append(deferred, group.changes...)
			}
		}
	}
	result.effective = configWithDeferredTuic(old, desired, deferred)
	// Repair unchanged relays too: a failed rollback or cold-start check does
	// not necessarily change the desired JSON on the next reconciliation.
	unchanged := *old
	unchanged.InboundConfigs = nil
	for _, inbound := range old.InboundConfigs {
		changed := false
		for _, change := range changes {
			for _, candidate := range []*xray.InboundConfig{change.Previous, change.Next} {
				if candidate != nil && (candidate.Port == inbound.Port || candidate.Tag == inbound.Tag) {
					changed = true
				}
			}
		}
		if !changed {
			unchanged.InboundConfigs = append(unchanged.InboundConfigs, inbound)
		}
	}
	if err := applyTuicRelayReplacements(api, unavailableTuicRepairs(&unchanged)); err != nil {
		failures = errors.Join(failures, fmt.Errorf("repair TUIC relay: %w", err))
	}
	return result, failures
}
