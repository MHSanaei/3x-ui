package config

import (
	_ "embed"
	"fmt"
	"strings"
	"x-ui/logger"

	"github.com/spf13/viper"
)

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
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
	logLevel := viper.GetString("XUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return viper.GetBool("XUI_DEBUG")
}

func GetBinFolderPath() string {
	binFolderPath := viper.GetString("XUI_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	return binFolderPath
}

func GetDBFolderPath() string {
	dbFolderPath := viper.GetString("XUI_DB_FOLDER")
	if dbFolderPath == "" {
		dbFolderPath = "/etc/x-ui"
	}
	return dbFolderPath
}

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

func init() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/x-ui")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		logger.Debug("Error reading config file: %s\n", err)
	}
}
