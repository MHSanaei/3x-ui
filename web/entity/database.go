package entity

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v2/config"
)

type DatabaseSetting struct {
	Driver                      string `json:"driver" form:"driver"`
	ConfigSource                string `json:"configSource" form:"configSource"`
	ReadOnly                    bool   `json:"readOnly" form:"readOnly"`
	SQLitePath                  string `json:"sqlitePath" form:"sqlitePath"`
	PostgresMode                string `json:"postgresMode" form:"postgresMode"`
	PostgresHost                string `json:"postgresHost" form:"postgresHost"`
	PostgresPort                int    `json:"postgresPort" form:"postgresPort"`
	PostgresDBName              string `json:"postgresDBName" form:"postgresDBName"`
	PostgresUser                string `json:"postgresUser" form:"postgresUser"`
	PostgresPassword            string `json:"postgresPassword" form:"postgresPassword"`
	PostgresPasswordSet         bool   `json:"postgresPasswordSet" form:"postgresPasswordSet"`
	PostgresSSLMode             string `json:"postgresSSLMode" form:"postgresSSLMode"`
	ManagedLocally              bool   `json:"managedLocally" form:"managedLocally"`
	LocalInstalled              bool   `json:"localInstalled" form:"localInstalled"`
	CanInstallLocally           bool   `json:"canInstallLocally" form:"canInstallLocally"`
	NativeSQLiteExportAvailable bool   `json:"nativeSQLiteExportAvailable" form:"nativeSQLiteExportAvailable"`
}

func DatabaseSettingFromConfig(cfg *config.DatabaseConfig) *DatabaseSetting {
	if cfg == nil {
		cfg = config.DefaultDatabaseConfig()
	}
	cfg = cfg.Clone().Normalize()
	return &DatabaseSetting{
		Driver:                      cfg.Driver,
		ConfigSource:                cfg.ConfigSource,
		SQLitePath:                  cfg.SQLite.Path,
		PostgresMode:                cfg.Postgres.Mode,
		PostgresHost:                cfg.Postgres.Host,
		PostgresPort:                cfg.Postgres.Port,
		PostgresDBName:              cfg.Postgres.DBName,
		PostgresUser:                cfg.Postgres.User,
		PostgresPasswordSet:         cfg.Postgres.Password != "",
		PostgresSSLMode:             cfg.Postgres.SSLMode,
		ManagedLocally:              cfg.Postgres.ManagedLocally,
		NativeSQLiteExportAvailable: cfg.Driver == config.DatabaseDriverSQLite,
	}
}

func (s *DatabaseSetting) Normalize() *DatabaseSetting {
	if s == nil {
		s = &DatabaseSetting{}
	}
	s.Driver = strings.ToLower(strings.TrimSpace(s.Driver))
	if s.Driver == "" {
		s.Driver = config.DatabaseDriverSQLite
	}
	s.PostgresMode = strings.ToLower(strings.TrimSpace(s.PostgresMode))
	if s.PostgresMode == "" {
		s.PostgresMode = config.DatabaseModeExternal
	}
	if s.PostgresHost == "" {
		s.PostgresHost = "127.0.0.1"
	}
	if s.PostgresPort <= 0 {
		s.PostgresPort = 5432
	}
	if s.PostgresSSLMode == "" {
		s.PostgresSSLMode = "disable"
	}
	if s.PostgresMode == config.DatabaseModeLocal {
		s.ManagedLocally = true
	}
	if s.SQLitePath == "" {
		s.SQLitePath = config.GetDBPath()
	}
	return s
}

func (s *DatabaseSetting) ToConfig(existing *config.DatabaseConfig) *config.DatabaseConfig {
	current := config.DefaultDatabaseConfig()
	if existing != nil {
		current = existing.Clone().Normalize()
	}
	s = s.Normalize()

	current.Driver = s.Driver
	current.SQLite.Path = s.SQLitePath
	current.Postgres.Mode = s.PostgresMode
	current.Postgres.Host = s.PostgresHost
	current.Postgres.Port = s.PostgresPort
	current.Postgres.DBName = s.PostgresDBName
	current.Postgres.User = s.PostgresUser
	if s.PostgresPassword != "" {
		current.Postgres.Password = s.PostgresPassword
	}
	current.Postgres.SSLMode = s.PostgresSSLMode
	current.Postgres.ManagedLocally = s.ManagedLocally
	return current.Normalize()
}
