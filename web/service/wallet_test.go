package service

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func newTestUser(t *testing.T, us *UserService, username string, balance int64) *model.User {
	t.Helper()
	u, err := us.AdminCreateUser(AdminUserInput{
		Username: username,
		Password: "Sup3rSecret",
		Role:     model.RoleUser,
		Balance:  balance,
	})
	if err != nil {
		t.Fatalf("AdminCreateUser failed: %v", err)
	}
	return u
}

func TestWalletCreditDebitRecordsTransactions(t *testing.T) {
	setupUserTestDB(t)
	us := &UserService{}
	ws := &WalletService{}
	u := newTestUser(t, us, "wallet_user", 100)

	if _, err := ws.Debit(u.Id, 30, "buy"); err != nil {
		t.Fatalf("Debit failed: %v", err)
	}
	bal, err := ws.GetBalance(u.Id)
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}
	if bal != 70 {
		t.Fatalf("expected balance 70 after debit, got %d", bal)
	}

	if _, err := ws.Credit(u.Id, 50, "topup"); err != nil {
		t.Fatalf("Credit failed: %v", err)
	}
	bal, _ = ws.GetBalance(u.Id)
	if bal != 120 {
		t.Fatalf("expected balance 120 after credit, got %d", bal)
	}

	txs, err := ws.ListTransactions(&u.Id, 100, 0)
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}
	if len(txs) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txs))
	}
	// Newest first: the credit.
	if txs[0].Type != model.TxCredit || txs[0].Amount != 50 || txs[0].BalanceBefore != 70 || txs[0].BalanceAfter != 120 {
		t.Fatalf("unexpected credit transaction: %+v", txs[0])
	}
	if txs[1].Type != model.TxDebit || txs[1].Amount != 30 || txs[1].BalanceBefore != 100 || txs[1].BalanceAfter != 70 {
		t.Fatalf("unexpected debit transaction: %+v", txs[1])
	}
}

func TestWalletDebitRejectsInsufficientBalance(t *testing.T) {
	setupUserTestDB(t)
	us := &UserService{}
	ws := &WalletService{}
	u := newTestUser(t, us, "poor_user", 10)

	if _, err := ws.Debit(u.Id, 25, "buy"); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
	bal, _ := ws.GetBalance(u.Id)
	if bal != 10 {
		t.Fatalf("balance must be unchanged after a rejected debit, got %d", bal)
	}
	txs, _ := ws.ListTransactions(&u.Id, 100, 0)
	if len(txs) != 0 {
		t.Fatalf("a rejected debit must record no transaction, got %d", len(txs))
	}
}

func TestWalletSetBalanceRecordsDelta(t *testing.T) {
	setupUserTestDB(t)
	us := &UserService{}
	ws := &WalletService{}
	u := newTestUser(t, us, "set_user", 40)

	rec, err := ws.SetBalance(u.Id, 100, "admin set")
	if err != nil {
		t.Fatalf("SetBalance failed: %v", err)
	}
	if rec == nil || rec.Type != model.TxCredit || rec.Amount != 60 {
		t.Fatalf("expected a credit of 60, got %+v", rec)
	}
	rec, err = ws.SetBalance(u.Id, 25, "admin set down")
	if err != nil {
		t.Fatalf("SetBalance down failed: %v", err)
	}
	if rec == nil || rec.Type != model.TxDebit || rec.Amount != 75 {
		t.Fatalf("expected a debit of 75, got %+v", rec)
	}
	bal, _ := ws.GetBalance(u.Id)
	if bal != 25 {
		t.Fatalf("expected balance 25, got %d", bal)
	}
}
