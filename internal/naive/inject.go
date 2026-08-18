package naive

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func InjectNaiveOutbounds(cfg *xray.Config) error {
	db := database.GetDB()
	if db == nil {
		return nil
	}

	var records []model.NaiveOutbound
	if err := db.Where("enabled = ?", true).Find(&records).Error; err != nil {
		return err
	}

	var outbounds []map[string]any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
			return err
		}
	}

	for _, record := range records {
		socks := BuildSocksOutbound(record.Tag, record.LocalPort)
		replaced := false
		for i, outbound := range outbounds {
			if outbound["tag"] == record.Tag && outbound["protocol"] == "naive" {
				outbounds[i] = socks
				replaced = true
				break
			}
		}
		if !replaced {
			outbounds = append(outbounds, socks)
		}
	}

	encoded, err := json.Marshal(outbounds)
	if err != nil {
		return err
	}
	cfg.OutboundConfigs = encoded
	return nil
}

func BuildSocksOutbound(tag string, port int) map[string]any {
	return map[string]any{
		"tag":      tag,
		"protocol": "socks",
		"settings": map[string]any{
			"servers": []map[string]any{{
				"address": "127.0.0.1",
				"port":    port,
				"users":   []any{},
			}},
		},
	}
}
