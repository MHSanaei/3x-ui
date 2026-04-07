package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
)

func configsEqual(a, b *config.DatabaseConfig) bool {
	if a == nil || b == nil {
		return false
	}
	a = a.Clone().Normalize()
	b = b.Clone().Normalize()
	if a.Driver != b.Driver {
		return false
	}
	if a.Driver == config.DatabaseDriverSQLite {
		return a.SQLite.Path == b.SQLite.Path
	}
	return a.Postgres.Mode == b.Postgres.Mode &&
		a.Postgres.Host == b.Postgres.Host &&
		a.Postgres.Port == b.Postgres.Port &&
		a.Postgres.DBName == b.Postgres.DBName &&
		a.Postgres.User == b.Postgres.User &&
		a.Postgres.Password == b.Postgres.Password &&
		a.Postgres.SSLMode == b.Postgres.SSLMode &&
		a.Postgres.ManagedLocally == b.Postgres.ManagedLocally
}

func loadSnapshotFromConfig(cfg *config.DatabaseConfig) (*BackupSnapshot, error) {
	cfg = cfg.Clone().Normalize()
	if cfg.UsesSQLite() {
		if _, err := os.Stat(cfg.SQLite.Path); err != nil {
			if os.IsNotExist(err) {
				return newBackupSnapshot(cfg.Driver), nil
			}
			return nil, err
		}
	}

	conn, err := OpenDatabase(cfg)
	if err != nil {
		return nil, err
	}
	defer CloseConnection(conn)
	if err := MigrateModels(conn); err != nil {
		return nil, err
	}
	return ExportSnapshot(conn, cfg.Driver)
}

func saveSwitchBackup(snapshot *BackupSnapshot, prefix string) error {
	if snapshot == nil {
		return nil
	}
	data, err := EncodePortableBackup(snapshot)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%s.xui-backup", prefix, time.Now().UTC().Format("20060102-150405"))
	return SavePortableBackup(filepath.Join(config.GetBackupFolderPath(), name), data)
}

// SwitchDatabase migrates panel data into a new backend and writes the new runtime configuration.
func SwitchDatabase(target *config.DatabaseConfig) error {
	if target == nil {
		return errors.New("target database configuration is nil")
	}
	target = target.Clone().Normalize()

	currentCfg, err := config.LoadDatabaseConfig()
	if err != nil {
		return err
	}
	if configsEqual(currentCfg, target) {
		return config.SaveDatabaseConfig(target)
	}

	sourceSnapshot, err := loadSnapshotFromConfig(currentCfg)
	if err != nil {
		return err
	}
	if err := saveSwitchBackup(sourceSnapshot, "switch"); err != nil {
		return err
	}

	if err := TestConnection(target); err != nil {
		return err
	}

	targetConn, err := OpenDatabase(target)
	if err != nil {
		return err
	}
	defer CloseConnection(targetConn)

	if err := MigrateModels(targetConn); err != nil {
		return err
	}

	if err := ApplySnapshot(targetConn, sourceSnapshot); err != nil {
		return err
	}

	return config.SaveDatabaseConfig(target)
}
