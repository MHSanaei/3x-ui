package service

import (
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"

	"gorm.io/gorm"
)

// ExternalLinkInput is one row from the client form's Links tab.
type ExternalLinkInput struct {
	Id         int    `json:"id"`
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Remark     string `json:"remark"`
	Enable     *bool  `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
	NamePrefix string `json:"namePrefix"`
}

func (s *ClientService) GetExternalLinksForRecord(id int) ([]model.ClientExternalLink, error) {
	var rows []model.ClientExternalLink
	if err := database.GetDB().
		Where("client_id = ?", id).
		Order("sort_index ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// normalizeExternalLinks validates and orders the incoming rows. A "link" must
// parse to a supported share-link scheme; a "subscription" must be an http(s)
// URL. Blank values are dropped; an invalid value is a hard error so the
// operator gets immediate feedback instead of a silently missing config.
func normalizeExternalLinks(inputs []ExternalLinkInput) ([]model.ClientExternalLink, error) {
	out := make([]model.ClientExternalLink, 0, len(inputs))
	for _, in := range inputs {
		value := strings.TrimSpace(in.Value)
		if value == "" {
			continue
		}
		kind := strings.TrimSpace(in.Kind)
		switch kind {
		case model.ExternalLinkKindSubscription:
			if !isHTTPURL(value) {
				return nil, common.NewError("external subscription must be an http(s) URL: " + value)
			}
		case model.ExternalLinkKindLink, "":
			kind = model.ExternalLinkKindLink
			if _, err := link.ParseLink(value); err != nil {
				return nil, common.NewError("unsupported or invalid share link: " + value)
			}
		default:
			return nil, common.NewError("unknown external link kind: " + kind)
		}
		enable := true
		if in.Enable != nil {
			enable = *in.Enable
		}
		out = append(out, model.ClientExternalLink{
			Id:         in.Id,
			Kind:       kind,
			Value:      value,
			Remark:     strings.TrimSpace(in.Remark),
			Enable:     &enable,
			ExpiryTime: in.ExpiryTime,
			NamePrefix: in.NamePrefix,
			SortIndex:  len(out),
		})
	}
	return out, nil
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// SetExternalLinksForRecord replaces a client's entire external-link set.
func (s *ClientService) SetExternalLinksForRecord(id int, inputs []ExternalLinkInput) error {
	rows, err := normalizeExternalLinks(inputs)
	if err != nil {
		return err
	}
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		var existing []model.ClientExternalLink
		if err := tx.Where("client_id = ?", id).Find(&existing).Error; err != nil {
			return err
		}
		byId := make(map[int]model.ClientExternalLink, len(existing))
		byKindValue := make(map[string]model.ClientExternalLink, len(existing))
		for _, row := range existing {
			byId[row.Id] = row
			key := row.Kind + "\x00" + row.Value
			if _, ok := byKindValue[key]; !ok {
				byKindValue[key] = row
			}
		}
		if err := tx.Where("client_id = ?", id).Delete(&model.ClientExternalLink{}).Error; err != nil {
			return err
		}
		for i := range rows {
			if old, ok := byId[rows[i].Id]; ok && old.Kind == rows[i].Kind && old.Value == rows[i].Value {
				rows[i].LastFetchAt = old.LastFetchAt
				rows[i].LastFetchError = old.LastFetchError
			} else if old, ok := byKindValue[rows[i].Kind+"\x00"+rows[i].Value]; ok {
				rows[i].LastFetchAt = old.LastFetchAt
				rows[i].LastFetchError = old.LastFetchError
			}
			rows[i].Id = 0
			rows[i].ClientId = id
			if err := tx.Create(&rows[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ClientService) SetExternalLinksByEmail(email string, inputs []ExternalLinkInput) error {
	if strings.TrimSpace(email) == "" {
		return common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	return s.SetExternalLinksForRecord(rec.Id, inputs)
}
