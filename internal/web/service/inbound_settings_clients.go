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

// settingsField reads one top-level settings key. The client list beside it is skipped
// rather than decoded, so reading a cipher or a network costs no per-client allocation.
func settingsField(settings, key string) any {
	var top map[string]json.RawMessage
	if json.Unmarshal([]byte(settings), &top) != nil {
		return nil
	}
	var value any
	_ = json.Unmarshal(top[key], &value)
	return value
}

// settingsObject decodes a settings blob; ok is false when it is not valid JSON
// for an object, and a JSON null decodes to a nil map.
func settingsObject(settings string) (map[string]any, bool) {
	var fields map[string]any
	if json.Unmarshal([]byte(settings), &fields) != nil {
		return nil, false
	}
	return fields, true
}

func encodeSettings(fields map[string]any) (string, error) {
	out, err := json.MarshalIndent(fields, "", "  ")
	return string(out), err
}

// rewriteSettingsClients runs edit over the decoded client objects and re-encodes the
// settings only when edit reports a change.
func rewriteSettingsClients(settings string, edit func(clients []any) bool) (string, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	clients, ok := parsed["clients"].([]any)
	if !ok || !edit(clients) {
		return settings, false
	}
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
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
