package service

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

func ParseInboundSettingsClients(settings string) ([]model.Client, error) {
	trimmed := strings.TrimSpace(settings)
	if trimmed == "" || trimmed == "null" {
		return nil, common.NewError("inbound settings is empty")
	}

	var payload struct {
		Clients json.RawMessage `json:"clients"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, err
	}
	if len(payload.Clients) == 0 || string(payload.Clients) == "null" {
		return nil, nil
	}

	var clients []model.Client
	if err := json.Unmarshal(payload.Clients, &clients); err != nil {
		return nil, err
	}
	return clients, nil
}

// settingsEntriesToClients decodes the wire entries a caller has already
// stamped, so a delta carries the persisted created_at / updated_at / subId
// rather than the pre-stamp values the request was parsed into.
func settingsEntriesToClients(entries []any) ([]model.Client, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	var clients []model.Client
	if err := json.Unmarshal(raw, &clients); err != nil {
		return nil, err
	}
	return clients, nil
}

// normalizeLegacyClientSettings rewrites settings whose clients still carry the
// string numbers of a v2.x export, so parsing and storage see a single shape.
func normalizeLegacyClientSettings(inbound *model.Inbound) {
	dec := json.NewDecoder(bytes.NewReader([]byte(inbound.Settings)))
	dec.UseNumber()
	var settings map[string]any
	if err := dec.Decode(&settings); err != nil {
		return
	}
	clients, _ := settings["clients"].([]any)
	changed := false
	for _, raw := range clients {
		if obj, ok := raw.(map[string]any); ok && model.NormalizeLegacyClientFields(obj) {
			changed = true
		}
	}
	if !changed {
		return
	}
	if out, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(out)
	}
}
