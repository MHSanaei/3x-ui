package telegramauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	codeTTL          = 5 * time.Minute
	maxPendingLinks  = 1024
	maxPendingLogins = 1024
)

var (
	ErrInvalidCode = errors.New("invalid or expired telegram code")
	ErrUnavailable = errors.New("telegram authentication unavailable")
	ErrTooMany     = errors.New("too many pending telegram requests")
	Default        = NewService()
)

type linkRequest struct {
	userID     int
	loginEpoch int64
	expiresAt  time.Time
}

type loginRequest struct {
	csrf       string
	ip         string
	expiresAt  time.Time
	userID     int
	telegramID int64
}

// Service holds short-lived, single-use Telegram authentication requests.
type Service struct {
	mu     sync.Mutex
	links  map[string]linkRequest
	logins map[string]loginRequest
	db     func() *gorm.DB
	now    func() time.Time
}

func NewService() *Service {
	return &Service{
		links:  make(map[string]linkRequest),
		logins: make(map[string]loginRequest),
		db:     database.GetDB,
		now:    time.Now,
	}
}

func (s *Service) StartLink(userID int) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var user model.User
	if userID <= 0 || s.db().First(&user, userID).Error != nil || user.TelegramID != 0 {
		return "", time.Time{}, ErrUnavailable
	}
	code, err := newCode()
	if err != nil {
		return "", time.Time{}, err
	}
	s.prune()
	for existing, request := range s.links {
		if request.userID == userID {
			delete(s.links, existing)
		}
	}
	if len(s.links) >= maxPendingLinks {
		return "", time.Time{}, ErrTooMany
	}
	expiresAt := s.now().Add(codeTTL)
	s.links[code] = linkRequest{userID: userID, loginEpoch: user.LoginEpoch, expiresAt: expiresAt}
	return code, expiresAt, nil
}

func (s *Service) ConsumeLink(code string, telegramUserID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.links[code]
	if !ok || telegramUserID <= 0 || !s.now().Before(request.expiresAt) {
		delete(s.links, code)
		return ErrInvalidCode
	}
	err := s.db().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.User{}).Where("telegram_id = ?", telegramUserID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrUnavailable
		}
		result := tx.Model(&model.User{}).
			Where("id = ? AND telegram_id = 0 AND login_epoch = ?", request.userID, request.loginEpoch).
			Update("telegram_id", telegramUserID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrUnavailable
		}
		return nil
	})
	if err == nil || errors.Is(err, ErrUnavailable) {
		delete(s.links, code)
	}
	return err
}

func (s *Service) StartLogin(csrf, ip string) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if csrf == "" {
		return "", time.Time{}, ErrUnavailable
	}
	var count int64
	if err := s.db().Model(&model.User{}).Where("telegram_id <> 0").Count(&count).Error; err != nil || count == 0 {
		return "", time.Time{}, ErrUnavailable
	}
	code, err := newCode()
	if err != nil {
		return "", time.Time{}, err
	}
	s.prune()
	for existing, request := range s.logins {
		if request.csrf == csrf {
			delete(s.logins, existing)
		}
	}
	if len(s.logins) >= maxPendingLogins {
		oldestCode := ""
		var oldestExpiry time.Time
		for existing, request := range s.logins {
			if request.telegramID == 0 && (oldestCode == "" || request.expiresAt.Before(oldestExpiry)) {
				oldestCode, oldestExpiry = existing, request.expiresAt
			}
		}
		if oldestCode == "" {
			return "", time.Time{}, ErrTooMany
		}
		delete(s.logins, oldestCode)
	}
	expiresAt := s.now().Add(codeTTL)
	s.logins[code] = loginRequest{csrf: csrf, ip: ip, expiresAt: expiresAt}
	return code, expiresAt, nil
}

// PrepareLogin records who received the approval prompt without approving the browser.
func (s *Service) PrepareLogin(code string, telegramUserID int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.logins[code]
	if !ok || telegramUserID <= 0 || !s.now().Before(request.expiresAt) || request.userID != 0 {
		return "", ErrInvalidCode
	}
	if request.telegramID != 0 && request.telegramID != telegramUserID {
		return "", ErrUnavailable
	}
	var count int64
	if err := s.db().Model(&model.User{}).Where("telegram_id = ?", telegramUserID).Count(&count).Error; err != nil {
		return "", err
	}
	if count != 1 {
		return "", ErrUnavailable
	}
	request.telegramID = telegramUserID
	s.logins[code] = request
	return request.ip, nil
}

func (s *Service) ApproveLogin(code string, telegramUserID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.logins[code]
	if !ok || telegramUserID <= 0 || !s.now().Before(request.expiresAt) || request.userID != 0 || request.telegramID != telegramUserID {
		return ErrInvalidCode
	}
	var users []model.User
	if err := s.db().Where("telegram_id = ?", telegramUserID).Limit(2).Find(&users).Error; err != nil {
		return err
	}
	if len(users) != 1 {
		return ErrUnavailable
	}
	request.userID = users[0].Id
	request.telegramID = telegramUserID
	s.logins[code] = request
	return nil
}

func (s *Service) RejectLogin(code string, telegramUserID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.logins[code]
	if !ok || !s.now().Before(request.expiresAt) || request.userID != 0 || request.telegramID != telegramUserID {
		return ErrInvalidCode
	}
	delete(s.logins, code)
	return nil
}

// CompleteLogin returns pending=true until Telegram approves the request.
func (s *Service) CompleteLogin(code, csrf string) (*model.User, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.logins[code]
	if !ok || !s.now().Before(request.expiresAt) || subtle.ConstantTimeCompare([]byte(request.csrf), []byte(csrf)) != 1 {
		return nil, false, ErrInvalidCode
	}
	if request.userID == 0 {
		return nil, true, nil
	}
	delete(s.logins, code)
	var user model.User
	err := s.db().Where("id = ? AND telegram_id = ?", request.userID, request.telegramID).First(&user).Error
	if err != nil {
		return nil, false, ErrInvalidCode
	}
	return &user, false, nil
}

func (s *Service) Unlink(userID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID <= 0 {
		return ErrUnavailable
	}
	result := s.db().Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"telegram_id": 0,
		"login_epoch": gorm.Expr("login_epoch + 1"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrUnavailable
	}
	for code, request := range s.links {
		if request.userID == userID {
			delete(s.links, code)
		}
	}
	for code, request := range s.logins {
		if request.userID == userID {
			delete(s.logins, code)
		}
	}
	return nil
}

func (s *Service) prune() {
	now := s.now()
	for code, request := range s.links {
		if !now.Before(request.expiresAt) {
			delete(s.links, code)
		}
	}
	for code, request := range s.logins {
		if !now.Before(request.expiresAt) {
			delete(s.logins, code)
		}
	}
}

func newCode() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}
