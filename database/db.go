package database

import (
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB { return db }

// InitDB sets up the database connection, migrates models, and runs seeders.
func InitDB(dbPath string) error {
	// ensure dir exists
	dir := path.Dir(dbPath)
	if err := os.MkdirAll(dir, fs.ModePerm); err != nil {
		return err
	}

	// open SQLite (dev)
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}
	db = database

	// migrations
	if err := AutoMigrate(); err != nil {
		return err
	}

	// seed admin
	if err := SeedAdmin(); err != nil {
		return err
	}

	return nil
}

// AutoMigrate applies schema migrations.
func AutoMigrate() error {
	return db.AutoMigrate(
		&model.User{}, // User{ Id, Username, PasswordHash, Role }
	)
}

// SeedAdmin creates a default admin if it doesn't exist.
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

// IsNotFound reports whether err is gorm's record-not-found.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// IsSQLiteDB reports whether current DB dialector is sqlite.
func IsSQLiteDB() bool {
	if db == nil {
		return false
	}
	return db.Dialector.Name() == "sqlite"
}

// Checkpoint runs WAL checkpoint for SQLite to compact the WAL file.
// No-op for non-SQLite databases.
func Checkpoint() error {
	if !IsSQLiteDB() {
		return nil
	}
	// FULL/TRUNCATE — в зависимости от нужной семантики.
	// TRUNCATE чаще используется, чтобы обрезать WAL-файл.
	return db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
}
