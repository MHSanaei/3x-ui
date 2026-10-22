package panel

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

// ResellerService manages reseller accounts: creation, credentials, traffic
// and client-count quotas, and the inbounds an admin assigns to each reseller.
//
// A reseller is a regular panel user (Role == "reseller") whose ParentId is
// the admin that created them. Resellers log in with the same
// username/password flow as admins, but the client/inbound APIs scope them to
// their own clients and the inbounds the admin assigned to them. Traffic of a
// reseller's clients is tracked separately from the admin's clients through
// ClientRecord.UserId.
type ResellerService struct{}

// ResellerCreateInput carries the fields the admin sets when creating a
// reseller from the web panel.
type ResellerCreateInput struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	TotalGB   int64  `json:"totalGB"`   // traffic quota in GB the reseller may allocate (0 = unlimited)
	MaxClients int   `json:"maxClients"` // max clients the reseller may create (0 = unlimited)
}

// ResellerUpdateInput carries updatable reseller fields.
type ResellerUpdateInput struct {
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"` // empty means unchanged
	TotalGB   int64  `json:"totalGB"`
	MaxClients int   `json:"maxClients"`
	Enable    *bool  `json:"enable,omitempty"`
}

// ResellerUsage reports a reseller's current consumption.
type ResellerUsage struct {
	ClientCount int
	UsedGB      int64 // traffic (bytes) consumed by the reseller's clients
}

// ResellerRow is a reseller account with its current usage, safe to expose to
// the admin UI (the hashed password is deliberately omitted).
type ResellerRow struct {
	ID         int    `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	ParentId   int    `json:"parentId"`
	TotalGB    int64  `json:"totalGB"`
	UsedGB     int64  `json:"usedGB"`
	MaxClients int    `json:"maxClients"`
	Enable     bool   `json:"enable"`
	Usage      ResellerUsage `json:"usage"`
}

// CreateReseller creates a new reseller account under the given parent (admin)
// with the supplied traffic quota and client limit.
func (s *ResellerService) CreateReseller(parentId int, input ResellerCreateInput) (*model.User, error) {
	if input.Username == "" {
		return nil, errors.New("username is required")
	}
	if input.Password == "" {
		return nil, errors.New("password is required")
	}

	hashedPassword, err := crypto.HashPasswordAsBcrypt(input.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:   input.Username,
		Password:   hashedPassword,
		Role:       string(model.UserRoleReseller),
		ParentId:   parentId,
		TotalGB:    input.TotalGB,
		MaxClients: input.MaxClients,
		Enable:     true,
	}

	if err := database.GetDB().Create(user).Error; err != nil {
		return nil, err
	}
	// Never echo the hashed password back to the caller.
	user.Password = ""
	return user, nil
}

// ListResellers returns all resellers owned by parentId, with their current
// usage computed from client traffic.
func (s *ResellerService) ListResellers(parentId int) ([]ResellerRow, error) {
	db := database.GetDB()
	var resellers []*model.User
	if err := db.Model(&model.User{}).
		Where("role = ? AND parent_id = ?", string(model.UserRoleReseller), parentId).
		Order("id ASC").
		Find(&resellers).Error; err != nil {
		return nil, err
	}
	rows := make([]ResellerRow, 0, len(resellers))
	for _, u := range resellers {
		rows = append(rows, ResellerRow{
			ID:         u.Id,
			Username:   u.Username,
			Role:       u.Role,
			ParentId:   u.ParentId,
			TotalGB:    u.TotalGB,
			UsedGB:     u.UsedGB,
			MaxClients: u.MaxClients,
			Enable:     u.Enable,
			Usage:      s.UsageFor(u.Id),
		})
	}
	return rows, nil
}

// GetReseller returns the reseller with the given id, or an error if it does
// not exist or is not a reseller owned by parentId (when parentId > 0).
func (s *ResellerService) GetReseller(id int, parentId int) (*model.User, error) {
	db := database.GetDB()
	user := &model.User{}
	q := db.Model(&model.User{}).Where("id = ? AND role = ?", id, string(model.UserRoleReseller))
	if parentId > 0 {
		q = q.Where("parent_id = ?", parentId)
	}
	if err := q.First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateReseller updates a reseller's credentials, quotas and enabled state.
func (s *ResellerService) UpdateReseller(id int, parentId int, input ResellerUpdateInput) error {
	user, err := s.GetReseller(id, parentId)
	if err != nil {
		return err
	}

	updates := map[string]any{
		"username":   input.Username,
		"total_gb":   input.TotalGB,
		"max_clients": input.MaxClients,
	}
	if input.Enable != nil {
		updates["enable"] = *input.Enable
	}
	if input.Password != "" {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(input.Password)
		if err != nil {
			return err
		}
		updates["password"] = hashedPassword
	}
	updates["login_epoch"] = gorm.Expr("login_epoch + 1")

	return database.GetDB().Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteReseller removes a reseller account. It refuses to delete a reseller
// that still has clients, so the admin must reassign or delete them first.
func (s *ResellerService) DeleteReseller(id int, parentId int) error {
	user, err := s.GetReseller(id, parentId)
	if err != nil {
		return err
	}

	var count int64
	if err := database.GetDB().Model(&model.ClientRecord{}).
		Where("user_id = ?", user.Id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("reseller still has clients; remove them before deleting the reseller")
	}

	// Drop the inbound assignments for this reseller.
	if err := database.GetDB().
		Where("reseller_id = ?", user.Id).
		Delete(&model.ResellerInbound{}).Error; err != nil {
		return err
	}
	return database.GetDB().Where("id = ?", user.Id).Delete(&model.User{}).Error
}

// AssignInbounds replaces the set of inbounds a reseller may use. Every id
// must refer to an existing inbound.
func (s *ResellerService) AssignInbounds(resellerId int, inboundIds []int) error {
	if len(inboundIds) == 0 {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			return tx.Where("reseller_id = ?", resellerId).
				Delete(&model.ResellerInbound{}).Error
		})
	}
	var count int64
	if err := database.GetDB().Model(&model.Inbound{}).
		Where("id IN ?", inboundIds).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(inboundIds) {
		return errors.New("one or more inbounds do not exist")
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("reseller_id = ?", resellerId).
			Delete(&model.ResellerInbound{}).Error; err != nil {
			return err
		}
		rows := make([]model.ResellerInbound, 0, len(inboundIds))
		for _, ibId := range inboundIds {
			rows = append(rows, model.ResellerInbound{ResellerId: resellerId, InboundId: ibId})
		}
		return tx.CreateInBatches(rows, 200).Error
	})
}

// AssignedInbounds returns the inbound ids a reseller is allowed to create
// clients on.
func (s *ResellerService) AssignedInbounds(resellerId int) ([]int, error) {
	var ids []int
	if err := database.GetDB().Model(&model.ResellerInbound{}).
		Where("reseller_id = ?", resellerId).
		Pluck("inbound_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UsageFor computes a reseller's current traffic consumption (bytes) and
// client count from the clients they own.
func (s *ResellerService) UsageFor(resellerId int) ResellerUsage {
	var usage ResellerUsage
	db := database.GetDB()
	db.Model(&model.ClientRecord{}).Where("user_id = ?", resellerId).Count(&usage.ClientCount)

	var total int64
	db.Table("clients c").
		Select("COALESCE(SUM(ct.up + ct.down), 0)").
		Joins("LEFT JOIN client_traffics ct ON ct.email = c.email").
		Where("c.user_id = ?", resellerId).
		Scan(&total)
	usage.UsedGB = total
	return usage
}

// RemainingGB returns how many more GB the reseller may allocate. 0 means
// unlimited if the reseller has no quota.
func (s *ResellerService) RemainingGB(resellerId int) (int64, error) {
	user, err := s.GetReseller(resellerId, 0)
	if err != nil {
		return 0, err
	}
	if user.TotalGB <= 0 {
		return 0, nil // unlimited
	}
	usage := s.UsageFor(resellerId)
	usedGB := usage.UsedGB / (1024 * 1024 * 1024)
	if usedGB >= user.TotalGB {
		return 0, nil
	}
	return user.TotalGB - usedGB, nil
}

// CanCreateClient reports whether the reseller may create another client of
// the given size (GB), and a human-readable reason if not.
func (s *ResellerService) CanCreateClient(resellerId int, clientTotalGB int64) (bool, string) {
	user, err := s.GetReseller(resellerId, 0)
	if err != nil {
		return false, "reseller not found"
	}
	if !user.Enable {
		return false, "reseller account is disabled"
	}
	if user.MaxClients > 0 {
		usage := s.UsageFor(resellerId)
		if usage.ClientCount >= user.MaxClients {
			return false, "client limit reached"
		}
	}
	if user.TotalGB > 0 {
		remaining, err := s.RemainingGB(resellerId)
		if err != nil {
			return false, "could not compute quota"
		}
		if clientTotalGB > 0 && clientTotalGB > remaining {
			return false, "traffic quota exceeded"
		}
	}
	return true, ""
}

// MarkClientCreated is a no-op hook kept for callers that want to record a
// client creation against a reseller; usage is computed on demand so nothing
// needs to be incremented here.
func (s *ResellerService) MarkClientCreated(resellerId int) error {
	return nil
}
