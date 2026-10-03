package telegramauth

import (
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func testService(t *testing.T) (*Service, *gorm.DB, *time.Time) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)
	s := NewService()
	s.db = func() *gorm.DB { return db }
	s.now = func() time.Time { return now }
	return s, db, &now
}

func createUser(t *testing.T, db *gorm.DB) model.User {
	t.Helper()
	user := model.User{Username: "admin"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func TestLinkIsUniqueAndSingleUse(t *testing.T) {
	s, db, _ := testService(t)
	first := createUser(t, db)
	second := createUser(t, db)
	code, _, err := s.StartLink(first.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(code, 123); err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(code, 456); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("reused link code: %v", err)
	}
	code, _, err = s.StartLink(second.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(code, 123); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("duplicate telegram binding: %v", err)
	}
	if err := s.Unlink(first.Id); err != nil {
		t.Fatal(err)
	}
	code, _, err = s.StartLink(second.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(code, 123); err != nil {
		t.Fatal(err)
	}
}

func TestLoginRequiresMatchingTelegramAndCSRF(t *testing.T) {
	s, db, _ := testService(t)
	user := createUser(t, db)
	link, _, err := s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(link, 123); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.StartLogin("browser-secret", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if got, pending, err := s.CompleteLogin(code, "browser-secret"); err != nil || !pending || got != nil {
		t.Fatalf("pending login = %v, %v, %v", got, pending, err)
	}
	if err := s.ApproveLogin(code, 123); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("login approved without bot confirmation: %v", err)
	}
	if _, err := s.PrepareLogin(code, 456); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unlinked telegram account prepared login: %v", err)
	}
	if ip, err := s.PrepareLogin(code, 123); err != nil || ip != "192.0.2.1" {
		t.Fatalf("prepared login = %q, %v", ip, err)
	}
	if err := s.ApproveLogin(code, 456); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("wrong telegram account approved: %v", err)
	}
	if err := s.ApproveLogin(code, 123); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CompleteLogin(code, "other-browser"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("wrong csrf completed login: %v", err)
	}
	got, pending, err := s.CompleteLogin(code, "browser-secret")
	if err != nil || pending || got == nil || got.Id != user.Id {
		t.Fatalf("completed login = %v, %v, %v", got, pending, err)
	}
	if _, _, err := s.CompleteLogin(code, "browser-secret"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("reused login code: %v", err)
	}
}

func TestLoginRechecksBindingAndExpiry(t *testing.T) {
	s, db, now := testService(t)
	user := createUser(t, db)
	link, _, err := s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(codeTTL)
	if err := s.ConsumeLink(link, 123); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired link accepted: %v", err)
	}
	link, _, err = s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(link, 123); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.StartLogin("browser", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareLogin(code, 123); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveLogin(code, 123); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("telegram_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CompleteLogin(code, "browser"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("changed binding logged in: %v", err)
	}
	link, _, err = s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(link, 123); err != nil {
		t.Fatal(err)
	}
	code, _, err = s.StartLogin("browser", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(codeTTL)
	if _, err := s.PrepareLogin(code, 123); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired login prepared: %v", err)
	}
	if err := s.ApproveLogin(code, 123); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expired login approved: %v", err)
	}
}

func TestPublicLoginCapacityDoesNotBlockLinksOrNewLogins(t *testing.T) {
	s, db, now := testService(t)
	user := createUser(t, db)
	link, _, err := s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(link, 123); err != nil {
		t.Fatal(err)
	}
	first := ""
	second := ""
	for i := range maxPendingLogins {
		*now = now.Add(time.Nanosecond)
		code, _, err := s.StartLogin("browser-"+strconv.Itoa(i), "192.0.2.1")
		if err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
		switch i {
		case 0:
			first = code
		case 1:
			second = code
		}
	}
	if _, _, err := s.StartLogin("browser-1", "192.0.2.1"); err != nil {
		t.Fatalf("same browser could not refresh at capacity: %v", err)
	}
	if _, err := s.PrepareLogin(second, 123); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("replaced browser request remained valid: %v", err)
	}
	if _, _, err := s.StartLogin("new-browser", "192.0.2.2"); err != nil {
		t.Fatalf("new login rejected at capacity: %v", err)
	}
	if _, err := s.PrepareLogin(first, 123); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("oldest public request was not evicted: %v", err)
	}
	if err := s.Unlink(user.Id); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.StartLink(user.Id); err != nil {
		t.Fatalf("public login flood blocked authenticated link: %v", err)
	}
}

func TestPublicFloodCannotEvictBotConfirmation(t *testing.T) {
	s, db, now := testService(t)
	user := createUser(t, db)
	link, _, err := s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(link, 123); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.StartLogin("real-browser", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareLogin(code, 123); err != nil {
		t.Fatal(err)
	}
	for i := range maxPendingLogins {
		*now = now.Add(time.Nanosecond)
		if _, _, err := s.StartLogin("attacker-"+strconv.Itoa(i), "198.51.100.1"); err != nil {
			t.Fatalf("public start %d: %v", i, err)
		}
	}
	if err := s.ApproveLogin(code, 123); err != nil {
		t.Fatalf("public flood evicted bot confirmation: %v", err)
	}
	got, pending, err := s.CompleteLogin(code, "real-browser")
	if err != nil || pending || got == nil || got.Id != user.Id {
		t.Fatalf("completed login after flood = %v, %v, %v", got, pending, err)
	}
}

func TestLinkRejectedAfterLoginEpochChanges(t *testing.T) {
	s, db, _ := testService(t)
	user := createUser(t, db)
	code, _, err := s.StartLink(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).
		Update("login_epoch", gorm.Expr("login_epoch + 1")).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.ConsumeLink(code, 123); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("link survived account change: %v", err)
	}
	var stored model.User
	if err := db.First(&stored, user.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TelegramID != 0 {
		t.Fatalf("unexpected telegram binding: %d", stored.TelegramID)
	}
}
