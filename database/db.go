package database

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"x-ui/config"
	"x-ui/database/model"
	"x-ui/util/crypto"
	"x-ui/xray"

	"gorm.io/driver/postgres"
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

// loadEnvFile loads environment variables from a file
func loadEnvFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil // File doesn't exist, not an error
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// getDatabaseConfig retrieves database configuration from settings
func getDatabaseConfig() (*config.DatabaseConfig, error) {
	// Load environment variables from file if it exists
	if err := loadEnvFile("/etc/x-ui/db.env"); err != nil {
		log.Printf("Warning: Could not load database environment file: %v", err)
	}

	// Try to get configuration from settings
	// This is a simplified version - in real implementation you'd get this from SettingService
	dbConfig := config.GetDefaultDatabaseConfig()

	// Load configuration from environment variables
	if dbType := os.Getenv("DB_TYPE"); dbType != "" {
		dbConfig.Type = config.DatabaseType(dbType)
	}

	if dbConfig.Type == config.DatabaseTypePostgreSQL {
		if host := os.Getenv("DB_HOST"); host != "" {
			dbConfig.Postgres.Host = host
		}
		if port := os.Getenv("DB_PORT"); port != "" {
			if p, err := strconv.Atoi(port); err == nil {
				dbConfig.Postgres.Port = p
			}
		}
		if database := os.Getenv("DB_NAME"); database != "" {
			dbConfig.Postgres.Database = database
		}
		if username := os.Getenv("DB_USER"); username != "" {
			dbConfig.Postgres.Username = username
		}
		if password := os.Getenv("DB_PASSWORD"); password != "" {
			dbConfig.Postgres.Password = password
		}
		if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
			dbConfig.Postgres.SSLMode = sslMode
		}
		if timeZone := os.Getenv("DB_TIMEZONE"); timeZone != "" {
			dbConfig.Postgres.TimeZone = timeZone
		}
	}

	return dbConfig, nil
}

func InitDB(dbPath string) error {
	// Try to get configuration from environment file first
	dbConfig, err := getDatabaseConfig()
	if err != nil {
		return err
	}

	// If still using SQLite and dbPath is provided, use it
	if dbConfig.Type == config.DatabaseTypeSQLite && dbPath != "" {
		dbConfig.SQLite.Path = dbPath
	}

	return InitDBWithConfig(dbConfig)
}

// InitDBWithConfig initializes database with provided configuration
func InitDBWithConfig(dbConfig *config.DatabaseConfig) error {
	// Validate configuration
	if err := dbConfig.ValidateConfig(); err != nil {
		return err
	}

	// Ensure directory exists for SQLite
	if err := dbConfig.EnsureDirectoryExists(); err != nil {
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

	// Open database connection based on type
	var err error
	switch dbConfig.Type {
	case config.DatabaseTypeSQLite:
		db, err = gorm.Open(sqlite.Open(dbConfig.GetDSN()), c)
	case config.DatabaseTypePostgreSQL:
		db, err = gorm.Open(postgres.Open(dbConfig.GetDSN()), c)
	default:
		return fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}

	if err != nil {
		return err
	}

	if err := initModels(); err != nil {
		return err
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

// TestDatabaseConnection tests database connection with provided configuration
func TestDatabaseConnection(dbConfig *config.DatabaseConfig) error {
	// Validate configuration
	if err := dbConfig.ValidateConfig(); err != nil {
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

	// Test database connection based on type
	var testDB *gorm.DB
	var err error
	switch dbConfig.Type {
	case config.DatabaseTypeSQLite:
		testDB, err = gorm.Open(sqlite.Open(dbConfig.GetDSN()), c)
	case config.DatabaseTypePostgreSQL:
		testDB, err = gorm.Open(postgres.Open(dbConfig.GetDSN()), c)
	default:
		return fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}

	if err != nil {
		return err
	}

	// Test the connection
	sqlDB, err := testDB.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	return sqlDB.Ping()
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
