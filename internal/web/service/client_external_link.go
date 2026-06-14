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
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Remark string `json:"remark"`
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
		out = append(out, model.ClientExternalLink{
			Kind:      kind,
			Value:     value,
			Remark:    strings.TrimSpace(in.Remark),
			SortIndex: len(out),
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
		if err := tx.Where("client_id = ?", id).Delete(&model.ClientExternalLink{}).Error; err != nil {
			return err
		}
		for i := range rows {
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
