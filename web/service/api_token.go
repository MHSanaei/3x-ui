package service

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/util/random"
)

type ApiTokenService struct{}

const apiTokenLength = 48

type ApiTokenView struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
}

func toView(t *model.ApiToken) *ApiTokenView {
	return &ApiTokenView{
		Id:        t.Id,
		Name:      t.Name,
		Token:     t.Token,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
	}
}

func (s *ApiTokenService) List() ([]*ApiTokenView, error) {
	db := database.GetDB()
	var rows []*model.ApiToken
	if err := db.Model(model.ApiToken{}).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*ApiTokenView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toView(r))
	}
	return out, nil
}

func (s *ApiTokenService) Create(name string) (*ApiTokenView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, common.NewError("token name is required")
	}
	if len(name) > 64 {
		return nil, common.NewError("token name must be 64 characters or fewer")
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(model.ApiToken{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewError("a token with that name already exists")
	}
	row := &model.ApiToken{
		Name:    name,
		Token:   random.Seq(apiTokenLength),
		Enabled: true,
	}
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}
	return toView(row), nil
}

func (s *ApiTokenService) Delete(id int) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	db := database.GetDB()
	return db.Where("id = ?", id).Delete(model.ApiToken{}).Error
}

func (s *ApiTokenService) SetEnabled(id int, enabled bool) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	db := database.GetDB()
	res := db.Model(model.ApiToken{}).Where("id = ?", id).Update("enabled", enabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("token not found")
	}
	return nil
}

// Match returns true when the presented bearer token matches any enabled
// row in api_tokens. Uses constant-time compare per row so a remote
// attacker can't time-attack tokens byte-by-byte.
func (s *ApiTokenService) Match(presented string) bool {
	if presented == "" {
		return false
	}
	db := database.GetDB()
	var rows []*model.ApiToken
	if err := db.Model(model.ApiToken{}).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return false
	}
	presentedBytes := []byte(presented)
	matched := false
	for _, r := range rows {
		if subtle.ConstantTimeCompare([]byte(r.Token), presentedBytes) == 1 {
			matched = true
		}
	}
	return matched
}
