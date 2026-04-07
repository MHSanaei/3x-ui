package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DatabaseDriverSQLite   = "sqlite"
	DatabaseDriverPostgres = "postgres"

	DatabaseConfigSourceDefault = "default"
	DatabaseConfigSourceFile    = "file"
	DatabaseConfigSourceEnv     = "env"

	DatabaseModeLocal    = "local"
	DatabaseModeExternal = "external"
)

type SQLiteDatabaseConfig struct {
	Path string `json:"path"`
}

type PostgresDatabaseConfig struct {
	Mode           string `json:"mode"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	DBName         string `json:"dbName"`
	User           string `json:"user"`
	Password       string `json:"password,omitempty"`
	SSLMode        string `json:"sslMode"`
	ManagedLocally bool   `json:"managedLocally"`
}

type DatabaseConfig struct {
	Driver       string                 `json:"driver"`
	ConfigSource string                 `json:"configSource,omitempty"`
	SQLite       SQLiteDatabaseConfig   `json:"sqlite"`
	Postgres     PostgresDatabaseConfig `json:"postgres"`
}

func DefaultDatabaseConfig() *DatabaseConfig {
	name := GetName()
	if name == "" {
		name = "x-ui"
	}
	return (&DatabaseConfig{
		Driver: DatabaseDriverSQLite,
		SQLite: SQLiteDatabaseConfig{
			Path: GetDBPath(),
		},
		Postgres: PostgresDatabaseConfig{
			Mode:           DatabaseModeExternal,
			Host:           "127.0.0.1",
			Port:           5432,
			DBName:         name,
			User:           name,
			SSLMode:        "disable",
			ManagedLocally: false,
		},
	}).Normalize()
}

func (c *DatabaseConfig) Clone() *DatabaseConfig {
	if c == nil {
		return nil
	}
	cloned := *c
	return &cloned
}

func (c *DatabaseConfig) Normalize() *DatabaseConfig {
	if c == nil {
		return DefaultDatabaseConfig()
	}

	if c.Driver == "" {
		c.Driver = DatabaseDriverSQLite
	}
	c.Driver = strings.ToLower(strings.TrimSpace(c.Driver))
	if c.Driver != DatabaseDriverSQLite && c.Driver != DatabaseDriverPostgres {
		c.Driver = DatabaseDriverSQLite
	}

	if c.SQLite.Path == "" {
		c.SQLite.Path = GetDBPath()
	}

	c.Postgres.Mode = strings.ToLower(strings.TrimSpace(c.Postgres.Mode))
	if c.Postgres.Mode != DatabaseModeLocal && c.Postgres.Mode != DatabaseModeExternal {
		if c.Postgres.ManagedLocally {
			c.Postgres.Mode = DatabaseModeLocal
		} else {
			c.Postgres.Mode = DatabaseModeExternal
		}
	}
	if c.Postgres.Host == "" {
		c.Postgres.Host = "127.0.0.1"
	}
	if c.Postgres.Port <= 0 {
		c.Postgres.Port = 5432
	}
	if c.Postgres.DBName == "" {
		c.Postgres.DBName = GetName()
	}
	if c.Postgres.User == "" {
		c.Postgres.User = GetName()
	}
	if c.Postgres.SSLMode == "" {
		c.Postgres.SSLMode = "disable"
	}
	if c.Postgres.Mode == DatabaseModeLocal {
		c.Postgres.ManagedLocally = true
	}

	return c
}

func (c *DatabaseConfig) UsesSQLite() bool {
	return c != nil && c.Normalize().Driver == DatabaseDriverSQLite
}

func (c *DatabaseConfig) UsesPostgres() bool {
	return c != nil && c.Normalize().Driver == DatabaseDriverPostgres
}

func HasDatabaseEnvOverride() bool {
	driver := strings.TrimSpace(os.Getenv("XUI_DB_DRIVER"))
	if driver != "" {
		return true
	}
	return false
}

func loadDatabaseConfigFromEnv() (*DatabaseConfig, error) {
	driver := strings.ToLower(strings.TrimSpace(os.Getenv("XUI_DB_DRIVER")))
	if driver == "" {
		return nil, errors.New("database env override is not configured")
	}

	cfg := DefaultDatabaseConfig()
	cfg.ConfigSource = DatabaseConfigSourceEnv
	cfg.Driver = driver

	if path := strings.TrimSpace(os.Getenv("XUI_DB_PATH")); path != "" {
		cfg.SQLite.Path = path
	}

	if mode := strings.TrimSpace(os.Getenv("XUI_DB_MODE")); mode != "" {
		cfg.Postgres.Mode = mode
	}
	if host := strings.TrimSpace(os.Getenv("XUI_DB_HOST")); host != "" {
		cfg.Postgres.Host = host
	}
	if portStr := strings.TrimSpace(os.Getenv("XUI_DB_PORT")); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, err
		}
		cfg.Postgres.Port = port
	}
	if dbName := strings.TrimSpace(os.Getenv("XUI_DB_NAME")); dbName != "" {
		cfg.Postgres.DBName = dbName
	}
	if user := strings.TrimSpace(os.Getenv("XUI_DB_USER")); user != "" {
		cfg.Postgres.User = user
	}
	if password := os.Getenv("XUI_DB_PASSWORD"); password != "" {
		cfg.Postgres.Password = password
	}
	if sslMode := strings.TrimSpace(os.Getenv("XUI_DB_SSLMODE")); sslMode != "" {
		cfg.Postgres.SSLMode = sslMode
	}
	if managedLocally := strings.TrimSpace(os.Getenv("XUI_DB_MANAGED_LOCALLY")); managedLocally != "" {
		value, err := strconv.ParseBool(managedLocally)
		if err != nil {
			return nil, err
		}
		cfg.Postgres.ManagedLocally = value
	}

	return cfg.Normalize(), nil
}

func GetDBConfigPath() string {
	return filepath.Join(GetDBFolderPath(), "database.json")
}

func GetBackupFolderPath() string {
	return filepath.Join(GetDBFolderPath(), "backups")
}

func GetPostgresManagerPath() string {
	return filepath.Join(getBaseDir(), "postgres-manager.sh")
}

func LoadDatabaseConfig() (*DatabaseConfig, error) {
	if HasDatabaseEnvOverride() {
		return loadDatabaseConfigFromEnv()
	}

	configPath := GetDBConfigPath()
	contents, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultDatabaseConfig()
			cfg.ConfigSource = DatabaseConfigSourceDefault
			return cfg, nil
		}
		return nil, err
	}

	cfg := DefaultDatabaseConfig()
	if err := json.Unmarshal(contents, cfg); err != nil {
		return nil, err
	}
	cfg.ConfigSource = DatabaseConfigSourceFile
	return cfg.Normalize(), nil
}

func SaveDatabaseConfig(cfg *DatabaseConfig) error {
	if HasDatabaseEnvOverride() {
		return errors.New("database configuration is managed by environment variables")
	}
	if cfg == nil {
		return errors.New("database configuration is nil")
	}

	normalized := cfg.Clone().Normalize()
	normalized.ConfigSource = DatabaseConfigSourceFile

	configPath := GetDBConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}

	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, configPath)
}
