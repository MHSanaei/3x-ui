package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
)

type DatabaseService struct{}

func (s *DatabaseService) currentConfig() (*config.DatabaseConfig, error) {
	current, err := config.LoadDatabaseConfig()
	if err != nil {
		return nil, err
	}
	return current.Normalize(), nil
}

func (s *DatabaseService) mergeSettingWithCurrent(setting *entity.DatabaseSetting) (*config.DatabaseConfig, error) {
	current, err := s.currentConfig()
	if err != nil {
		return nil, err
	}
	target := setting.ToConfig(current)
	if target.UsesPostgres() && target.Postgres.Password == "" && current.UsesPostgres() {
		sameEndpoint := target.Postgres.Host == current.Postgres.Host &&
			target.Postgres.Port == current.Postgres.Port &&
			target.Postgres.DBName == current.Postgres.DBName &&
			target.Postgres.User == current.Postgres.User
		if sameEndpoint {
			target.Postgres.Password = current.Postgres.Password
		}
	}
	return target.Normalize(), nil
}

func (s *DatabaseService) canInstallLocally() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return false
	}
	if strings.TrimSpace(os.Getenv("container")) != "" {
		return false
	}
	output, err := exec.Command("id", "-u").Output()
	return err == nil && strings.TrimSpace(string(output)) == "0"
}

func (s *DatabaseService) postgresManagerExists() bool {
	path := config.GetPostgresManagerPath()
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *DatabaseService) runPostgresManager(args ...string) (string, error) {
	if !s.postgresManagerExists() {
		return "", errors.New("postgres-manager.sh not found")
	}
	cmd := exec.Command(config.GetPostgresManagerPath(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func (s *DatabaseService) postgresStatus() (bool, bool) {
	if s.postgresManagerExists() {
		output, err := s.runPostgresManager("status")
		if err == nil {
			installed := strings.Contains(output, "installed=true")
			running := strings.Contains(output, "running=true")
			return installed, running
		}
	}

	_, err := exec.LookPath("psql")
	return err == nil, false
}

func (s *DatabaseService) GetSetting() (*entity.DatabaseSetting, error) {
	current, err := s.currentConfig()
	if err != nil {
		return nil, err
	}

	setting := entity.DatabaseSettingFromConfig(current)
	setting.ReadOnly = current.ConfigSource == config.DatabaseConfigSourceEnv
	setting.CanInstallLocally = s.canInstallLocally()
	setting.LocalInstalled, _ = s.postgresStatus()
	return setting, nil
}

func (s *DatabaseService) TestSetting(setting *entity.DatabaseSetting) error {
	target, err := s.mergeSettingWithCurrent(setting)
	if err != nil {
		return err
	}
	return database.TestConnection(target)
}

func (s *DatabaseService) InstallLocalPostgres() (string, error) {
	if !s.canInstallLocally() {
		return "", errors.New("local PostgreSQL installation requires root privileges")
	}
	return s.runPostgresManager("init-local")
}

func (s *DatabaseService) prepareLocalPostgres(target *config.DatabaseConfig) error {
	if !target.UsesPostgres() || !target.Postgres.ManagedLocally {
		return nil
	}
	if !s.canInstallLocally() {
		return errors.New("local PostgreSQL management requires root privileges")
	}

	if _, err := s.runPostgresManager("init-local"); err != nil {
		return err
	}

	args := []string{
		"create-db-user",
		"--user", target.Postgres.User,
		"--db", target.Postgres.DBName,
	}
	if target.Postgres.Password != "" {
		args = append(args, "--password", target.Postgres.Password)
	}
	_, err := s.runPostgresManager(args...)
	return err
}

func (s *DatabaseService) SwitchDatabase(setting *entity.DatabaseSetting) error {
	target, err := s.mergeSettingWithCurrent(setting)
	if err != nil {
		return err
	}
	if err := s.prepareLocalPostgres(target); err != nil {
		return err
	}
	return database.SwitchDatabase(target)
}

func (s *DatabaseService) backupFilename(prefix string) string {
	return fmt.Sprintf("%s-%s.xui-backup", prefix, time.Now().UTC().Format("20060102-150405"))
}

func (s *DatabaseService) saveCurrentRestorePoint(prefix string) (string, error) {
	data, err := database.EncodeCurrentPortableBackup()
	if err != nil {
		return "", err
	}
	path := filepath.Join(config.GetBackupFolderPath(), s.backupFilename(prefix))
	return path, database.SavePortableBackup(path, data)
}

func (s *DatabaseService) ExportPortableBackup() ([]byte, string, error) {
	data, err := database.EncodeCurrentPortableBackup()
	if err != nil {
		return nil, "", err
	}
	return data, s.backupFilename("portable"), nil
}

func (s *DatabaseService) ExportNativeSQLite() ([]byte, string, error) {
	currentCfg, err := s.currentConfig()
	if err != nil {
		return nil, "", err
	}
	if !currentCfg.UsesSQLite() {
		return nil, "", errors.New("native SQLite export is only available when SQLite is the active backend")
	}
	if err := database.Checkpoint(); err != nil {
		return nil, "", err
	}
	contents, err := os.ReadFile(currentCfg.SQLite.Path)
	if err != nil {
		return nil, "", err
	}
	return contents, "x-ui.db", nil
}

func (s *DatabaseService) decodeImport(raw []byte) (*database.BackupSnapshot, string, error) {
	snapshot, err := database.DecodePortableBackup(raw)
	if err == nil {
		return snapshot, "portable", nil
	}

	reader := bytes.NewReader(raw)
	isSQLite, sqliteErr := database.IsSQLiteDB(reader)
	if sqliteErr == nil && isSQLite {
		tempFile, err := os.CreateTemp("", "xui-legacy-*.db")
		if err != nil {
			return nil, "", err
		}
		tempPath := tempFile.Name()
		defer os.Remove(tempPath)
		defer tempFile.Close()

		if _, err := tempFile.Write(raw); err != nil {
			return nil, "", err
		}
		if err := tempFile.Close(); err != nil {
			return nil, "", err
		}
		snapshot, err := database.LoadSnapshotFromSQLiteFile(tempPath)
		if err != nil {
			return nil, "", err
		}
		return snapshot, "sqlite-legacy", nil
	}

	return nil, "", errors.New("unsupported backup format")
}

func (s *DatabaseService) ImportBackup(file multipart.File) (string, error) {
	raw, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	snapshot, backupType, err := s.decodeImport(raw)
	if err != nil {
		return "", err
	}
	if _, err := s.saveCurrentRestorePoint("restore"); err != nil {
		return "", err
	}
	if err := database.ApplySnapshot(database.GetDB(), snapshot); err != nil {
		return "", err
	}

	inboundService := &InboundService{}
	inboundService.MigrateDB()
	logger.Infof("Database import completed using %s backup", backupType)
	return backupType, nil
}
