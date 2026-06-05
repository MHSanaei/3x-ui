package service

import (
	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// PaymentService persists payment-gateway attempts and flips their status
// idempotently so a balance is credited at most once per payment.
type PaymentService struct{}

// CreatePending records a new pending payment for a user/authority.
func (s *PaymentService) CreatePending(userId int, gateway, authority string, amount int64) (*model.Payment, error) {
	p := &model.Payment{
		UserId:    userId,
		Gateway:   gateway,
		Authority: authority,
		Amount:    amount,
		Status:    model.PaymentPending,
	}
	if err := database.GetDB().Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// GetByAuthority loads a payment by its gateway authority.
func (s *PaymentService) GetByAuthority(authority string) (*model.Payment, error) {
	var p model.Payment
	if err := database.GetDB().Where("authority = ?", authority).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// MarkPaid atomically transitions a payment from pending to paid. It returns
// true only for the call that performed the transition (so exactly one caller
// credits the wallet); concurrent/duplicate callbacks see false.
func (s *PaymentService) MarkPaid(authority, refID string) (bool, *model.Payment, error) {
	db := database.GetDB()
	var p model.Payment
	if err := db.Where("authority = ?", authority).First(&p).Error; err != nil {
		return false, nil, err
	}
	res := db.Model(&model.Payment{}).
		Where("authority = ? AND status = ?", authority, model.PaymentPending).
		Updates(map[string]any{"status": model.PaymentPaid, "ref_id": refID})
	if res.Error != nil {
		return false, &p, res.Error
	}
	p.Status = model.PaymentPaid
	p.RefId = refID
	return res.RowsAffected == 1, &p, nil
}

// MarkFailed flips a still-pending payment to failed (best effort).
func (s *PaymentService) MarkFailed(authority string) error {
	return database.GetDB().Model(&model.Payment{}).
		Where("authority = ? AND status = ?", authority, model.PaymentPending).
		Update("status", model.PaymentFailed).Error
}

// ListForUser returns a user's payment history, newest first.
func (s *PaymentService) ListForUser(userId, limit, offset int) ([]model.Payment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var rows []model.Payment
	err := database.GetDB().Where("user_id = ?", userId).
		Order("id desc").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, err
}
