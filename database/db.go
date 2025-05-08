package database

import (
	"bytes"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"

	"x-ui/config"
	"x-ui/database/model"
	"x-ui/util/crypto"
	"x-ui/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func initUser() error {
	empty, err := isTableEmpty("users")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}
	if empty {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)

		if err != nil {
			log.Printf("Error hashing default password: %v", err)
			return err
		}

		user := &model.User{
			Username: defaultUsername,
			Password: hashedPassword,
		}
		return db.Create(user).Error
	}
	return nil
}

func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	} else {
		var seedersHistory []string
		db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory)

		if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
			var users []model.User
			db.Find(&users)

			for _, user := range users {
				hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
				if err != nil {
					log.Printf("Error hashing password for user '%s': %v", user.Username, err)
					return err
				}
				db.Model(&user).Update("password", hashedPassword)
			}

			hashSeeder := &model.HistoryOfSeeders{
				SeederName: "UserPasswordHash",
			}
			return db.Create(hashSeeder).Error
		}
	}

	return nil
}

func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

func InitDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, fs.ModePerm)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}

	if err := initModels(); err != nil {
		return err
	}

	isUsersEmpty, err := isTableEmpty("users")

	if err := initUser(); err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func Checkpoint() error {
	// Update WAL
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}
