package wechat

import (
	"context"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type reportClientLimit struct {
	Email      string
	TotalGB    int64
	ExpiryTime int64
}

func reportClientLimits(ctx context.Context, inbounds []*model.Inbound) (map[string]reportClientLimit, error) {
	emails := make([]string, 0)
	seen := make(map[string]bool)
	for _, inbound := range inbounds {
		if inbound == nil {
			continue
		}
		for _, client := range inbound.ClientStats {
			key := strings.ToLower(client.Email)
			if !seen[key] && (client.Total == 0 || client.ExpiryTime == 0) {
				seen[key] = true
				emails = append(emails, client.Email)
			}
		}
	}
	limits := make(map[string]reportClientLimit, len(emails))
	spellings := service.EmailSpellings(emails)
	// Each spelling is bound twice, keeping each query at no more than 500 parameters.
	const batchSize = 250
	for start := 0; start < len(spellings); start += batchSize {
		batch := spellings[start:min(start+batchSize, len(spellings))]
		var rows []reportClientLimit
		if err := database.GetDB().WithContext(ctx).Model(&model.ClientRecord{}).
			Select("email", "total_gb", "expiry_time").Where("LOWER(email) IN ? OR email IN ?", batch, batch).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			limits[strings.ToLower(row.Email)] = row
		}
	}
	return limits, nil
}
