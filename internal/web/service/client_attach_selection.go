package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const attachCandidatePageSize = 100

type AttachCandidateParams struct {
	SourceID int    `form:"sourceId"`
	Targets  string `form:"inboundIds"`
	Page     int    `form:"page"`
	Search   string `form:"search"`
	Group    string `form:"group"`
	Freeze   bool   `form:"freeze"`
}

type AttachCandidate struct {
	ID              int    `json:"id" gorm:"column:id" example:"1"`
	Email           string `json:"email" example:"alice@example.test"`
	Group           string `json:"group" example:"staff"`
	Comment         string `json:"comment" example:"Example client"`
	Enable          bool   `json:"enable" example:"true"`
	AlreadyAttached bool   `json:"alreadyAttached" example:"false"`
}

type AttachCandidatePage struct {
	Items      []AttachCandidate `json:"items"`
	Total      int64             `json:"total" example:"120"`
	Eligible   int64             `json:"eligible" example:"118"`
	Groups     []string          `json:"groups" example:"[\"staff\"]"`
	ScopeToken string            `json:"scopeToken,omitempty"`
	MaxID      int               `json:"maxId,omitempty"`
}

type AttachSelection struct {
	Mode              string `json:"mode"`
	SourceID          int    `json:"sourceId"`
	ClientIDs         []int  `json:"clientIds"`
	ExcludedClientIDs []int  `json:"excludedClientIds"`
	ScopeToken        string `json:"scopeToken"`
}

type AttachSelectionResult struct {
	Attached int `json:"attached" example:"9997"`
	Skipped  int `json:"skipped" example:"0"`
	Errors   int `json:"errors" example:"0"`
}

func attachScope(db *gorm.DB, sourceID int, ids []int) ([]int, error) {
	if len(ids) > 100 || sourceID < 0 || (sourceID == 0 && len(ids) == 0) {
		return nil, fmt.Errorf("invalid client attachment scope")
	}
	targets := uniqueInts(ids)
	slices.Sort(targets)
	inbounds := slices.Clone(targets)
	if sourceID > 0 {
		if slices.Contains(targets, sourceID) {
			return nil, fmt.Errorf("source inbound cannot be a target")
		}
		inbounds = append(inbounds, sourceID)
	}
	var rows []model.Inbound
	if err := db.Select("id", "protocol").Where("id IN ?", inbounds).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(inbounds) {
		return nil, fmt.Errorf("an inbound in the attachment scope no longer exists")
	}
	for _, row := range rows {
		switch row.Protocol {
		case model.VMESS, model.VLESS, model.Trojan, model.Shadowsocks, model.Hysteria,
			model.MTProto, model.WireGuard, model.AmneziaWG, model.TUIC, model.MASQUE:
		default:
			return nil, fmt.Errorf("inbound %d does not support attaching clients", row.Id)
		}
	}
	return targets, nil
}

func attachCandidates(db *gorm.DB, sourceID int) *gorm.DB {
	q := db.Table("clients AS c").Where("c.email <> ''")
	if sourceID > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM client_inbounds AS src WHERE src.client_id = c.id AND src.inbound_id = ?)", sourceID)
	}
	return q
}

const attachedToTargetsSQL = "(SELECT COUNT(*) FROM client_inbounds AS dst WHERE dst.client_id = c.id AND dst.inbound_id IN ?) = ?"

func eligibleAttachCandidates(q *gorm.DB, targets []int) *gorm.DB {
	if len(targets) > 0 {
		return q.Where("NOT ("+attachedToTargetsSQL+")", targets, len(targets))
	}
	return q
}

func writeAttachID(h hash.Hash, id int) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(id))
	_, _ = h.Write(buf[:])
}

func attachScopeHash(sourceID int, targets []int) hash.Hash {
	h := sha256.New()
	writeAttachID(h, sourceID)
	writeAttachID(h, len(targets))
	for _, id := range targets {
		writeAttachID(h, id)
	}
	return h
}

func (s *ClientService) ListAttachCandidates(ctx context.Context, p AttachCandidateParams) (*AttachCandidatePage, error) {
	if p.Page < 0 || p.Page > 1_000_000 || len(p.Search) > 256 || len(p.Group) > 256 {
		return nil, fmt.Errorf("invalid client attachment page")
	}
	db := database.GetDB().WithContext(ctx)
	var ids []int
	if p.Targets != "" {
		for part := range strings.SplitSeq(p.Targets, ",") {
			id, err := strconv.Atoi(part)
			if err != nil || id < 1 || len(ids) >= 100 {
				return nil, fmt.Errorf("invalid attachment target ID")
			}
			ids = append(ids, id)
		}
	}
	targets, err := attachScope(db, p.SourceID, ids)
	if err != nil {
		return nil, err
	}
	resp := &AttachCandidatePage{Items: []AttachCandidate{}, Groups: []string{}}
	eligible := eligibleAttachCandidates(attachCandidates(db, p.SourceID), targets)
	if p.Freeze {
		if len(targets) == 0 {
			return nil, fmt.Errorf("select attachment targets first")
		}
		h := attachScopeHash(p.SourceID, targets)
		rows, err := eligible.Select("c.id").Order("c.id").Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			writeAttachID(h, id)
			resp.Eligible++
			resp.MaxID = id
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		resp.ScopeToken = hex.EncodeToString(h.Sum(nil))
		return resp, nil
	}
	if err := eligible.Count(&resp.Eligible).Error; err != nil {
		return nil, err
	}

	filtered := attachCandidates(db, p.SourceID)
	if p.SourceID == 0 {
		if err := attachCandidates(db, 0).Where("c.group_name <> ''").
			Distinct("c.group_name").Order("c.group_name").Limit(100).Pluck("c.group_name", &resp.Groups).Error; err != nil {
			return nil, err
		}
		if p.Group != "" {
			filtered = filtered.Where("c.group_name = ?", p.Group)
		}
	}
	if search := strings.TrimSpace(p.Search); search != "" {
		col := "c.group_name"
		if p.SourceID > 0 {
			col = "c.comment"
		}
		const like = " LIKE ? ESCAPE '\\'"
		predicates := []string{"LOWER(c.email)" + like, "LOWER(" + col + ")" + like}
		pattern := "%" + escapeLikeLiteral(strings.ToLower(search)) + "%"
		args := []any{pattern, pattern}
		for _, variant := range caseVariants(search) {
			pattern := "%" + escapeLikeLiteral(variant) + "%"
			predicates = append(predicates, "c.email"+like, col+like)
			args = append(args, pattern, pattern)
		}
		filtered = filtered.Where("("+strings.Join(predicates, " OR ")+")", args...)
	}
	if err := filtered.Count(&resp.Total).Error; err != nil {
		return nil, err
	}
	columns := "c.id, c.email, c.group_name AS \"group\", c.comment, c.enable, "
	if len(targets) == 0 {
		filtered = filtered.Select(columns + "false AS already_attached")
	} else {
		filtered = filtered.Select(columns+attachedToTargetsSQL+" AS already_attached", targets, len(targets))
	}
	err = filtered.Order("c.id").Offset(max(0, p.Page-1) * attachCandidatePageSize).
		Limit(attachCandidatePageSize).Scan(&resp.Items).Error
	return resp, err
}

func (s *ClientService) BulkAttachSelection(ctx context.Context, inboundSvc *InboundService, targets []int, selection AttachSelection) (*AttachSelectionResult, bool, error) {
	db := database.GetDB().WithContext(ctx)
	targets, err := attachScope(db, selection.SourceID, targets)
	if err != nil {
		return nil, false, err
	}
	if len(targets) == 0 {
		return nil, false, fmt.Errorf("select attachment targets first")
	}
	stale := fmt.Errorf("client attachment selection changed; clear the selection and select clients again")
	q := eligibleAttachCandidates(attachCandidates(db, selection.SourceID), targets)
	var records []*model.ClientRecord
	switch selection.Mode {
	case "explicit":
		if len(selection.ClientIDs) == 0 || len(selection.ClientIDs) > 50_000 || selection.ScopeToken != "" || len(selection.ExcludedClientIDs) > 0 {
			return nil, false, fmt.Errorf("invalid explicit client attachment selection")
		}
		ids := uniqueInts(selection.ClientIDs)
		for _, chunk := range chunkInts(ids, 400) {
			var part []*model.ClientRecord
			if err := q.Session(&gorm.Session{}).Where("c.id IN ?", chunk).Find(&part).Error; err != nil {
				return nil, false, err
			}
			if len(part) != len(chunk) {
				return nil, false, stale
			}
			records = append(records, part...)
		}
	case "allMatching":
		if len(selection.ScopeToken) != sha256.Size*2 || len(selection.ClientIDs) > 0 || len(selection.ExcludedClientIDs) > 50_000 {
			return nil, false, fmt.Errorf("invalid all-clients attachment selection")
		}
		excluded := make(map[int]struct{}, len(selection.ExcludedClientIDs))
		for _, id := range selection.ExcludedClientIDs {
			excluded[id] = struct{}{}
		}
		h := attachScopeHash(selection.SourceID, targets)
		// Find hydrates stored client settings through the model's read hooks.
		var eligibleRecords []*model.ClientRecord
		if err := q.Order("c.id").Find(&eligibleRecords).Error; err != nil {
			return nil, false, err
		}
		records = eligibleRecords[:0]
		for _, record := range eligibleRecords {
			writeAttachID(h, record.Id)
			if _, skip := excluded[record.Id]; skip {
				delete(excluded, record.Id)
				continue
			}
			records = append(records, record)
		}
		if len(excluded) > 0 || hex.EncodeToString(h.Sum(nil)) != selection.ScopeToken {
			return nil, false, stale
		}
	default:
		return nil, false, fmt.Errorf("invalid client attachment selection mode")
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	result, restart, err := s.bulkAttachRecords(inboundSvc, records, targets, &BulkAttachResult{})
	if err != nil {
		return nil, restart, err
	}
	return &AttachSelectionResult{Attached: len(result.Attached), Skipped: len(result.Skipped), Errors: len(result.Errors)}, restart, nil
}
