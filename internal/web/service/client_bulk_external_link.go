package service

import (
	"sort"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BulkExternalLinksResult struct {
	Added   int `json:"added" example:"4"`
	Skipped int `json:"skipped" example:"0"`
	Errors  int `json:"errors" example:"0"`
}

// BulkAppendExternalLinks appends panel-local subscription entries atomically.
// Client row locks serialize concurrent appends; no runtime configuration changes.
func (s *ClientService) BulkAppendExternalLinks(emails []string, inputs []ExternalLinkInput) (BulkAdjustResult, error) {
	result := BulkAdjustResult{ExternalLinks: &BulkExternalLinksResult{}}
	rows, err := normalizeExternalLinks(inputs)
	if err != nil {
		return result, err
	}
	if len(rows) == 0 {
		return result, common.NewError("at least one external link or subscription is required")
	}
	cleanEmails := trimmedUniqueEmails(emails)
	if len(cleanEmails) == 0 {
		return result, common.NewError("client emails are required")
	}
	sort.Strings(cleanEmails)
	err = database.GetDB().Transaction(func(tx *gorm.DB) error {
		records, err := clientRecordsByEmail(tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("id ASC"), cleanEmails)
		if err != nil {
			return err
		}
		ids := make([]int, 0, len(records))
		for _, email := range cleanEmails {
			if rec := records[email]; rec != nil {
				ids = append(ids, rec.Id)
			} else {
				result.Skipped = append(result.Skipped, BulkAdjustReport{Email: email, Reason: "client not found"})
				result.ExternalLinks.Errors++
			}
		}
		type linkKey struct{ kind, value string }
		seen := make(map[int]map[linkKey]bool, len(ids))
		nextSort := make(map[int]int, len(ids))
		for _, batch := range chunkInts(ids, sqlInChunk) {
			var existing []model.ClientExternalLink
			if err := tx.Where("client_id IN ?", batch).Find(&existing).Error; err != nil {
				return err
			}
			for _, row := range existing {
				if seen[row.ClientId] == nil {
					seen[row.ClientId] = make(map[linkKey]bool)
				}
				seen[row.ClientId][linkKey{row.Kind, row.Value}] = true
				if row.SortIndex >= nextSort[row.ClientId] {
					nextSort[row.ClientId] = row.SortIndex + 1
				}
			}
		}
		pending := make([]model.ClientExternalLink, 0)
		for _, id := range ids {
			if seen[id] == nil {
				seen[id] = make(map[linkKey]bool)
			}
			changed := false
			for _, row := range rows {
				key := linkKey{row.Kind, row.Value}
				if seen[id][key] {
					result.ExternalLinks.Skipped++
					continue
				}
				seen[id][key] = true
				row.ClientId, row.SortIndex = id, nextSort[id]
				nextSort[id]++
				pending = append(pending, row)
				changed = true
			}
			if changed {
				result.Adjusted++
			}
		}
		if len(pending) > 0 {
			if err := tx.CreateInBatches(&pending, 200).Error; err != nil {
				return err
			}
		}
		result.ExternalLinks.Added = len(pending)
		return nil
	})
	if err != nil {
		return BulkAdjustResult{ExternalLinks: &BulkExternalLinksResult{}}, err
	}
	return result, nil
}
