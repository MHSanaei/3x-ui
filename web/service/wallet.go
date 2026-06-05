package service

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"gorm.io/gorm"
)

// Wallet errors. Sentinels so callers can branch (e.g. the client-create path
// maps ErrInsufficientBalance to a user-facing "Insufficient balance").
var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrBalanceConflict     = errors.New("balance changed concurrently, retry")
)

// WalletService owns balance mutations and the auditable transaction log. Every
// balance change is recorded as a model.Transaction with the before/after
// snapshot, and every change is applied inside a DB transaction using a
// compare-and-swap update so concurrent debits cannot oversell a balance.
type WalletService struct{}

const walletMaxRetries = 4

// GetBalance returns the current balance for a user.
func (s *WalletService) GetBalance(userId int) (int64, error) {
	var u model.User
	if err := database.GetDB().Select("balance").Where("id = ?", userId).First(&u).Error; err != nil {
		return 0, err
	}
	return u.Balance, nil
}

// applyDelta mutates the balance by delta (signed) inside tx and writes the
// matching transaction row. delta>0 records a credit, delta<0 a debit. A debit
// that would drive the balance negative returns ErrInsufficientBalance. The
// balance update is a compare-and-swap (WHERE balance = before); a 0-row result
// means another writer moved the balance first and surfaces as
// ErrBalanceConflict for the retry wrapper.
func (s *WalletService) applyDelta(tx *gorm.DB, userId int, delta int64, txType, desc string) (*model.Transaction, error) {
	var u model.User
	if err := tx.Where("id = ?", userId).First(&u).Error; err != nil {
		return nil, err
	}
	before := u.Balance
	after := before + delta
	if after < 0 {
		return nil, ErrInsufficientBalance
	}
	res := tx.Model(&model.User{}).
		Where("id = ? AND balance = ?", userId, before).
		Update("balance", after)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrBalanceConflict
	}
	amount := delta
	if amount < 0 {
		amount = -amount
	}
	rec := &model.Transaction{
		UserId:        userId,
		Amount:        amount,
		Type:          txType,
		Description:   desc,
		BalanceBefore: before,
		BalanceAfter:  after,
	}
	if err := tx.Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

// withRetry runs fn inside a fresh DB transaction, retrying a bounded number of
// times when applyDelta reports a compare-and-swap conflict.
func (s *WalletService) withRetry(fn func(tx *gorm.DB) error) error {
	var err error
	for i := 0; i < walletMaxRetries; i++ {
		err = database.GetDB().Transaction(fn)
		if !errors.Is(err, ErrBalanceConflict) {
			return err
		}
	}
	return err
}

// Credit adds amount (>0) to a user's balance and records a credit transaction.
func (s *WalletService) Credit(userId int, amount int64, desc string) (*model.Transaction, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	var rec *model.Transaction
	err := s.withRetry(func(tx *gorm.DB) error {
		r, e := s.applyDelta(tx, userId, amount, model.TxCredit, desc)
		rec = r
		return e
	})
	return rec, err
}

// Debit subtracts amount (>0) from a user's balance, recording a debit
// transaction. Returns ErrInsufficientBalance when the balance is too low.
func (s *WalletService) Debit(userId int, amount int64, desc string) (*model.Transaction, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	var rec *model.Transaction
	err := s.withRetry(func(tx *gorm.DB) error {
		r, e := s.applyDelta(tx, userId, -amount, model.TxDebit, desc)
		rec = r
		return e
	})
	return rec, err
}

// SetBalance forces a user's balance to target (>=0), recording the difference
// as a credit or debit. A no-op (target == current) records nothing.
func (s *WalletService) SetBalance(userId int, target int64, desc string) (*model.Transaction, error) {
	if target < 0 {
		return nil, ErrInvalidAmount
	}
	var rec *model.Transaction
	err := s.withRetry(func(tx *gorm.DB) error {
		var u model.User
		if e := tx.Where("id = ?", userId).First(&u).Error; e != nil {
			return e
		}
		delta := target - u.Balance
		if delta == 0 {
			rec = nil
			return nil
		}
		txType := model.TxCredit
		if delta < 0 {
			txType = model.TxDebit
		}
		r, e := s.applyDelta(tx, userId, delta, txType, desc)
		rec = r
		return e
	})
	return rec, err
}

// ListTransactions returns the wallet history. When userId is non-nil it is
// scoped to that user; otherwise all transactions are returned (admin view).
// Results are newest-first and capped by limit/offset.
func (s *WalletService) ListTransactions(userId *int, limit, offset int) ([]model.Transaction, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := database.GetDB().Model(&model.Transaction{}).Order("id desc").Limit(limit).Offset(offset)
	if userId != nil {
		q = q.Where("user_id = ?", *userId)
	}
	var rows []model.Transaction
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
