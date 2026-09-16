package sub

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// seedClientExternalLink gives one client its own link the way the panel's Links
// tab does: a shared library row plus that client's own assignment overrides.
func seedClientExternalLink(t *testing.T, clientId int, row model.ClientExternalLink) model.ExternalLink {
	t.Helper()
	db := database.GetDB()
	var link model.ExternalLink
	err := db.Where("kind = ? AND value = ?", row.Kind, row.Value).First(&link).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		link = model.ExternalLink{
			Kind:       row.Kind,
			Value:      row.Value,
			Remark:     row.Remark,
			NamePrefix: row.NamePrefix,
			Enable:     row.Enable,
			ExpiryTime: row.ExpiryTime,
			SortIndex:  row.SortIndex,
		}
		if err := db.Create(&link).Error; err != nil {
			t.Fatalf("seed library link %q: %v", row.Value, err)
		}
	} else if err != nil {
		t.Fatalf("look up library link %q: %v", row.Value, err)
	}
	assignment := model.ExternalLinkAssignment{
		LinkId:     link.Id,
		TargetType: model.ExternalLinkTargetClient,
		TargetId:   clientId,
		Enable:     row.Enable,
		ExpiryTime: row.ExpiryTime,
		Remark:     row.Remark,
		NamePrefix: row.NamePrefix,
		SortIndex:  row.SortIndex,
		Origin:     model.ExternalLinkOriginPanel,
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&assignment).Error; err != nil {
		t.Fatalf("seed library assignment for %q: %v", row.Value, err)
	}
	return link
}
