package panel

import (
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

type ApiTokenService struct{}

const apiTokenLength = 48

type ApiTokenView struct {
	Id        int    `json:"id" example:"2"`
	Name      string `json:"name" example:"central-panel-a"`
	Token     string `json:"token,omitempty" example:"new-token-string"`
	Enabled   bool   `json:"enabled" example:"true"`
	CreatedAt int64  `json:"createdAt" example:"1736000000"`
	Scope     string `json:"scope" example:"admin"`
	ExpiresAt int64  `json:"expiresAt" example:"0"`
}

func apiTokenCreatedAtSeconds(createdAt int64) int64 {
	if createdAt >= model.ApiTokenUnixMillisecondsThreshold {
		return createdAt / 1000
	}
	return createdAt
}

// toView builds the metadata view returned by List. It never carries the
// token value: only a SHA-256 hash is stored, and the plaintext is shown
// exactly once at creation time.
func toView(t *model.ApiToken) *ApiTokenView {
	return &ApiTokenView{
		Id:        t.Id,
		Name:      t.Name,
		Enabled:   t.Enabled,
		CreatedAt: apiTokenCreatedAtSeconds(t.CreatedAt),
		Scope:     t.Scope,
		ExpiresAt: t.ExpiresAt,
	}
}

// NormalizeScope validates a requested scope, defaulting empty to admin so
// callers that omit it keep the legacy full-access behavior.
func NormalizeScope(scope string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", model.ApiScopeAdmin:
		return model.ApiScopeAdmin, nil
	case model.ApiScopeMonitor:
		return model.ApiScopeMonitor, nil
	case model.ApiScopeNodeSync:
		return model.ApiScopeNodeSync, nil
	default:
		return "", common.NewError("scope must be 'admin', 'monitor', or 'node-sync'")
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

func (s *ApiTokenService) Create(name, scope string, expiresAt int64) (*ApiTokenView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, common.NewError("token name is required")
	}
	if len(name) > 64 {
		return nil, common.NewError("token name must be 64 characters or fewer")
	}
	normScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if expiresAt < 0 || (expiresAt != 0 && expiresAt <= nowMilli()) {
		return nil, common.NewError("expiresAt must be 0 (never) or a future unix-ms timestamp")
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(model.ApiToken{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewError("a token with that name already exists")
	}
	plaintext := random.Seq(apiTokenLength)
	row := &model.ApiToken{
		Name:      name,
		Token:     crypto.HashTokenSHA256(plaintext),
		Enabled:   true,
		Scope:     normScope,
		ExpiresAt: expiresAt,
	}
	if err := db.Create(row).Error; err != nil {
		return nil, err
	}
	view := toView(row)
	view.Token = plaintext
	return view, nil
}

// RecreateByName replaces any token with this name, keeping exactly one so a
// repeatedly-run caller cannot accumulate credentials it can never revoke.
func (s *ApiTokenService) RecreateByName(name string) (*ApiTokenView, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, common.NewError("token name is required")
	}
	// Same column, same limit as Create: the CLI now feeds this operator input.
	if len(name) > 64 {
		return nil, common.NewError("token name must be 64 characters or fewer")
	}
	plaintext := random.Seq(apiTokenLength)
	row := &model.ApiToken{Name: name, Token: crypto.HashTokenSHA256(plaintext), Enabled: true}
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("name = ?", name).Delete(model.ApiToken{}).Error; err != nil {
			return err
		}
		return tx.Create(row).Error
	}); err != nil {
		return nil, err
	}
	view := toView(row)
	view.Token = plaintext
	return view, nil
}

func (s *ApiTokenService) Delete(id int) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	db := database.GetDB()
	return db.Where("id = ?", id).Delete(model.ApiToken{}).Error
}

func (s *ApiTokenService) DeleteExpectedScope(id int, expectedScope string) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	scope, err := requireExpectedScope(expectedScope)
	if err != nil {
		return err
	}
	res := database.GetDB().Where("id = ? AND scope = ?", id, scope).Delete(model.ApiToken{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("token not found with expected scope")
	}
	return nil
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

func (s *ApiTokenService) SetEnabledExpectedScope(id int, expectedScope string, enabled bool) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	scope, err := requireExpectedScope(expectedScope)
	if err != nil {
		return err
	}
	res := database.GetDB().Model(model.ApiToken{}).Where("id = ? AND scope = ?", id, scope).Update("enabled", enabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("token not found with expected scope")
	}
	return nil
}

func nowMilli() int64 { return time.Now().UnixMilli() }

// DisableExpectedScope fails closed unless the stored scope matches the caller,
// preventing rotation from revoking a newly minted token after a wrong ID.
func (s *ApiTokenService) DisableExpectedScope(id int, expectedScope string) error {
	if id <= 0 {
		return common.NewError("invalid token id")
	}
	return s.SetEnabledExpectedScope(id, expectedScope, false)
}

func requireExpectedScope(expectedScope string) (string, error) {
	if strings.TrimSpace(expectedScope) == "" {
		return "", common.NewError("expected scope is required")
	}
	scope, err := NormalizeScope(expectedScope)
	if err != nil {
		return "", err
	}
	return scope, nil
}

// MatchToken returns the enabled, non-expired api_token row whose stored
// SHA-256 hash matches the presented bearer value, or (nil,false). The loop
// scans every enabled row with constant-time compares, then applies expiry and
// scope checks to avoid treating corrupt values as admin.
func (s *ApiTokenService) MatchToken(presented string) (*model.ApiToken, bool) {
	if presented == "" {
		return nil, false
	}
	db := database.GetDB()
	var rows []*model.ApiToken
	if err := db.Model(model.ApiToken{}).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, false
	}
	presentedHash := []byte(crypto.HashTokenSHA256(presented))
	var matched *model.ApiToken
	for _, r := range rows {
		if subtle.ConstantTimeCompare([]byte(r.Token), presentedHash) == 1 {
			matched = r
		}
	}
	if matched == nil {
		return nil, false
	}
	if !model.IsKnownApiScope(matched.Scope) {
		return nil, false
	}
	if matched.ExpiresAt != 0 && nowMilli() >= matched.ExpiresAt {
		return nil, false
	}
	return matched, true
}

// Match is the legacy boolean form for callers that do not need scope.
func (s *ApiTokenService) Match(presented string) bool {
	_, ok := s.MatchToken(presented)
	return ok
}
