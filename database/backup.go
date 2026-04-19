package database

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

const PortableBackupFormatVersion = 1

type BackupManifest struct {
	FormatVersion  int    `json:"formatVersion"`
	CreatedAt      string `json:"createdAt"`
	SourceDriver   string `json:"sourceDriver"`
	AppVersion     string `json:"appVersion"`
	IncludesConfig bool   `json:"includesConfig"`
}

type BackupSnapshot struct {
	Manifest         BackupManifest            `json:"manifest"`
	Users            []model.User             `json:"users"`
	Inbounds         []model.Inbound          `json:"inbounds"`
	ClientTraffics   []xray.ClientTraffic     `json:"clientTraffics"`
	OutboundTraffics []model.OutboundTraffics `json:"outboundTraffics"`
	Settings         []model.Setting          `json:"settings"`
	InboundClientIps []model.InboundClientIps `json:"inboundClientIps"`
	HistoryOfSeeders []model.HistoryOfSeeders `json:"historyOfSeeders"`
}

func newBackupSnapshot(sourceDriver string) *BackupSnapshot {
	return &BackupSnapshot{
		Manifest: BackupManifest{
			FormatVersion:  PortableBackupFormatVersion,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			SourceDriver:   sourceDriver,
			AppVersion:     config.GetVersion(),
			IncludesConfig: false,
		},
	}
}

func loadSnapshotRows(conn *gorm.DB, modelRef any, dest any) error {
	if !conn.Migrator().HasTable(modelRef) {
		return nil
	}
	return conn.Model(modelRef).Order("id ASC").Find(dest).Error
}

// ExportSnapshot extracts a logical snapshot from an arbitrary database connection.
func ExportSnapshot(conn *gorm.DB, sourceDriver string) (*BackupSnapshot, error) {
	snapshot := newBackupSnapshot(sourceDriver)

	if err := loadSnapshotRows(conn, &model.User{}, &snapshot.Users); err != nil {
		return nil, err
	}
	if err := loadSnapshotRows(conn, &model.Inbound{}, &snapshot.Inbounds); err != nil {
		return nil, err
	}
	for i := range snapshot.Inbounds {
		snapshot.Inbounds[i].ClientStats = nil
	}
	if err := loadSnapshotRows(conn, &xray.ClientTraffic{}, &snapshot.ClientTraffics); err != nil {
		return nil, err
	}
	if err := loadSnapshotRows(conn, &model.OutboundTraffics{}, &snapshot.OutboundTraffics); err != nil {
		return nil, err
	}
	if err := loadSnapshotRows(conn, &model.Setting{}, &snapshot.Settings); err != nil {
		return nil, err
	}
	if err := loadSnapshotRows(conn, &model.InboundClientIps{}, &snapshot.InboundClientIps); err != nil {
		return nil, err
	}
	if err := loadSnapshotRows(conn, &model.HistoryOfSeeders{}, &snapshot.HistoryOfSeeders); err != nil {
		return nil, err
	}

	return snapshot, nil
}

// ExportCurrentSnapshot extracts a logical snapshot from the active database.
func ExportCurrentSnapshot() (*BackupSnapshot, error) {
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	return ExportSnapshot(db, GetDriver())
}

// LoadSnapshotFromSQLiteFile extracts a logical snapshot from a legacy SQLite database file.
func LoadSnapshotFromSQLiteFile(path string) (*BackupSnapshot, error) {
	if err := ValidateSQLiteDB(path); err != nil {
		return nil, err
	}
	cfg := config.DefaultDatabaseConfig()
	cfg.Driver = config.DatabaseDriverSQLite
	cfg.SQLite.Path = path
	conn, err := OpenDatabase(cfg)
	if err != nil {
		return nil, err
	}
	defer CloseConnection(conn)
	if err := MigrateModels(conn); err != nil {
		return nil, err
	}
	return ExportSnapshot(conn, config.DatabaseDriverSQLite)
}

// EncodePortableBackup serializes a logical snapshot into the portable .xui-backup format.
func EncodePortableBackup(snapshot *BackupSnapshot) ([]byte, error) {
	if snapshot == nil {
		return nil, errors.New("backup snapshot is nil")
	}

	manifestBytes, err := json.MarshalIndent(snapshot.Manifest, "", "  ")
	if err != nil {
		return nil, err
	}

	payload := struct {
		Users            []model.User             `json:"users"`
		Inbounds         []model.Inbound          `json:"inbounds"`
		ClientTraffics   []xray.ClientTraffic     `json:"clientTraffics"`
		OutboundTraffics []model.OutboundTraffics `json:"outboundTraffics"`
		Settings         []model.Setting          `json:"settings"`
		InboundClientIps []model.InboundClientIps `json:"inboundClientIps"`
		HistoryOfSeeders []model.HistoryOfSeeders `json:"historyOfSeeders"`
	}{
		Users:            snapshot.Users,
		Inbounds:         snapshot.Inbounds,
		ClientTraffics:   snapshot.ClientTraffics,
		OutboundTraffics: snapshot.OutboundTraffics,
		Settings:         snapshot.Settings,
		InboundClientIps: snapshot.InboundClientIps,
		HistoryOfSeeders: snapshot.HistoryOfSeeders,
	}

	dataBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}

	buffer := &bytes.Buffer{}
	archive := zip.NewWriter(buffer)

	manifestWriter, err := archive.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err := manifestWriter.Write(manifestBytes); err != nil {
		return nil, err
	}

	dataWriter, err := archive.Create("data.json")
	if err != nil {
		return nil, err
	}
	if _, err := dataWriter.Write(dataBytes); err != nil {
		return nil, err
	}

	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// EncodeCurrentPortableBackup serializes the active database into the portable backup format.
func EncodeCurrentPortableBackup() ([]byte, error) {
	snapshot, err := ExportCurrentSnapshot()
	if err != nil {
		return nil, err
	}
	return EncodePortableBackup(snapshot)
}

// DecodePortableBackup parses a .xui-backup archive back into a logical snapshot.
func DecodePortableBackup(data []byte) (*BackupSnapshot, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}

	manifestFile, ok := files["manifest.json"]
	if !ok {
		return nil, errors.New("portable backup is missing manifest.json")
	}
	dataFile, ok := files["data.json"]
	if !ok {
		return nil, errors.New("portable backup is missing data.json")
	}

	readZipFile := func(file *zip.File) ([]byte, error) {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}

	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return nil, err
	}
	dataBytes, err := readZipFile(dataFile)
	if err != nil {
		return nil, err
	}

	snapshot := &BackupSnapshot{}
	if err := json.Unmarshal(manifestBytes, &snapshot.Manifest); err != nil {
		return nil, err
	}
	if snapshot.Manifest.FormatVersion != PortableBackupFormatVersion {
		return nil, fmt.Errorf("unsupported backup format version: %d", snapshot.Manifest.FormatVersion)
	}

	payload := struct {
		Users            []model.User             `json:"users"`
		Inbounds         []model.Inbound          `json:"inbounds"`
		ClientTraffics   []xray.ClientTraffic     `json:"clientTraffics"`
		OutboundTraffics []model.OutboundTraffics `json:"outboundTraffics"`
		Settings         []model.Setting          `json:"settings"`
		InboundClientIps []model.InboundClientIps `json:"inboundClientIps"`
		HistoryOfSeeders []model.HistoryOfSeeders `json:"historyOfSeeders"`
	}{}

	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		return nil, err
	}

	snapshot.Users = payload.Users
	snapshot.Inbounds = payload.Inbounds
	snapshot.ClientTraffics = payload.ClientTraffics
	snapshot.OutboundTraffics = payload.OutboundTraffics
	snapshot.Settings = payload.Settings
	snapshot.InboundClientIps = payload.InboundClientIps
	snapshot.HistoryOfSeeders = payload.HistoryOfSeeders
	return snapshot, nil
}

func clearApplicationTables(tx *gorm.DB) error {
	deleteAll := func(modelRef any) error {
		return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(modelRef).Error
	}

	if err := deleteAll(&xray.ClientTraffic{}); err != nil {
		return err
	}
	if err := deleteAll(&model.OutboundTraffics{}); err != nil {
		return err
	}
	if err := deleteAll(&model.InboundClientIps{}); err != nil {
		return err
	}
	if err := deleteAll(&model.HistoryOfSeeders{}); err != nil {
		return err
	}
	if err := deleteAll(&model.Setting{}); err != nil {
		return err
	}
	if err := deleteAll(&model.Inbound{}); err != nil {
		return err
	}
	if err := deleteAll(&model.User{}); err != nil {
		return err
	}
	return nil
}

func resetPostgresSequence(tx *gorm.DB, tableName string) error {
	var seq string
	if err := tx.Raw("SELECT pg_get_serial_sequence(?, ?)", tableName, "id").Scan(&seq).Error; err != nil {
		return err
	}
	if seq == "" {
		return nil
	}

	var maxID int64
	if err := tx.Raw(fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", tableName)).Scan(&maxID).Error; err != nil {
		return err
	}

	if maxID > 0 {
		return tx.Exec("SELECT setval(CAST(? AS regclass), ?, true)", seq, maxID).Error
	}
	return tx.Exec("SELECT setval(CAST(? AS regclass), ?, false)", seq, 1).Error
}

func resetSequences(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	tables := []string{
		"users",
		"inbounds",
		"client_traffics",
		"outbound_traffics",
		"settings",
		"inbound_client_ips",
		"history_of_seeders",
	}
	for _, tableName := range tables {
		if err := resetPostgresSequence(tx, tableName); err != nil {
			return err
		}
	}
	return nil
}

// ApplySnapshot fully replaces application data in the target database using a logical snapshot.
func ApplySnapshot(conn *gorm.DB, snapshot *BackupSnapshot) error {
	if conn == nil {
		return errors.New("target database is nil")
	}
	if snapshot == nil {
		return errors.New("backup snapshot is nil")
	}
	if err := MigrateModels(conn); err != nil {
		return err
	}

	return conn.Transaction(func(tx *gorm.DB) error {
		if err := clearApplicationTables(tx); err != nil {
			return err
		}

		for i := range snapshot.Inbounds {
			snapshot.Inbounds[i].ClientStats = nil
		}

		if len(snapshot.Users) > 0 {
			if err := tx.Create(&snapshot.Users).Error; err != nil {
				return err
			}
		}
		if len(snapshot.Inbounds) > 0 {
			if err := tx.Create(&snapshot.Inbounds).Error; err != nil {
				return err
			}
		}
		if len(snapshot.ClientTraffics) > 0 {
			if err := tx.Create(&snapshot.ClientTraffics).Error; err != nil {
				return err
			}
		}
		if len(snapshot.OutboundTraffics) > 0 {
			if err := tx.Create(&snapshot.OutboundTraffics).Error; err != nil {
				return err
			}
		}
		if len(snapshot.Settings) > 0 {
			if err := tx.Create(&snapshot.Settings).Error; err != nil {
				return err
			}
		}
		if len(snapshot.InboundClientIps) > 0 {
			if err := tx.Create(&snapshot.InboundClientIps).Error; err != nil {
				return err
			}
		}
		if len(snapshot.HistoryOfSeeders) > 0 {
			if err := tx.Create(&snapshot.HistoryOfSeeders).Error; err != nil {
				return err
			}
		}

		return resetSequences(tx)
	})
}

// SavePortableBackup writes a portable backup archive to disk.
func SavePortableBackup(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
