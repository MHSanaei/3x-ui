package service

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/crypto"
	ldaputil "github.com/mhsanaei/3x-ui/v3/util/ldap"
	"github.com/xlzd/gotp"
	"gorm.io/gorm"
)

// Registration errors. They are sentinel values so the controller can map each
// failure to a localized, user-facing message without string matching.
var (
	ErrUsernameTaken   = errors.New("username already taken")
	ErrEmailTaken      = errors.New("email already registered")
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPhone    = errors.New("invalid phone number")
	ErrInvalidFullName = errors.New("invalid full name")
	ErrWeakPassword    = errors.New("password does not meet the strength requirements")
)

var (
	usernameRegex = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)
	// E.164-ish: optional leading +, then digits and common separators.
	phoneRegex = regexp.MustCompile(`^\+?[0-9][0-9 ()\-.]{4,19}$`)
)

// RegisterInput carries the already-trimmed fields captured by the registration
// form. The controller is responsible for trimming and confirm-password
// matching; the service re-validates defensively and owns uniqueness + persistence.
type RegisterInput struct {
	FullName string
	Phone    string
	Email    string
	Username string
	Password string
}

// ValidatePasswordStrength enforces the shared password policy: at least 8
// characters with a mix of upper-case, lower-case and digit. Mirrored in the
// frontend zod schema so client and server agree.
func ValidatePasswordStrength(password string) error {
	if len([]rune(password)) < 8 {
		return ErrWeakPassword
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}

// normalizeRegisterInput trims whitespace and lower-cases the fields used for
// case-insensitive uniqueness so "Admin" and "admin" can't both register.
func normalizeRegisterInput(in RegisterInput) RegisterInput {
	in.FullName = strings.TrimSpace(in.FullName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Username = strings.TrimSpace(in.Username)
	// Password is intentionally left untouched — leading/trailing characters
	// are significant.
	return in
}

func validateRegisterInput(in RegisterInput) error {
	if n := len([]rune(in.FullName)); n < 2 || n > 100 {
		return ErrInvalidFullName
	}
	if !usernameRegex.MatchString(in.Username) {
		return ErrInvalidUsername
	}
	if !phoneRegex.MatchString(in.Phone) {
		return ErrInvalidPhone
	}
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email || len(in.Email) > 254 {
		return ErrInvalidEmail
	}
	return ValidatePasswordStrength(in.Password)
}

// Register validates the input, guarantees the username and email are unique
// (case-insensitively), securely hashes the password with bcrypt and persists
// the new panel user inside a single transaction. The returned user has its
// password field cleared so callers can safely serialize it.
func (s *UserService) Register(input RegisterInput) (*model.User, error) {
	in := normalizeRegisterInput(input)
	if err := validateRegisterInput(in); err != nil {
		return nil, err
	}

	hashedPassword, err := crypto.HashPasswordAsBcrypt(in.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: in.Username,
		Password: hashedPassword,
		FullName: in.FullName,
		Phone:    in.Phone,
		Email:    in.Email,
	}

	db := database.GetDB()
	err = db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(model.User{}).Where("LOWER(username) = ?", strings.ToLower(in.Username)).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrUsernameTaken
		}
		if err := tx.Model(model.User{}).Where("LOWER(email) = ?", in.Email).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrEmailTaken
		}
		return tx.Create(user).Error
	})
	if err != nil {
		return nil, err
	}

	user.Password = ""
	return user, nil
}

// UserService provides business logic for user management and authentication.
// It handles user creation, login, password management, and 2FA operations.
type UserService struct {
	settingService SettingService
}

// GetFirstUser retrieves the first user from the database.
// This is typically used for initial setup or when there's only one admin user.
func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string, twoFactorCode string) (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}

	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil, errors.New("invalid credentials")
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil, err
	}

	if !crypto.CheckPasswordHash(user.Password, password) {
		ldapEnabled, _ := s.settingService.GetLdapEnable()
		if !ldapEnabled {
			return nil, errors.New("invalid credentials")
		}

		host, _ := s.settingService.GetLdapHost()
		port, _ := s.settingService.GetLdapPort()
		useTLS, _ := s.settingService.GetLdapUseTLS()
		bindDN, _ := s.settingService.GetLdapBindDN()
		ldapPass, _ := s.settingService.GetLdapPassword()
		baseDN, _ := s.settingService.GetLdapBaseDN()
		userFilter, _ := s.settingService.GetLdapUserFilter()
		userAttr, _ := s.settingService.GetLdapUserAttr()

		cfg := ldaputil.Config{
			Host:       host,
			Port:       port,
			UseTLS:     useTLS,
			BindDN:     bindDN,
			Password:   ldapPass,
			BaseDN:     baseDN,
			UserFilter: userFilter,
			UserAttr:   userAttr,
		}
		ok, err := ldaputil.AuthenticateUser(cfg, username, password)
		if err != nil || !ok {
			return nil, errors.New("invalid credentials")
		}
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		logger.Warning("check two factor err:", err)
		return nil, err
	}

	if twoFactorEnable {
		twoFactorToken, err := s.settingService.GetTwoFactorToken()

		if err != nil {
			logger.Warning("check two factor token err:", err)
			return nil, err
		}

		if gotp.NewDefaultTOTP(twoFactorToken).Now() != twoFactorCode {
			return nil, errors.New("invalid 2fa code")
		}
	}

	return user, nil
}

func (s *UserService) BumpLoginEpoch() error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("1 = 1").
		Update("login_epoch", gorm.Expr("login_epoch + 1")).
		Error
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	db := database.GetDB()
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)

	if err != nil {
		return err
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		return err
	}

	if twoFactorEnable {
		s.settingService.SetTwoFactorEnable(false)
		s.settingService.SetTwoFactorToken("")
	}

	return db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"username":    username,
			"password":    hashedPassword,
			"login_epoch": gorm.Expr("login_epoch + 1"),
		}).
		Error
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	hashedPassword, er := crypto.HashPasswordAsBcrypt(password)

	if er != nil {
		return er
	}

	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPassword
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	user.LoginEpoch++
	return db.Save(user).Error
}
