package database

import (
	"errors"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// EnsureClientGroup returns the canonical group row for name, creating it if
// absent. Matching is case-insensitive so casing cannot create a second group.
func EnsureClientGroup(tx *gorm.DB, name string) (*model.ClientGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	if tx == nil {
		tx = db
	}
	var group model.ClientGroup
	err := tx.Where("LOWER(name) = LOWER(?)", name).Order("id ASC").First(&group).Error
	if err == nil {
		return &group, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	group = model.ClientGroup{Name: name}
	if err := tx.Create(&group).Error; err != nil {
		// A concurrent writer may have created it between the lookup and insert.
		var existing model.ClientGroup
		if e := tx.Where("LOWER(name) = LOWER(?)", name).Order("id ASC").First(&existing).Error; e == nil {
			return &existing, nil
		}
		return nil, err
	}
	return &group, nil
}

// PruneHostGroupClientGroups drops bindings whose host group no longer has any
// hosts row (e.g. the inbound was deleted). Idempotent; runs on every start.
func PruneHostGroupClientGroups() error {
	return db.Exec(`DELETE FROM host_group_client_groups
		WHERE host_group_id NOT IN (SELECT DISTINCT group_id FROM hosts)
		   OR client_group_id NOT IN (SELECT id FROM client_groups)`).Error
}

// EnsureClientGroupRows repairs group labels that have no client_groups row and
// rewrites case variants to the canonical row's spelling. Idempotent.
func EnsureClientGroupRows() error {
	var names []string
	if err := db.Model(&model.ClientRecord{}).
		Where("TRIM(COALESCE(group_name, '')) <> ''").
		Distinct().Pluck("group_name", &names).Error; err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return db.Transaction(func(tx *gorm.DB) error {
		for _, raw := range names {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			group, err := EnsureClientGroup(tx, name)
			if err != nil {
				return err
			}
			if group == nil || group.Name == raw {
				continue
			}
			if err := tx.Model(&model.ClientRecord{}).
				Where("group_name = ?", raw).
				UpdateColumn("group_name", group.Name).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
