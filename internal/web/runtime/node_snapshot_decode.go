package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// Counters and memberships allow million-client deployments; catalogs retain the legacy cap.
const maxNodeSnapshotClients = 1 << 20

// Count raw entries before allocating typed rows, including for an oversized list of empty objects.
func decodeSnapshotList[T any](raw json.RawMessage, limit int) ([]T, error) {
	count, err := json_util.CountCappedList(raw, limit)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	result := make([]T, 0, count)
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func decodeSnapshotInbounds(raw json.RawMessage) ([]*model.Inbound, error) {
	rows, err := json_util.DecodeCappedList[json.RawMessage](raw, maxNodeInbounds)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	result := make([]*model.Inbound, len(rows))
	for i, row := range rows {
		if bytes.Equal(bytes.TrimSpace(row), []byte("null")) {
			continue
		}
		type inboundAlias model.Inbound
		inbound := &model.Inbound{}
		fields := struct {
			*inboundAlias
			Settings       json.RawMessage `json:"settings"`
			StreamSettings json.RawMessage `json:"streamSettings"`
			Sniffing       json.RawMessage `json:"sniffing"`
			ClientStats    json.RawMessage `json:"clientStats"`
		}{inboundAlias: (*inboundAlias)(inbound)}
		if err := json.Unmarshal(row, &fields); err != nil {
			return nil, err
		}
		// These legacy stats are replaced by the separately bounded compact counters.
		if _, err := json_util.CountCappedList(fields.ClientStats, maxNodeSnapshotClients); err != nil {
			return nil, err
		}
		inbound.Settings = snapshotConfigString(fields.Settings)
		inbound.StreamSettings = snapshotConfigString(fields.StreamSettings)
		inbound.Sniffing = snapshotConfigString(fields.Sniffing)
		result[i] = inbound
	}

	return result, nil
}

func snapshotConfigString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return value
		}
	}
	return string(raw)
}

func decodeNodeTrafficSnapshot(raw json.RawMessage) (NodeTrafficSnapshotV1, error) {
	var fields struct {
		Version           int             `json:"version"`
		Revision          string          `json:"revision"`
		Structure         json.RawMessage `json:"structure"`
		InboundTraffic    json.RawMessage `json:"inboundTraffic"`
		Clients           json.RawMessage `json:"clients"`
		OnlineEmails      json.RawMessage `json:"onlineEmails"`
		OnlineTree        json.RawMessage `json:"onlineTree"`
		ActiveInboundTree json.RawMessage `json:"activeInboundTree"`
		LastOnlineMap     json.RawMessage `json:"lastOnlineMap"`
	}
	var wire NodeTrafficSnapshotV1
	if err := json.Unmarshal(raw, &fields); err != nil {
		return wire, err
	}
	wire.Version, wire.Revision = fields.Version, fields.Revision
	if len(fields.Structure) > 0 && !bytes.Equal(bytes.TrimSpace(fields.Structure), []byte("null")) {
		var structure struct {
			Inbounds    json.RawMessage `json:"inbounds"`
			Attachments json.RawMessage `json:"attachments"`
		}
		if err := json.Unmarshal(fields.Structure, &structure); err != nil {
			return wire, err
		}
		wire.Structure = &NodeSnapshotStructure{}
		var err error
		if wire.Structure.Inbounds, err = decodeSnapshotInbounds(structure.Inbounds); err != nil {
			return wire, fmt.Errorf("structure.inbounds: %w", err)
		}
		if wire.Structure.Attachments, err = decodeSnapshotList[NodeSnapshotAttachment](structure.Attachments, maxNodeSnapshotClients); err != nil {
			return wire, fmt.Errorf("structure.attachments: %w", err)
		}
	}
	var err error
	if wire.Clients, err = decodeSnapshotList[NodeClientTraffic](fields.Clients, maxNodeSnapshotClients); err != nil {
		return wire, fmt.Errorf("clients: %w", err)
	}
	if wire.InboundTraffic, err = decodeSnapshotList[NodeInboundTraffic](fields.InboundTraffic, maxNodeInbounds); err != nil {
		return wire, fmt.Errorf("inboundTraffic: %w", err)
	}
	if wire.OnlineEmails, err = decodeSnapshotList[string](fields.OnlineEmails, maxNodeMapValues); err != nil {
		return wire, fmt.Errorf("onlineEmails: %w", err)
	}
	for _, entry := range []struct {
		name   string
		raw    json.RawMessage
		target any
	}{
		{"onlineTree", fields.OnlineTree, &wire.OnlineTree},
		{"activeInboundTree", fields.ActiveInboundTree, &wire.ActiveInboundTree},
		{"lastOnlineMap", fields.LastOnlineMap, &wire.LastOnlineMap},
	} {
		if len(entry.raw) > 0 {
			if err := decodeCappedValues(entry.raw, entry.target, maxNodeMapValues); err != nil {
				return wire, fmt.Errorf("%s: %w", entry.name, err)
			}
		}
	}
	return wire, nil
}
