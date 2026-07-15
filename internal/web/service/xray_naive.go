package service

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/naive"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func injectNaiveOutbounds(cfg *xray.Config) error {
	return naive.InjectNaiveOutbounds(cfg)
}

func naiveConfigMetadata() ([]map[string]any, error) {
	var records []model.NaiveOutbound
	if err := database.GetDB().Order("tag asc").Find(&records).Error; err != nil {
		return nil, err
	}
	manager := naive.GetManager()
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"tag":       record.Tag,
			"proxy":     maskProxy(record.ProxyURL),
			"localPort": record.LocalPort,
			"running":   manager.IsRunning(record.Tag),
		})
	}
	return items, nil
}

func maskProxy(raw string) string {
	for _, prefix := range []string{"https://", "http://", "quic://"} {
		if !strings.HasPrefix(raw, prefix) {
			continue
		}
		rest := strings.TrimPrefix(raw, prefix)
		if at := strings.IndexByte(rest, '@'); at >= 0 {
			return prefix + "****@" + rest[at+1:]
		}
	}
	return raw
}
