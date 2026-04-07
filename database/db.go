// Package database provides database initialization, migration, and management utilities
// for the 3x-ui panel using GORM with SQLite or PostgreSQL.
package database

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	db       *gorm.DB
	dbConfig *config.DatabaseConfig
)

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func gormConfig() *gorm.Config {
	var loggerImpl gormlogger.Interface
	if config.IsDebug() {
		loggerImpl = gormlogger.Default
	} else {
		loggerImpl = gormlogger.Discard
	}
	return &gorm.Config{Logger: loggerImpl}
}

func openSQLiteDatabase(path string) (*gorm.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, fs.ModePerm); err != nil {
		return nil, err
	}
	return gorm.Open(sqlite.Open(path), gormConfig())
}

func buildPostgresDSN(cfg *config.DatabaseConfig) string {
	user := url.User(cfg.Postgres.User)
	if cfg.Postgres.Password != "" {
		user = url.UserPassword(cfg.Postgres.User, cfg.Postgres.Password)
	}
	authority := net.JoinHostPort(cfg.Postgres.Host, strconv.Itoa(cfg.Postgres.Port))
	u := &url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   authority,
		Path:   cfg.Postgres.DBName,
	}
	query := u.Query()
	query.Set("sslmode", cfg.Postgres.SSLMode)
	u.RawQuery = query.Encode()
	return u.String()
}

func openPostgresDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := gormConfig()
	gormCfg.PrepareStmt = true
	conn, err := gorm.Open(postgres.Open(buildPostgresDSN(cfg)), gormCfg)
	if err != nil {
		return nil, err
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	return conn, nil
}

// OpenDatabase opens a database connection from the provided runtime configuration.
func OpenDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	if cfg == nil {
		cfg = config.DefaultDatabaseConfig()
	}
	cfg = cfg.Clone().Normalize()

	switch cfg.Driver {
	case config.DatabaseDriverSQLite:
		return openSQLiteDatabase(cfg.SQLite.Path)
	case config.DatabaseDriverPostgres:
		return openPostgresDatabase(cfg)
	default:
		return nil, errors.New("unsupported database driver: " + cfg.Driver)
	}
}

// CloseConnection closes a standalone gorm connection.
func CloseConnection(conn *gorm.DB) error {
	if conn == nil {
		return nil
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func initModels(conn *gorm.DB) error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
	}
	for _, item := range models {
		if err := conn.AutoMigrate(item); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func isTableEmpty(conn *gorm.DB, tableName string) (bool, error) {
	if !conn.Migrator().HasTable(tableName) {
		return true, nil
	}
	var count int64
	err := conn.Table(tableName).Count(&count).Error
	return count == 0, err
}

func initUser(conn *gorm.DB) error {
	empty, err := isTableEmpty(conn, "users")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}
	if !empty {
		return nil
	}

	hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)
	if err != nil {
		log.Printf("Error hashing default password: %v", err)
		return err
	}

	user := &model.User{
		Username: defaultUsername,
		Password: hashedPassword,
	}
	return conn.Create(user).Error
}

func runSeeders(conn *gorm.DB, isUsersEmpty bool) error {
	empty, err := isTableEmpty(conn, "history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return conn.Create(hashSeeder).Error
	}

	var seedersHistory []string
	if err := conn.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory).Error; err != nil {
		return err
	}

	if slices.Contains(seedersHistory, "UserPasswordHash") || isUsersEmpty {
		return nil
	}

	var users []model.User
	if err := conn.Find(&users).Error; err != nil {
		return err
	}

	for _, user := range users {
		hashedPassword, hashErr := crypto.HashPasswordAsBcrypt(user.Password)
		if hashErr != nil {
			log.Printf("Error hashing password for user '%s': %v", user.Username, hashErr)
			return hashErr
		}
		if err := conn.Model(&user).Update("password", hashedPassword).Error; err != nil {
			return err
		}
	}

	hashSeeder := &model.HistoryOfSeeders{
		SeederName: "UserPasswordHash",
	}
	return conn.Create(hashSeeder).Error
}

// MigrateModels migrates the database schema for all panel models.
func MigrateModels(conn *gorm.DB) error {
	return initModels(conn)
}

// PrepareDatabase migrates the schema and optionally seeds the database.
func PrepareDatabase(conn *gorm.DB, seed bool) error {
	if err := initModels(conn); err != nil {
		return err
	}
	if !seed {
		return nil
	}

	isUsersEmpty, err := isTableEmpty(conn, "users")
	if err != nil {
		return err
	}

	if err := initUser(conn); err != nil {
		return err
	}
	return runSeeders(conn, isUsersEmpty)
}

// TestConnection verifies that the provided database configuration is reachable.
func TestConnection(cfg *config.DatabaseConfig) error {
	conn, err := OpenDatabase(cfg)
	if err != nil {
		return err
	}
	defer CloseConnection(conn)

	sqlDB, err := conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// InitDB sets up the database connection, migrates models, and runs seeders.
func InitDB() error {
	cfg, err := config.LoadDatabaseConfig()
	if err != nil {
		return err
	}
	return InitDBWithConfig(cfg)
}

// InitDBWithConfig sets up the database using an explicit runtime config.
func InitDBWithConfig(cfg *config.DatabaseConfig) error {
	conn, err := OpenDatabase(cfg)
	if err != nil {
		return err
	}
	if err := PrepareDatabase(conn, true); err != nil {
		_ = CloseConnection(conn)
		return err
	}

	if err := CloseDB(); err != nil {
		_ = CloseConnection(conn)
		return err
	}

	db = conn
	dbConfig = cfg.Clone().Normalize()
	return nil
}

// CloseDB closes the global database connection if it exists.
func CloseDB() error {
	if db == nil {
		dbConfig = nil
		return nil
	}
	err := CloseConnection(db)
	db = nil
	dbConfig = nil
	return err
}

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB {
	return db
}

// GetDBConfig returns a copy of the active database runtime configuration.
func GetDBConfig() *config.DatabaseConfig {
	if dbConfig == nil {
		return nil
	}
	return dbConfig.Clone()
}

// GetDriver returns the active GORM dialector name.
func GetDriver() string {
	if db != nil && db.Dialector != nil {
		return db.Dialector.Name()
	}
	if dbConfig != nil {
		switch dbConfig.Driver {
		case config.DatabaseDriverPostgres:
			return "postgres"
		case config.DatabaseDriverSQLite:
			return "sqlite"
		}
	}
	return ""
}

// IsSQLite reports whether the active database uses SQLite.
func IsSQLite() bool {
	return GetDriver() == "sqlite"
}

// IsPostgres reports whether the active database uses PostgreSQL.
func IsPostgres() bool {
	return GetDriver() == "postgres"
}

// IsDatabaseEmpty reports whether the provided database contains any application rows.
func IsDatabaseEmpty(conn *gorm.DB) (bool, error) {
	tables := []string{
		"users",
		"inbounds",
		"outbound_traffics",
		"settings",
		"inbound_client_ips",
		"client_traffics",
		"history_of_seeders",
	}
	for _, table := range tables {
		empty, err := isTableEmpty(conn, table)
		if err != nil {
			return false, err
		}
		if !empty {
			return false, nil
		}
	}
	return true, nil
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
	if !IsSQLite() || db == nil {
		return nil
	}
	return db.Exec("PRAGMA wal_checkpoint;").Error
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: gormlogger.Discard})
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
