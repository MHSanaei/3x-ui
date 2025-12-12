package database

import (
	"errors"
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// InitDB открывает sqlite и выполняет миграции / начальное заполнение.
func InitDB(dbPath string) error {
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}
	db = database

	// миграции
	if err := AutoMigrate(); err != nil {
		return err
	}

	// seed admin (один раз создаём дефолтного админа при отсутствии)
	if err := SeedAdmin(); err != nil {
		return err
	}

	return nil
}

// GetDB возвращает активное соединение GORM.
func GetDB() *gorm.DB {
	return db
}

// CloseDB закрывает соединение с БД.
func CloseDB() error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// IsNotFound — хелпер для проверки "запись не найдена".
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// Checkpoint — безопасный чекпоинт WAL для sqlite.
// Для других СУБД — no-op.
func Checkpoint() error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if db.Dialector.Name() != "sqlite" {
		return nil
	}
	// TRUNCATE обычно полезнее, чтобы подрезать WAL-файл.
	return db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
}

// AutoMigrate применяет миграции схемы.
func AutoMigrate() error {
	return db.AutoMigrate(
		&model.User{},
		&model.Setting{}, // таблица настроек
	)
}

// SeedAdmin создаёт дефолтного админа, если его нет.
func SeedAdmin() error {
	var count int64
	if err := db.Model(&model.User{}).
		Where("username = ?", "admin@local.test").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("Admin12345!"), 12)
	admin := model.User{
		Username:     "admin@local.test",
		PasswordHash: string(hash),
		Role:         "admin",
	}
	return db.Create(&admin).Error
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return errors.New("sqlite integrity check failed: " + res)
	}
	return nil
}
