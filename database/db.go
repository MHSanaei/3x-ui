// Package database provides database initialization, migration, and management utilities
// for the 3x-ui panel using GORM with SQLite and optional MySQL split storage.
package database

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB
var inboundDB *gorm.DB

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func initSQLiteModels(includeInboundModels bool) error {
	models := []any{
		&model.User{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&model.HistoryOfSeeders{},
	}
	if includeInboundModels {
		models = append(models, &model.Inbound{}, &xray.ClientTraffic{})
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func initInboundModels() error {
	if inboundDB == nil {
		return errors.New("inbound database is nil")
	}
	models := []any{
		&model.Inbound{},
		&xray.ClientTraffic{},
	}
	for _, model := range models {
		if err := inboundDB.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating inbound model: %v", err)
			return err
		}
	}
	return nil
}

func migrateInboundDataIfNeeded() error {
	if inboundDB == nil || db == nil || inboundDB == db {
		return nil
	}

	var mysqlInboundCount int64
	if err := inboundDB.Model(&model.Inbound{}).Count(&mysqlInboundCount).Error; err != nil {
		return err
	}
	if mysqlInboundCount == 0 {
		var sqliteInbounds []model.Inbound
		if err := db.Model(&model.Inbound{}).Find(&sqliteInbounds).Error; err != nil {
			return err
		}
		if len(sqliteInbounds) > 0 {
			if err := inboundDB.CreateInBatches(&sqliteInbounds, 200).Error; err != nil {
				return err
			}
		}
	}

	var mysqlClientTrafficCount int64
	if err := inboundDB.Model(&xray.ClientTraffic{}).Count(&mysqlClientTrafficCount).Error; err != nil {
		return err
	}
	if mysqlClientTrafficCount == 0 {
		var sqliteClientTraffics []xray.ClientTraffic
		if err := db.Model(&xray.ClientTraffic{}).Find(&sqliteClientTraffics).Error; err != nil {
			return err
		}
		if len(sqliteClientTraffics) > 0 {
			if err := inboundDB.CreateInBatches(&sqliteClientTraffics, 500).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func getMySQLDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("XUI_MYSQL_DSN")); dsn != "" {
		return dsn
	}

	host := strings.TrimSpace(os.Getenv("XUI_MYSQL_HOST"))
	port := strings.TrimSpace(os.Getenv("XUI_MYSQL_PORT"))
	user := strings.TrimSpace(os.Getenv("XUI_MYSQL_USER"))
	pass := os.Getenv("XUI_MYSQL_PASSWORD")
	dbName := strings.TrimSpace(os.Getenv("XUI_MYSQL_DB"))
	params := strings.TrimSpace(os.Getenv("XUI_MYSQL_PARAMS"))

	if host == "" || user == "" || dbName == "" {
		return ""
	}
	if port == "" {
		port = "3306"
	}
	if params == "" {
		params = "charset=utf8mb4&parseTime=True&loc=Local"
	}
	return user + ":" + pass + "@tcp(" + host + ":" + port + ")/" + dbName + "?" + params
}

// initUser creates a default admin user if the users table is empty.
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

// runSeeders migrates user passwords to bcrypt and records seeder execution to prevent re-running.
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

// isTableEmpty returns true if the named table contains zero rows.
func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

// InitDB sets up the database connection, migrates models, and runs seeders.
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
	inboundDB = db

	mysqlDSN := getMySQLDSN()
	useDedicatedInboundDB := mysqlDSN != ""
	if useDedicatedInboundDB {
		mysqlInboundDB, mysqlErr := gorm.Open(mysql.Open(mysqlDSN), c)
		if mysqlErr != nil {
			return mysqlErr
		}
		inboundDB = mysqlInboundDB
	}

	if err := initSQLiteModels(!useDedicatedInboundDB); err != nil {
		return err
	}
	if useDedicatedInboundDB {
		if err := initInboundModels(); err != nil {
			return err
		}
		if err := migrateInboundDataIfNeeded(); err != nil {
			return err
		}
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}

	if err := initUser(); err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

// CloseDB closes the database connection if it exists.
func CloseDB() error {
	var closeErr error
	if inboundDB != nil && db != nil && inboundDB != db {
		sqlInboundDB, err := inboundDB.DB()
		if err != nil {
			closeErr = err
		} else {
			closeErr = sqlInboundDB.Close()
		}
	}
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			if closeErr != nil {
				return closeErr
			}
			return err
		}
		if err = sqlDB.Close(); err != nil {
			if closeErr != nil {
				return closeErr
			}
			return err
		}
	}
	return closeErr
}

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB {
	return db
}

// GetInboundDB returns the DB used for inbounds and client traffics.
func GetInboundDB() *gorm.DB {
	if inboundDB != nil {
		return inboundDB
	}
	return db
}

// IsNotFound checks if the given error is a GORM record not found error.
func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

// IsSQLiteDB checks if the given file is a valid SQLite database by reading its signature.
func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

// Checkpoint performs a WAL checkpoint on the SQLite database to ensure data consistency.
func Checkpoint() error {
	// Update WAL
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
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
