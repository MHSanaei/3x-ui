package wechat

import (
	"context"
	"strings"
	"unicode"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
	spellings := reportEmailSpellings(emails)
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

// SQLite LOWER folds ASCII only; match the same Unicode spellings as inbound identity lookups.
func reportEmailSpellings(emails []string) []string {
	seen := make(map[string]bool, len(emails))
	spellings := make([]string, 0, len(emails))
	for _, email := range emails {
		lower := strings.ToLower(email)
		variants := []string{lower}
		if strings.ContainsFunc(email, func(r rune) bool { return r > unicode.MaxASCII }) {
			title := lower
			if runes := []rune(lower); len(runes) > 0 {
				title = strings.ToUpper(string(runes[:1])) + string(runes[1:])
			}
			variants = append(variants, email, strings.ToUpper(email), title)
		}
		for _, variant := range variants {
			if !seen[variant] {
				seen[variant] = true
				spellings = append(spellings, variant)
			}
		}
	}
	return spellings
}
