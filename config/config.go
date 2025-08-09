package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"x-ui/logger"

	"github.com/otiai10/copy"
)

//go:embed version
var version string

//go:embed name
var name string

// default folder for database
var defaultDbFolder = "/etc/x-ui"

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

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", getDBFolderPath(), GetName())
}

func GetLogFolder() string {
	logFolderPath := os.Getenv("XUI_LOG_FOLDER")
	if logFolderPath == "" {
		logFolderPath = "/var/log"
	}
	return logFolderPath
}

func getDBFolderPath() string {
	dbFolderPath := os.Getenv("XUI_DB_FOLDER")
	if dbFolderPath != "" {
		return dbFolderPath
	}

	if runtime.GOOS == "windows" {
		return getWindowsDbPath()
	} else {
		return defaultDbFolder
	}
}

func getWindowsDbPath() string {
	homeDir := os.Getenv("LOCALAPPDATA")
	if homeDir == "" {
		logger.Errorf("Error while getting local app data folder, falling back to %s", defaultDbFolder)
		return defaultDbFolder
	}

	userFolder := filepath.Join(homeDir, "x-ui")
	err := moveExistingDb(defaultDbFolder, userFolder)
	if err != nil {
		logger.Error("Error while moving existing DB: %w, falling back to %s", err, defaultDbFolder)
		return defaultDbFolder
	}

	return userFolder
}

func moveExistingDb(from string, to string) error {
	if _, err := os.Stat(to); os.IsNotExist(err) {
		if _, err := os.Stat(from); !os.IsNotExist(err) {
			if err := copy.Copy(from, to); err != nil {
				return fmt.Errorf("failed to copy %s to %s: %w", from, to, err)
			}
		}
	}
	return nil
}
