package service

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// bindConflict names two generated inbounds whose listens cannot coexist.
type bindConflict struct {
	tagA    string
	tagB    string
	listen  string
	listenB string
	port    int
	shared  transportBits
}

func (c bindConflict) String() string {
	return fmt.Sprintf("inbounds %q and %q both bind %s:%d (%s)",
		c.tagA, c.tagB, displayListen(c.listen), c.port, transportTagSuffix(c.shared))
}

// bindConflicts reports inbounds of newCfg whose sockets collide. A collision the
// running config already serves is excused: it is demonstration, not a guess.
func bindConflicts(newCfg, runningCfg *xray.Config) []bindConflict {
	conflicts := rawBindConflicts(newCfg)
	if len(conflicts) == 0 {
		return nil
	}
	excused := runningBindPairs(runningCfg)
	if len(excused) == 0 {
		return conflicts
	}
	kept := make([]bindConflict, 0, len(conflicts))
	for _, c := range conflicts {
		if _, ok := excused[bindPairKey(c)]; !ok {
			kept = append(kept, c)
		}
	}
	return kept
}

// rawBindConflicts groups inbounds by port first: only a port two inbounds share
// is worth parsing transports for, which keeps the probe free on clean configs.
func rawBindConflicts(cfg *xray.Config) []bindConflict {
	if cfg == nil {
		return nil
	}
	byPort := make(map[int][]*xray.InboundConfig, len(cfg.InboundConfigs))
	for i := range cfg.InboundConfigs {
		ib := &cfg.InboundConfigs[i]
		if ib.Port > 0 {
			byPort[ib.Port] = append(byPort[ib.Port], ib)
		}
	}

	var conflicts []bindConflict
	for port, group := range byPort {
		for i := range group {
			for j := i + 1; j < len(group); j++ {
				left, right := group[i], group[j]
				listenLeft, listenRight := configListen(left.Listen), configListen(right.Listen)
				bindLeft := bindAddr{listen: listenLeft, v6only: streamV6Only(string(left.StreamSettings))}
				bindRight := bindAddr{listen: listenRight, v6only: streamV6Only(string(right.StreamSettings))}
				if !listenOverlaps(bindLeft, bindRight) {
					continue
				}
				// One port carrying tcp on one inbound and udp on another is a
				// supported deployment (vless/tcp + hysteria2/udp), never a clash.
				shared := configTransports(left) & configTransports(right)
				if shared == 0 {
					continue
				}
				conflicts = append(conflicts, bindConflict{
					tagA:    left.Tag,
					tagB:    right.Tag,
					listen:  listenLeft,
					listenB: listenRight,
					port:    port,
					shared:  shared,
				})
			}
		}
	}
	// Port grouping iterates a map: order the report so the first conflict the
	// caller shows is the same on every restart.
	slices.SortFunc(conflicts, func(a, b bindConflict) int {
		return cmp.Or(cmp.Compare(a.port, b.port), cmp.Compare(a.tagA, b.tagA), cmp.Compare(a.tagB, b.tagB))
	})
	return conflicts
}

// runningBindPairs is what the core is demonstrably binding right now, keyed the
// way a new config's conflicts are, so only the identical one is excused.
func runningBindPairs(cfg *xray.Config) map[string]struct{} {
	conflicts := rawBindConflicts(cfg)
	if len(conflicts) == 0 {
		return nil
	}
	pairs := make(map[string]struct{}, len(conflicts))
	for _, c := range conflicts {
		pairs[bindPairKey(c)] = struct{}{}
	}
	return pairs
}

// bindPairKey is the socket set an excuse was granted for: the two listens, the
// port and the shared transports, never the tags, which the generator reorders.
func bindPairKey(c bindConflict) string {
	left, right := c.listen, c.listenB
	if left > right {
		left, right = right, left
	}
	return fmt.Sprintf("%d\x00%s\x00%s\x00%d", c.port, left, right, c.shared)
}

func displayListen(listen string) string {
	if isAnyListen(listen) {
		return "*"
	}
	return listen
}

// configListen decodes a generated inbound's listen field: absent or empty means
// every address, which is what the core does with an empty listen too.
func configListen(raw json_util.RawMessage) string {
	var listen string
	if len(raw) == 0 || json.Unmarshal(raw, &listen) != nil {
		return ""
	}
	return listen
}

// configTransports reads a generated inbound's transports through the same rule
// the save-time guards use, so the two cannot drift apart.
func configTransports(ib *xray.InboundConfig) transportBits {
	// "socks" is the panel's own bridge shape (injectAmneziawgnetSocks and
	// friends): its udp flag lives in settings like a mixed inbound's.
	if ib.Protocol == "socks" {
		return inboundTransports(model.Mixed, string(ib.StreamSettings), string(ib.Settings))
	}
	return inboundTransports(model.Protocol(ib.Protocol), string(ib.StreamSettings), string(ib.Settings))
}
