package service

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

type LinkService struct{}

type LinkAssignResult struct {
	Clients  int      `json:"clients"`
	Links    int      `json:"links"`
	Attached int      `json:"attached"`
	Skipped  int      `json:"skipped"`
	Missing  []string `json:"missing"`
}

func (s *LinkService) GetLinks() ([]*model.Link, error) {
	var rows []*model.Link
	err := database.GetDB().Order("sort_index asc, id asc").Find(&rows).Error
	return rows, err
}

func (s *LinkService) GetLink(id int) (*model.Link, error) {
	row := &model.Link{}
	if err := database.GetDB().First(row, id).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func normalizeManagedLink(link *model.Link) error {
	rows, err := normalizeExternalLinks([]ExternalLinkInput{{
		Kind:   link.Kind,
		Value:  link.Value,
		Remark: link.Remark,
	}})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return common.NewError("link value is required")
	}
	link.Kind = rows[0].Kind
	link.Value = rows[0].Value
	link.Remark = rows[0].Remark
	return nil
}

func (s *LinkService) AddLink(link *model.Link) (*model.Link, error) {
	if err := normalizeManagedLink(link); err != nil {
		return nil, err
	}
	link.Id = 0
	if err := database.GetDB().Create(link).Error; err != nil {
		return nil, err
	}
	return link, nil
}

func (s *LinkService) UpdateLink(id int, link *model.Link) (*model.Link, error) {
	if err := normalizeManagedLink(link); err != nil {
		return nil, err
	}
	db := database.GetDB()
	existing := &model.Link{}
	if err := db.First(existing, id).Error; err != nil {
		return nil, err
	}
	link.Id = id
	link.SortIndex = existing.SortIndex
	link.CreatedAt = existing.CreatedAt
	if err := db.Save(link).Error; err != nil {
		return nil, err
	}
	return s.GetLink(id)
}

func (s *LinkService) DeleteLink(id int) error {
	return database.GetDB().Delete(&model.Link{}, id).Error
}

func (s *LinkService) DeleteLinks(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return database.GetDB().Where("id IN ?", ids).Delete(&model.Link{}).Error
}

func (s *LinkService) SetLinkEnable(id int, enable bool) error {
	return database.GetDB().Model(&model.Link{}).Where("id = ?", id).Update("is_disabled", !enable).Error
}

func (s *LinkService) SetLinksEnable(ids []int, enable bool) error {
	if len(ids) == 0 {
		return nil
	}
	return database.GetDB().Model(&model.Link{}).Where("id IN ?", ids).Update("is_disabled", !enable).Error
}

func (s *LinkService) ReorderLinks(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	tx := database.GetDB().Begin()
	for i, id := range ids {
		if err := tx.Model(&model.Link{}).Where("id = ?", id).Update("sort_index", i).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (s *LinkService) AssignLinks(linkIds []int, emails []string) (*LinkAssignResult, error) {
	result := &LinkAssignResult{Missing: []string{}}
	if len(linkIds) == 0 {
		return result, common.NewError("select at least one link")
	}
	if len(emails) == 0 {
		return result, common.NewError("select at least one client")
	}

	db := database.GetDB()
	var links []model.Link
	if err := db.Where("id IN ? AND is_disabled = ?", linkIds, false).
		Order("sort_index asc, id asc").
		Find(&links).Error; err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return result, common.NewError("no enabled links selected")
	}

	cleanEmails := make([]string, 0, len(emails))
	seenEmails := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seenEmails[key]; ok {
			continue
		}
		seenEmails[key] = struct{}{}
		cleanEmails = append(cleanEmails, email)
	}
	if len(cleanEmails) == 0 {
		return result, common.NewError("select at least one client")
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var clients []model.ClientRecord
		if err := tx.Where("email IN ?", cleanEmails).Find(&clients).Error; err != nil {
			return err
		}
		byEmail := make(map[string]model.ClientRecord, len(clients))
		for _, client := range clients {
			byEmail[strings.ToLower(client.Email)] = client
		}
		for _, email := range cleanEmails {
			if _, ok := byEmail[strings.ToLower(email)]; !ok {
				result.Missing = append(result.Missing, email)
			}
		}

		result.Clients = len(clients)
		result.Links = len(links)
		clientIDs := make([]int, 0, len(clients))
		for _, client := range clients {
			clientIDs = append(clientIDs, client.Id)
		}

		var existing []model.ClientExternalLink
		if err := tx.Where("client_id IN ?", clientIDs).Find(&existing).Error; err != nil {
			return err
		}
		seenByClient := make(map[int]map[string]struct{}, len(clients))
		maxSortByClient := make(map[int]int, len(clients))
		for _, client := range clients {
			seenByClient[client.Id] = map[string]struct{}{}
			maxSortByClient[client.Id] = -1
		}
		for _, row := range existing {
			seenByClient[row.ClientId][row.Kind+"\x00"+row.Value] = struct{}{}
			if row.SortIndex > maxSortByClient[row.ClientId] {
				maxSortByClient[row.ClientId] = row.SortIndex
			}
		}

		rowsToCreate := make([]model.ClientExternalLink, 0, len(clients)*len(links))
		for _, client := range clients {
			seen := seenByClient[client.Id]
			maxSort := maxSortByClient[client.Id]
			for _, link := range links {
				key := link.Kind + "\x00" + link.Value
				if _, ok := seen[key]; ok {
					result.Skipped++
					continue
				}
				maxSort++
				rowsToCreate = append(rowsToCreate, model.ClientExternalLink{
					ClientId:  client.Id,
					Kind:      link.Kind,
					Value:     link.Value,
					Remark:    link.Remark,
					SortIndex: maxSort,
				})
				seen[key] = struct{}{}
				result.Attached++
			}
		}
		if len(rowsToCreate) > 0 {
			return tx.CreateInBatches(rowsToCreate, 200).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
