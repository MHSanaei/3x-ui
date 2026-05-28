package service

import (
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"

	"gorm.io/gorm"
)

type FallbackService struct{}

// FallbackInput is the payload shape POSTed by the inbound form.
type FallbackInput struct {
	ChildId   int    `json:"childId"`
	Name      string `json:"name"`
	Alpn      string `json:"alpn"`
	Path      string `json:"path"`
	Dest      string `json:"dest"`
	Xver      int    `json:"xver"`
	SortOrder int    `json:"sortOrder"`
}

// GetByMaster returns every fallback rule attached to the master inbound.
func (s *FallbackService) GetByMaster(masterId int) ([]model.InboundFallback, error) {
	var rows []model.InboundFallback
	err := database.GetDB().
		Where("master_id = ?", masterId).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetParentForChild finds the first fallback rule that points at childId.
// Used by client-link generation: when a child inbound is attached as a
// fallback, its client links should advertise the master's address+port
// and TLS instead of the child's loopback listen.
func (s *FallbackService) GetParentForChild(childId int) (*model.InboundFallback, error) {
	var row model.InboundFallback
	err := database.GetDB().
		Where("child_id = ?", childId).
		Order("sort_order ASC, id ASC").
		First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// SetByMaster replaces the master's entire fallback list atomically.
func (s *FallbackService) SetByMaster(masterId int, items []FallbackInput) error {
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("master_id = ?", masterId).Delete(&model.InboundFallback{}).Error; err != nil {
			return err
		}
		for i, c := range items {
			if c.ChildId <= 0 || c.ChildId == masterId {
				continue
			}
			row := model.InboundFallback{
				MasterId:  masterId,
				ChildId:   c.ChildId,
				Name:      c.Name,
				Alpn:      c.Alpn,
				Path:      c.Path,
				Dest:      c.Dest,
				Xver:      c.Xver,
				SortOrder: c.SortOrder,
			}
			if row.SortOrder == 0 {
				row.SortOrder = i
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *FallbackService) BuildFallbacksJSON(tx *gorm.DB, masterId int) ([]map[string]any, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	var rows []model.InboundFallback
	err := tx.Where("master_id = ?", masterId).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	childIds := make([]int, 0, len(rows))
	for i := range rows {
		childIds = append(childIds, rows[i].ChildId)
	}
	var children []model.Inbound
	if err := tx.Where("id IN ?", childIds).Find(&children).Error; err != nil {
		return nil, err
	}
	byId := make(map[int]*model.Inbound, len(children))
	for i := range children {
		byId[children[i].Id] = &children[i]
	}

	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		child, ok := byId[r.ChildId]
		if !ok {
			continue
		}
		dest := r.Dest
		if dest == "" {
			listen := strings.TrimSpace(child.Listen)
			if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "::0" {
				listen = "127.0.0.1"
			}
			dest = fmt.Sprintf("%s:%d", listen, child.Port)
		}
		entry := map[string]any{
			"dest": dest,
		}
		if r.Name != "" {
			entry["name"] = r.Name
		}
		if r.Alpn != "" {
			entry["alpn"] = r.Alpn
		}
		if r.Path != "" {
			entry["path"] = r.Path
		}
		if r.Xver > 0 {
			entry["xver"] = r.Xver
		}
		out = append(out, entry)
	}
	return out, nil
}
