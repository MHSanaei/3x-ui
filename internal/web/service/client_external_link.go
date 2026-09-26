package service

import (
	"errors"
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/link"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExternalLinkInput is one row from the client form's Links tab.
type ExternalLinkInput struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Remark     string `json:"remark"`
	Enable     *bool  `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
	NamePrefix string `json:"namePrefix"`
}

// externalLinkAssignmentRow is one client-scoped assignment joined with the
// library row it points at, in the shape the Links tab renders.
type externalLinkAssignmentRow struct {
	AssignmentId   int
	TargetId       int
	LinkId         int
	Enable         *bool
	ExpiryTime     int64
	Remark         string
	NamePrefix     string
	SortIndex      int
	CreatedAt      int64
	Kind           string
	Value          string
	LinkRemark     string
	LinkNamePrefix string
	LinkEnable     *bool
	LinkExpiryTime int64
	LastFetchAt    int64
	LastFetchError string
}

// resolvedExpiry applies one assignment's expiry over the library row: the
// sentinel means never, 0 means inherit, any other value wins.
func resolvedExpiry(linkExpiry, assignmentExpiry int64) int64 {
	switch {
	case assignmentExpiry < 0:
		return 0
	case assignmentExpiry > 0:
		return assignmentExpiry
	}
	return linkExpiry
}

// toClientExternalLink applies the assignment overrides over the library row,
// in the JSON shape the panel's Links tab has always used.
func (row externalLinkAssignmentRow) toClientExternalLink(clientId int) model.ClientExternalLink {
	enable := row.LinkEnable == nil || *row.LinkEnable
	if row.Enable != nil {
		enable = *row.Enable
	}
	expiry := resolvedExpiry(row.LinkExpiryTime, row.ExpiryTime)
	remark := row.LinkRemark
	if row.Remark != "" {
		remark = row.Remark
	}
	prefix := row.LinkNamePrefix
	if row.NamePrefix != "" {
		prefix = row.NamePrefix
	}
	return model.ClientExternalLink{
		Id:             row.AssignmentId,
		ClientId:       clientId,
		Kind:           row.Kind,
		Value:          row.Value,
		Remark:         remark,
		NamePrefix:     prefix,
		Enable:         &enable,
		ExpiryTime:     expiry,
		SortIndex:      row.SortIndex,
		LastFetchAt:    row.LastFetchAt,
		LastFetchError: row.LastFetchError,
		CreatedAt:      row.CreatedAt,
	}
}

// externalLinkAssignmentColumns feeds every read of an assignment joined with
// its library row, so the client form and the export cannot drift apart.
const externalLinkAssignmentColumns = "external_link_assignments.id AS assignment_id, " +
	"external_link_assignments.target_id AS target_id, external_link_assignments.link_id AS link_id, " +
	"external_link_assignments.enable AS enable, external_link_assignments.expiry_time AS expiry_time, " +
	"external_link_assignments.remark AS remark, external_link_assignments.name_prefix AS name_prefix, " +
	"external_link_assignments.sort_index AS sort_index, external_link_assignments.created_at AS created_at, " +
	"external_links.kind AS kind, external_links.value AS value, external_links.remark AS link_remark, " +
	"external_links.name_prefix AS link_name_prefix, external_links.enable AS link_enable, " +
	"external_links.expiry_time AS link_expiry_time, external_links.last_fetch_at AS last_fetch_at, " +
	"external_links.last_fetch_error AS last_fetch_error"

// GetExternalLinksForRecord returns the links a client owns directly: the form
// edits what it owns, the library page edits what everyone shares.
func (s *ClientService) GetExternalLinksForRecord(id int) ([]model.ClientExternalLink, error) {
	db := database.GetDB()
	rows := []externalLinkAssignmentRow{}
	if err := db.Model(&model.ExternalLinkAssignment{}).
		Select(externalLinkAssignmentColumns).
		Joins("JOIN external_links ON external_links.id = external_link_assignments.link_id").
		Where("external_link_assignments.target_type = ? AND external_link_assignments.target_id = ?",
			model.ExternalLinkTargetClient, id).
		Order("external_link_assignments.sort_index ASC, external_link_assignments.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.ClientExternalLink, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toClientExternalLink(id))
	}
	return out, nil
}

// SetExternalLinksForRecord replaces the links a client owns: library rows are
// reused by (kind, value) and created only when the entry is new.
func (s *ClientService) SetExternalLinksForRecord(id int, inputs []ExternalLinkInput) error {
	rows, err := normalizeExternalLinks(inputs)
	if err != nil {
		return err
	}
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		kept := make([]int, 0, len(rows))
		for index := range rows {
			linkRow, err := upsertExternalLinkTx(tx, rows[index])
			if err != nil {
				return err
			}
			kept = append(kept, linkRow.Id)
			assignment := model.ExternalLinkAssignment{
				LinkId:     linkRow.Id,
				TargetType: model.ExternalLinkTargetClient,
				TargetId:   id,
				// Naming and lifecycle stay on the assignment: editing one
				// client must not rename or disable the shared library entry.
				Enable: rows[index].Enable,
				// The form's 0 has always meant "no expiry": store it explicitly.
				ExpiryTime: model.AssignmentExpiry(rows[index].ExpiryTime),
				Remark:     rows[index].Remark,
				NamePrefix: rows[index].NamePrefix,
				SortIndex:  index,
				Origin:     model.ExternalLinkOriginPanel,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "link_id"}, {Name: "target_type"}, {Name: "target_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"enable", "expiry_time", "remark", "name_prefix", "sort_index"}),
			}).Create(&assignment).Error; err != nil {
				return err
			}
		}
		// Drop what this save no longer lists. Master-pushed node rows are left
		// alone: a local save must not delete another panel's bindings.
		removed := tx.Where("target_type = ? AND target_id = ? AND origin <> ?",
			model.ExternalLinkTargetClient, id, model.ExternalLinkOriginNode)
		if len(kept) > 0 {
			removed = removed.Where("link_id NOT IN ?", kept)
		}
		return removed.Delete(&model.ExternalLinkAssignment{}).Error
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

// upsertExternalLinkTx returns the library row for a (kind, value) pair, creating
// it only when new: the caller stores its own overrides on the assignment.
func upsertExternalLinkTx(tx *gorm.DB, in model.ClientExternalLink) (model.ExternalLink, error) {
	var existing model.ExternalLink
	err := tx.Where("kind = ? AND value = ?", in.Kind, in.Value).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return existing, err
	}
	created := model.ExternalLink{
		Kind:       in.Kind,
		Value:      in.Value,
		Remark:     in.Remark,
		NamePrefix: in.NamePrefix,
		Enable:     in.Enable,
		ExpiryTime: in.ExpiryTime,
		SortIndex:  in.SortIndex,
	}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if res.Error != nil {
		return existing, res.Error
	}
	if res.RowsAffected == 0 {
		// A parallel save inserted the same pair first: reuse its row.
		return existing, tx.Where("kind = ? AND value = ?", in.Kind, in.Value).First(&existing).Error
	}
	return created, nil
}

// normalizeExternalLinks validates and orders the incoming rows. A "link" must
// parse to a supported share-link scheme; a "subscription" must be an http(s)
// URL. Blank values are dropped; an invalid value is a hard error so the
// operator gets immediate feedback instead of a silently missing config.
func normalizeExternalLinks(inputs []ExternalLinkInput) ([]model.ClientExternalLink, error) {
	out := make([]model.ClientExternalLink, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
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
		if in.ExpiryTime < 0 {
			return nil, common.NewError("external link expiryTime must be 0 (never) or a future unix millisecond timestamp: " + value)
		}
		key := kind + "\x00" + value
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		enable := true
		if in.Enable != nil {
			enable = *in.Enable
		}
		out = append(out, model.ClientExternalLink{
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
