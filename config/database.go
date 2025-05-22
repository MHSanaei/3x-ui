package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type     DatabaseType   `json:"type"`
	SQLite   SQLiteConfig   `json:"sqlite"`
	Postgres PostgresConfig `json:"postgres"`
}

// SQLiteConfig holds SQLite specific configuration
type SQLiteConfig struct {
	Path string `json:"path"`
}

// PostgresConfig holds PostgreSQL specific configuration
type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"sslMode"`
	TimeZone string `json:"timeZone"`
}

// GetDSN returns the data source name for the database
func (c *DatabaseConfig) GetDSN() string {
	switch c.Type {
	case DatabaseTypeSQLite:
		return c.SQLite.Path
	case DatabaseTypePostgreSQL:
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			c.Postgres.Host,
			c.Postgres.Username,
			c.Postgres.Password,
			c.Postgres.Database,
			c.Postgres.Port,
			c.Postgres.SSLMode,
			c.Postgres.TimeZone,
		)
	default:
		return c.SQLite.Path
	}
}

// GetDefaultDatabaseConfig returns default database configuration
func GetDefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Type: DatabaseTypeSQLite,
		SQLite: SQLiteConfig{
			Path: getDefaultSQLitePath(),
		},
		Postgres: PostgresConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "x_ui",
			Username: "x_ui",
			Password: "",
			SSLMode:  "disable",
			TimeZone: "UTC",
		},
	}
}

// getDefaultSQLitePath returns the default SQLite database path
func getDefaultSQLitePath() string {
	if IsDebug() {
		return "db/x-ui.db"
	}
	return "/etc/x-ui/x-ui.db"
}

// ValidateConfig validates the database configuration
func (c *DatabaseConfig) ValidateConfig() error {
	switch c.Type {
	case DatabaseTypeSQLite:
		if c.SQLite.Path == "" {
			return fmt.Errorf("SQLite path cannot be empty")
		}
	case DatabaseTypePostgreSQL:
		if c.Postgres.Host == "" {
			return fmt.Errorf("PostgreSQL host cannot be empty")
		}
		if c.Postgres.Database == "" {
			return fmt.Errorf("PostgreSQL database name cannot be empty")
		}
		if c.Postgres.Username == "" {
			return fmt.Errorf("PostgreSQL username cannot be empty")
		}
		if c.Postgres.Port <= 0 || c.Postgres.Port > 65535 {
			return fmt.Errorf("PostgreSQL port must be between 1 and 65535")
		}
	default:
		return fmt.Errorf("unsupported database type: %s", c.Type)
	}
	return nil
}

// IsPostgreSQL returns true if the database type is PostgreSQL
func (c *DatabaseConfig) IsPostgreSQL() bool {
	return c.Type == DatabaseTypePostgreSQL
}

// IsSQLite returns true if the database type is SQLite
func (c *DatabaseConfig) IsSQLite() bool {
	return c.Type == DatabaseTypeSQLite
}

// EnsureDirectoryExists ensures the directory for SQLite database exists
func (c *DatabaseConfig) EnsureDirectoryExists() error {
	if c.Type == DatabaseTypeSQLite {
		dir := filepath.Dir(c.SQLite.Path)
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
