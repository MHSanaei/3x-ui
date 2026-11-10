package service

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/xtls/xray-core/infra/conf"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func vlessDecryptionEnabled(settings string) bool {
	var s struct {
		Decryption string `json:"decryption"`
	}
	return json.Unmarshal([]byte(settings), &s) == nil && vlessEncValueSet(s.Decryption)
}

// normalizeVlessFallbacks also protects API/import callers that bypass the form adapter.
func normalizeVlessFallbacks(inbound *model.Inbound) error {
	if inbound.Protocol != model.VLESS {
		return nil
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return fmt.Errorf("inbound %d: invalid VLESS settings: %w", inbound.Id, err)
	}
	raw, exists := settings["fallbacks"]
	if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var fallbacks []*conf.VLessInboundFallback
	if err := json.Unmarshal(raw, &fallbacks); err != nil {
		return fmt.Errorf("inbound %d: invalid VLESS fallbacks: %w", inbound.Id, err)
	}
	if len(fallbacks) == 0 {
		delete(settings, "fallbacks")
		normalized, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		inbound.Settings = string(normalized)
		return nil
	}
	if vlessDecryptionEnabled(inbound.Settings) {
		return vlessFallbackConflict(inbound.Id)
	}
	for _, fallback := range fallbacks {
		if fallback == nil {
			return fmt.Errorf("inbound %d: VLESS fallback entries must be objects", inbound.Id)
		}
	}
	candidate := conf.VLessInboundConfig{Decryption: "none", Fallbacks: fallbacks}
	if _, err := candidate.Build(); err != nil {
		return fmt.Errorf("inbound %d: invalid VLESS fallbacks: %w", inbound.Id, err)
	}
	return nil
}

func vlessFallbackConflict(id int) error {
	return fmt.Errorf("inbound %d: VLESS fallbacks cannot be used with active server decryption; remove fallback rules before enabling decryption", id)
}

func validateVlessManagedFallbacks(tx *gorm.DB, inbound *model.Inbound) error {
	if inbound.Protocol != model.VLESS || !vlessDecryptionEnabled(inbound.Settings) || inbound.Id == 0 {
		return nil
	}
	fallbacks, err := (&FallbackService{}).BuildFallbacksJSON(tx, inbound.Id)
	if err != nil {
		return err
	}
	if len(fallbacks) > 0 {
		return vlessFallbackConflict(inbound.Id)
	}
	return nil
}
