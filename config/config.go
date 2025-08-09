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
	if dbFolderPath != "" {
		return dbFolderPath
	}

	defaultFolder := "/etc/x-ui"

	if runtime.GOOS == "windows" {
		homeDir := os.Getenv("LOCALAPPDATA")
		if homeDir == "" {
			logger.Error("Error while getting local app data folder")
			return defaultFolder
		}

		userFolder := filepath.Join(homeDir, "x-ui")
		err := moveExistingDb(defaultFolder, userFolder)
		if err != nil {
			logger.Error("Error while moving existing DB: %w", err)
			return defaultFolder
		}

		return userFolder
	} else {
		return defaultFolder
	}
}

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

func GetLogFolder() string {
	logFolderPath := os.Getenv("XUI_LOG_FOLDER")
	if logFolderPath == "" {
		logFolderPath = "/var/log"
	}
	return logFolderPath
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
