package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type snapshotClientIdentity struct {
	InboundID int
	Email     string
}

type NodeSnapshotAttachment struct {
	InboundID int    `json:"inboundId" example:"1"`
	Email     string `json:"email" example:"user1"`
}

type NodeSnapshotStructure struct {
	Inbounds    []*model.Inbound         `json:"inbounds"`
	Attachments []NodeSnapshotAttachment `json:"attachments"`
}

type NodeInboundTraffic struct {
	ID   int   `json:"id" example:"1"`
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type NodeClientTraffic struct {
	ID           int    `json:"id" example:"14825"`
	InboundID    int    `json:"inboundId" example:"1"`
	Email        string `json:"email" example:"user1"`
	Enable       bool   `json:"enable" example:"true"`
	Up           int64  `json:"up" example:"1048576"`
	Down         int64  `json:"down" example:"2097152"`
	Total        int64  `json:"total" example:"10737418240"`
	ExpiryTime   int64  `json:"expiryTime" example:"1735689600000"`
	Reset        int    `json:"reset"`
	ResetDay     int    `json:"resetDay"`
	ResetWeekday int    `json:"resetWeekday"`
	ResetMax     int    `json:"resetMax"`
	ResetCount   int    `json:"resetCount"`
	LastOnline   int64  `json:"lastOnline" example:"1735680000000"`
}

func (c NodeClientTraffic) traffic() xray.ClientTraffic {
	return xray.ClientTraffic{Id: c.ID, InboundId: c.InboundID, Email: c.Email, Enable: c.Enable, Up: c.Up, Down: c.Down, Total: c.Total, ExpiryTime: c.ExpiryTime, Reset: c.Reset, ResetDay: c.ResetDay, ResetWeekday: c.ResetWeekday, ResetMax: c.ResetMax, ResetCount: c.ResetCount, LastOnline: c.LastOnline}
}

// NodeTrafficSnapshotV1 carries complete counters every tick; only unchanged structure is omitted.
type NodeTrafficSnapshotV1 struct {
	Version           int                    `json:"version" example:"1"`
	Revision          string                 `json:"revision" example:"0000000000000000000000000000000000000000000000000000000000000001"`
	Structure         *NodeSnapshotStructure `json:"structure,omitempty"`
	InboundTraffic    []NodeInboundTraffic   `json:"inboundTraffic"`
	Clients           []NodeClientTraffic    `json:"clients"`
	OnlineEmails      []string               `json:"onlineEmails"`
	OnlineTree        map[string][]string    `json:"onlineTree"`
	ActiveInboundTree map[string][]string    `json:"activeInboundTree"`
	LastOnlineMap     map[string]int64       `json:"lastOnlineMap"`
}

func (r *Remote) FetchTrafficSnapshot(ctx context.Context) (*TrafficSnapshot, error) {
	select {
	case r.snapshotGate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-r.snapshotGate }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	supported := r.supportsTrafficSnapshot
	capsEpoch := r.snapshotCapsEpoch
	r.mu.RUnlock()
	if capsEpoch != r.snapshotSeenCapsEpoch {
		r.snapshotSeenCapsEpoch = capsEpoch
		r.snapshotGeneration++
		r.snapshotRevision = ""
		r.snapshotStructure = nil
		r.snapshotClients = nil
		r.snapshotUnsupportedUntil = time.Time{}
	}
	if supported && time.Now().After(r.snapshotUnsupportedUntil) {
		snap, err := r.fetchCompactSnapshot(ctx)
		if err == nil {
			return snap, nil
		}
		r.snapshotGeneration++
		r.snapshotRevision = ""
		r.snapshotStructure = nil
		r.snapshotClients = nil
		var status *remoteHTTPError
		if !errors.As(err, &status) || (status.code != http.StatusNotFound && status.code != http.StatusMethodNotAllowed) {
			return nil, err
		}
		r.snapshotUnsupportedUntil = time.Now().Add(5 * time.Minute)
	}
	snap, err := r.fetchLegacyTrafficSnapshot(ctx)
	if err == nil && ctx.Err() != nil {
		snap = nil
		err = ctx.Err()
	}
	if err != nil {
		r.snapshotGeneration++
		r.snapshotRevision = ""
		r.snapshotStructure = nil
		r.snapshotClients = nil
	}
	return snap, err
}

func (r *Remote) fetchCompactSnapshot(ctx context.Context) (*TrafficSnapshot, error) {
	path := "panel/api/inbounds/trafficSnapshot?revision=" + url.QueryEscape(r.snapshotRevision)
	env, err := r.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var wire NodeTrafficSnapshotV1
	if err := json.Unmarshal(env.Obj, &wire); err != nil {
		return nil, fmt.Errorf("decode traffic snapshot: %w", err)
	}
	if wire.Version != 1 || wire.Revision == "" || wire.Clients == nil || wire.InboundTraffic == nil {
		return nil, fmt.Errorf("incomplete or unsupported traffic snapshot")
	}
	structure := wire.Structure
	if structure == nil {
		if wire.Revision != r.snapshotRevision || r.snapshotStructure == nil {
			return nil, fmt.Errorf("traffic snapshot has no matching structure")
		}
		if len(wire.Clients) != len(r.snapshotClients) {
			return nil, fmt.Errorf("traffic snapshot omitted client counters")
		}
		for _, row := range wire.Clients {
			if identity, ok := r.snapshotClients[row.ID]; !ok || identity.InboundID != row.InboundID || identity.Email != row.Email {
				return nil, fmt.Errorf("traffic snapshot changed client roster without structure")
			}
		}
		structure = r.snapshotStructure
	}
	snap, err := wire.expand(structure)
	if err != nil {
		return nil, err
	}
	snap.StructuralRevision = fmt.Sprintf("%p/%d/%s", r, r.snapshotGeneration, wire.Revision)
	if wire.Structure != nil {
		identities := make(map[int]snapshotClientIdentity, len(wire.Clients))
		for _, row := range wire.Clients {
			if _, duplicate := identities[row.ID]; row.ID <= 0 || duplicate {
				return nil, fmt.Errorf("invalid snapshot client identity")
			}
			identities[row.ID] = snapshotClientIdentity{InboundID: row.InboundID, Email: row.Email}
		}
		r.snapshotClients = identities
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.snapshotRevision = wire.Revision
	r.snapshotStructure = structure
	return snap, nil
}

func (w *NodeTrafficSnapshotV1) expand(structure *NodeSnapshotStructure) (*TrafficSnapshot, error) {
	if structure.Inbounds == nil || structure.Attachments == nil {
		return nil, fmt.Errorf("incomplete traffic snapshot structure")
	}
	snap := &TrafficSnapshot{Inbounds: make([]*model.Inbound, 0, len(structure.Inbounds)), OnlineEmails: w.OnlineEmails, OnlineTree: w.OnlineTree, ActiveInboundTree: w.ActiveInboundTree, LastOnlineMap: w.LastOnlineMap}
	byID := make(map[int]*model.Inbound, len(structure.Inbounds))
	seen := make(map[int]map[string]bool, len(structure.Inbounds))
	for _, ib := range structure.Inbounds {
		if ib == nil || byID[ib.Id] != nil {
			return nil, fmt.Errorf("invalid or duplicate snapshot inbound")
		}
		copy := *ib
		if ib.NodeID != nil {
			nodeID := *ib.NodeID
			copy.NodeID = &nodeID
		}
		if ib.FallbackParent != nil {
			parent := *ib.FallbackParent
			copy.FallbackParent = &parent
		}
		copy.ClientStats = make([]xray.ClientTraffic, 0)
		byID[copy.Id] = &copy
		seen[copy.Id] = map[string]bool{}
		snap.Inbounds = append(snap.Inbounds, &copy)
	}
	totals := make(map[int]bool, len(w.InboundTraffic))
	for _, counter := range w.InboundTraffic {
		ib := byID[counter.ID]
		if ib == nil || totals[counter.ID] {
			return nil, fmt.Errorf("invalid snapshot inbound counters")
		}
		totals[counter.ID] = true
		ib.Up = counter.Up
		ib.Down = counter.Down
	}
	if len(totals) != len(byID) {
		return nil, fmt.Errorf("missing snapshot inbound counters")
	}
	stats := make(map[string]xray.ClientTraffic, len(w.Clients))
	clientIDs := make(map[int]bool, len(w.Clients))
	for _, counter := range w.Clients {
		row := counter.traffic()
		if row.Id > 0 && clientIDs[row.Id] {
			return nil, fmt.Errorf("duplicate snapshot client identity")
		}
		clientIDs[row.Id] = true
		if _, duplicate := stats[row.Email]; row.Email != "" && duplicate {
			return nil, fmt.Errorf("duplicate snapshot client counters")
		}
		stats[row.Email] = row
		if home := byID[row.InboundId]; home != nil {
			home.ClientStats = append(home.ClientStats, row)
			seen[home.Id][strings.ToLower(row.Email)] = true
		}
	}
	for _, attachment := range structure.Attachments {
		if attachment.Email == "" {
			continue
		}
		ib := byID[attachment.InboundID]
		if ib == nil {
			return nil, fmt.Errorf("snapshot attachment references an unknown inbound")
		}
		key := strings.ToLower(attachment.Email)
		if row, ok := stats[attachment.Email]; ok && !seen[ib.Id][key] {
			ib.ClientStats = append(ib.ClientStats, row)
			seen[ib.Id][key] = true
		}
	}
	return snap, nil
}
