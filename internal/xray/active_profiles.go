package xray

import (
	"sort"
	"time"
)

// ActiveProfileSnapshot contains exact, connection-based attribution. Supported
// is false for an older core; an empty supported snapshot means no live links.
// ObservedAt belongs to the originating node and is preserved across relays.
type ActiveProfileSnapshot struct {
	Supported  bool                `json:"supported"`
	ObservedAt int64               `json:"observedAt"`
	Clients    map[string][]string `json:"clients"`
}

type ActiveProfileTree map[string]ActiveProfileSnapshot

type capturedProfiles struct {
	Tree     ActiveProfileTree
	Received time.Time
}

const activeProfileTTL = 20 * time.Second

func cloneProfileSnapshot(s ActiveProfileSnapshot) ActiveProfileSnapshot {
	out := ActiveProfileSnapshot{Supported: s.Supported, ObservedAt: s.ObservedAt, Clients: map[string][]string{}}
	if s.Supported {
		for email, tags := range s.Clients {
			out.Clients[email] = append([]string{}, tags...)
		}
	}
	return out
}

func (p *Process) SetLocalActiveProfiles(users []OnlineUser, supported bool, now time.Time) {
	snapshot := ActiveProfileSnapshot{Supported: supported, ObservedAt: now.UnixMilli(), Clients: map[string][]string{}}
	if supported {
		for _, user := range users {
			if user.Email == "" {
				continue
			}
			seen := map[string]bool{}
			for _, tag := range user.InboundTags {
				if tag != "" && !seen[tag] {
					snapshot.Clients[user.Email] = append(snapshot.Clients[user.Email], tag)
					seen[tag] = true
				}
			}
			sort.Strings(snapshot.Clients[user.Email])
		}
	}
	p.onlineMu.Lock()
	p.localProfiles = capturedProfiles{Tree: ActiveProfileTree{"": snapshot}, Received: now}
	p.onlineMu.Unlock()
}

func (p *Process) GetLocalActiveProfiles(now time.Time) ActiveProfileSnapshot {
	p.onlineMu.RLock()
	defer p.onlineMu.RUnlock()
	if p.localProfiles.Received.IsZero() || now.Sub(p.localProfiles.Received) >= activeProfileTTL {
		return ActiveProfileSnapshot{Clients: map[string][]string{}}
	}
	return cloneProfileSnapshot(p.localProfiles.Tree[""])
}

func (p *Process) SetNodeActiveProfiles(nodeID int, tree ActiveProfileTree, now time.Time) {
	p.onlineMu.Lock()
	defer p.onlineMu.Unlock()
	if p.nodeProfiles == nil {
		p.nodeProfiles = map[int]capturedProfiles{}
	}
	if len(tree) == 0 {
		delete(p.nodeProfiles, nodeID)
		return
	}
	copyTree := ActiveProfileTree{}
	for guid, snapshot := range tree {
		if guid != "" {
			copyTree[guid] = cloneProfileSnapshot(snapshot)
		}
	}
	p.nodeProfiles[nodeID] = capturedProfiles{Tree: copyTree, Received: now}
}

// A duplicated path to a descendant must not resurrect its older connections.
// Compare source timestamps only within the same GUID, never between nodes.
func (p *Process) GetMergedActiveProfiles(now time.Time) ActiveProfileTree {
	p.onlineMu.RLock()
	defer p.onlineMu.RUnlock()
	out := ActiveProfileTree{}
	for _, captured := range p.nodeProfiles {
		if now.Sub(captured.Received) >= activeProfileTTL {
			continue
		}
		for guid, snapshot := range captured.Tree {
			old, exists := out[guid]
			if !exists || snapshot.ObservedAt > old.ObservedAt {
				out[guid] = cloneProfileSnapshot(snapshot)
			}
		}
	}
	return out
}
