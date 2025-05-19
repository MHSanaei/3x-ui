package config

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug  LogLevel = "debug"
	Info   LogLevel = "info"
	Notice LogLevel = "notice"
	Warn   LogLevel = "warn"
	Error  LogLevel = "error"
)

func GetVersion() string {
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("XUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return os.Getenv("XUI_DEBUG") == "true"
}

func GetBinFolderPath() string {
	binFolderPath := os.Getenv("XUI_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	return binFolderPath
}

func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("XUI_DB_FOLDER")
	if dbFolderPath == "" {
		dbFolderPath = "/etc/x-ui"
	}
	return dbFolderPath
}

// DatabaseConfig holds the database configuration
type DatabaseConfig struct {
	Connection string
	Host       string
	Port       string
	Database   string
	Username   string
	Password   string
}

// GetDatabaseConfig returns the database configuration from environment variables
func GetDatabaseConfig() (*DatabaseConfig, error) {
	config := &DatabaseConfig{
		Connection: strings.ToLower(os.Getenv("XUI_DB_CONNECTION")),
		Host:       os.Getenv("XUI_DB_HOST"),
		Port:       os.Getenv("XUI_DB_PORT"),
		Database:   os.Getenv("XUI_DB_DATABASE"),
		Username:   os.Getenv("XUI_DB_USERNAME"),
		Password:   os.Getenv("XUI_DB_PASSWORD"),
	}

	if config.Connection == "mysql" {
		if config.Host == "" || config.Database == "" || config.Username == "" {
			return nil, fmt.Errorf("missing required MySQL configuration: host, database, and username are required")
		}
		if config.Port == "" {
			config.Port = "3306"
		}
	}

	return config, nil
}

func GetDBPath() string {
	config, err := GetDatabaseConfig()
	if err != nil {
		log.Fatalf("Error getting database config: %v", err)
	}

	if config.Connection == "mysql" {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local",
			config.Username,
			config.Password,
			config.Host,
			config.Port,
			config.Database)
	}

	// Connection is sqlite
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

func GetLogFolder() string {
	logFolderPath := os.Getenv("XUI_LOG_FOLDER")
	if logFolderPath == "" {
		logFolderPath = "/var/log"
	}
	return logFolderPath
}
