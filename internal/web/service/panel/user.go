package panel

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	ldaputil "github.com/mhsanaei/3x-ui/v3/internal/util/ldap"
	"github.com/mhsanaei/3x-ui/v3/internal/util/totp"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const firstUserCacheTTL = 15 * time.Second

var firstUserCache struct {
	mu         sync.RWMutex
	db         *gorm.DB
	generation uint64
	user       *model.User
	expiresAt  time.Time
	group      singleflight.Group
	hookMu     sync.Mutex
}

func invalidateFirstUserCache() {
	firstUserCache.mu.Lock()
	firstUserCache.generation++
	firstUserCache.user = nil
	firstUserCache.expiresAt = time.Time{}
	firstUserCache.mu.Unlock()
}

func ensureFirstUserWriteHooks(db *gorm.DB) {
	if db == nil {
		return
	}
	const cbCreate = "panel:first_user_cache_create"
	firstUserCache.hookMu.Lock()
	defer firstUserCache.hookMu.Unlock()
	if db.Callback().Create().Get(cbCreate) != nil {
		return
	}
	cb := func(tx *gorm.DB) {
		if tx != nil && tx.Statement != nil && tx.Statement.Table == "users" {
			invalidateFirstUserCache()
		}
	}
	onRaw := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		sqlStr := strings.ToLower(tx.Statement.SQL.String())
		if strings.Contains(sqlStr, "users") &&
			(strings.Contains(sqlStr, "update ") || strings.Contains(sqlStr, "delete ") ||
				strings.Contains(sqlStr, "insert ") || strings.Contains(sqlStr, "replace ")) {
			invalidateFirstUserCache()
		}
	}
	_ = db.Callback().Create().After("gorm:create").Register(cbCreate, cb)
	_ = db.Callback().Update().After("gorm:update").Register("panel:first_user_cache_update", cb)
	_ = db.Callback().Delete().After("gorm:delete").Register("panel:first_user_cache_delete", cb)
	_ = db.Callback().Raw().After("gorm:raw").Register("panel:first_user_cache_raw", onRaw)
}

// UserService provides business logic for user management and authentication.
// It handles user creation, login, password management, and 2FA operations.
type UserService struct {
	settingService service.SettingService
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

// GetFirstUserCached returns a shallow copy of the cached first user for API
// auth middleware, invalidating automatically on writes or DB swap.
func (s *UserService) GetFirstUserCached() (*model.User, error) {
	db := database.GetDB()
	if db == nil {
		return nil, gorm.ErrInvalidDB
	}
	ensureFirstUserWriteHooks(db)
	now := time.Now()
	firstUserCache.mu.RLock()
	if firstUserCache.db == db && firstUserCache.user != nil && now.Before(firstUserCache.expiresAt) {
		u := *firstUserCache.user
		firstUserCache.mu.RUnlock()
		return &u, nil
	}
	firstUserCache.mu.RUnlock()

	v, err, _ := firstUserCache.group.Do(fmt.Sprintf("%p:first", db), func() (any, error) {
		firstUserCache.mu.RLock()
		if firstUserCache.db == db && firstUserCache.user != nil && time.Now().Before(firstUserCache.expiresAt) {
			u := *firstUserCache.user
			firstUserCache.mu.RUnlock()
			return &u, nil
		}
		gen := firstUserCache.generation
		prevDB := firstUserCache.db
		firstUserCache.mu.RUnlock()

		var user model.User
		if err := db.Model(model.User{}).First(&user).Error; err != nil {
			return nil, err
		}
		firstUserCache.mu.Lock()
		if firstUserCache.db != db {
			firstUserCache.db = db
			firstUserCache.generation++
			stored := user
			firstUserCache.user = &stored
			firstUserCache.expiresAt = time.Now().Add(firstUserCacheTTL)
		} else if gen == firstUserCache.generation && prevDB == db {
			stored := user
			firstUserCache.user = &stored
			firstUserCache.expiresAt = time.Now().Add(firstUserCacheTTL)
		}
		firstUserCache.mu.Unlock()
		ret := user
		return &ret, nil
	})
	if err != nil {
		return nil, err
	}
	u := *v.(*model.User)
	return &u, nil
}

// HasDefaultCredential reports that nobody has chosen the login yet: a database
// holds no user before the first start, and an older one may still hold admin/admin.
func (s *UserService) HasDefaultCredential() (bool, error) {
	user, err := s.GetFirstUser()
	if database.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return user.Username == "admin" && crypto.CheckPasswordHash(user.Password, "admin"), nil
}

func (s *UserService) CheckUser(username string, password string, twoFactorCode string) (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}

	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
		skipVerify, _ := s.settingService.GetLdapInsecureSkipVerify()
		bindDN, _ := s.settingService.GetLdapBindDN()
		ldapPass, _ := s.settingService.GetLdapPassword()
		baseDN, _ := s.settingService.GetLdapBaseDN()
		userFilter, _ := s.settingService.GetLdapUserFilter()
		userAttr, _ := s.settingService.GetLdapUserAttr()

		cfg := ldaputil.Config{
			Host:               host,
			Port:               port,
			UseTLS:             useTLS,
			InsecureSkipVerify: skipVerify,
			BindDN:             bindDN,
			Password:           ldapPass,
			BaseDN:             baseDN,
			UserFilter:         userFilter,
			UserAttr:           userAttr,
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

		if !totp.VerifyWithSkew(twoFactorToken, twoFactorCode, time.Now()) {
			return nil, errors.New("invalid 2fa code")
		}
	}

	return user, nil
}

func (s *UserService) BumpLoginEpoch() error {
	db := database.GetDB()
	if err := db.Model(model.User{}).
		Where("1 = 1").
		Update("login_epoch", gorm.Expr("login_epoch + 1")).
		Error; err != nil {
		return err
	}
	invalidateFirstUserCache()
	return nil
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	if len(username) > MaxUsernameLength {
		return fmt.Errorf("username can not be longer than %d bytes", MaxUsernameLength)
	}
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
		_ = s.settingService.SetTwoFactorEnable(false)
		_ = s.settingService.SetTwoFactorToken("")
	}

	if err := db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"username":    username,
			"password":    hashedPassword,
			"login_epoch": gorm.Expr("login_epoch + 1"),
		}).
		Error; err != nil {
		return err
	}
	invalidateFirstUserCache()
	return nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if len(username) > MaxUsernameLength {
		return fmt.Errorf("username can not be longer than %d bytes", MaxUsernameLength)
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
		if err := db.Model(model.User{}).Create(user).Error; err != nil {
			return err
		}
		invalidateFirstUserCache()
		return nil
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	user.LoginEpoch++
	if err := db.Save(user).Error; err != nil {
		return err
	}
	invalidateFirstUserCache()
	return nil
}

// MaxUsernameLength bounds a panel username: the login route refuses longer names before
// logging or recording them, so no stored name may exceed it.
const MaxUsernameLength = 256
