package service

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func sqliteImportHostValues(machine string) map[string]string {
	port, address := "2053", "127.0.0.2"
	if machine == "source" {
		port, address = "2087", "127.0.0.3"
	}
	return map[string]string{
		"webListen": address, "webDomain": machine + ".example", "webPort": port,
		"webCertFile": "/" + machine + "/panel.crt", "webKeyFile": "/" + machine + "/panel.key", "webBasePath": "/" + machine + "/",
		"subListen": address, "subDomain": "sub." + machine + ".example", "subPort": port,
		"subCertFile": "/" + machine + "/sub.crt", "subKeyFile": "/" + machine + "/sub.key",
		"subURI": "https://" + machine + ".example/sub/", "subJsonURI": "https://" + machine + ".example/json/",
		"secret": machine + "-secret", "panelGuid": machine + "-guid",
		"nodeMtlsCaCertPem": machine + "-ca-cert", "nodeMtlsCaKeyPem": machine + "-ca-key",
		"nodeMtlsClientCertPem": machine + "-client-cert", "nodeMtlsClientKeyPem": machine + "-client-key",
		"nodeMtlsClientCertSha256": machine + "-client-sha", "nodeMtlsClientCAPem": machine + "-client-ca",
	}
}

func seedSQLiteImportSettings(t *testing.T, values map[string]string, template string) {
	t.Helper()
	db := database.GetDB()
	if err := db.Exec("DELETE FROM settings").Error; err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Setting{Key: "remarkTemplate", Value: template}).Error; err != nil {
		t.Fatal(err)
	}
}

// Every migration gets its own schema, even when CI supplies a shared database.
func sqliteImportPostgresDestination(t *testing.T, dsn string) {
	t.Helper()
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	schema := fmt.Sprintf("import_host_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.CloseDB()
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	scoped := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		scoped = parsed.String()
	}
	t.Setenv("XUI_DB_TYPE", "postgres")
	t.Setenv("XUI_DB_DSN", scoped)
	if err := database.InitDB(""); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.User{Id: 101, Username: "destination-user", Password: "test-password"}).Error; err != nil {
		t.Fatal(err)
	}
}

// Database import/reopen is real; the core executable is a harmless local stub.
// No real Xray binary, network listener, or production configuration is used.
func sqliteImportTestCore(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", bin)
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())
	stub := "#!/bin/sh\nif [ \"$1\" = -version ]; then echo 'Xray 0.0.0 test'; exit 0; fi\nexec sleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, xray.GetBinaryName()), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	previousProcess, previousResult := xrayState.snapshot()
	previousHeldBack := xrayState.heldBackReason()
	previousStopped := isManuallyStopped.Load()
	xrayState.replace(nil)
	t.Cleanup(func() {
		if process := currentXrayProcess(); process != nil {
			_ = process.Stop()
		}
		xrayState.mu.Lock()
		xrayState.process, xrayState.result, xrayState.heldBack = previousProcess, previousResult, previousHeldBack
		xrayState.mu.Unlock()
		isManuallyStopped.Store(previousStopped)
	})
}

func TestImportSQLiteIntoPostgresHostSettings(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("XUI_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("set XUI_TEST_PG_DSN to run real SQLite-to-PostgreSQL imports")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the isolated core executable fixture requires a POSIX environment")
	}
	t.Setenv("XUI_DB_TYPE", "sqlite")
	t.Setenv("XUI_DB_DSN", "")
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	sourcePath := config.GetDBPath()
	dbtest.InitDB(t, sourcePath)
	sourceValues := sqliteImportHostValues("source")
	seedSQLiteImportSettings(t, sourceValues, "source-template")
	db := database.GetDB()
	if err := db.Model(&model.User{}).Where("id = 1").Update("username", "source-user").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Inbound{UserId: 1, Tag: "source-inbound", Port: 62001, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp"}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientRecord{Email: "source-client", SubID: "source-sub", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	dump, err := database.DumpSQLiteToBytes(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	for format, uploadBytes := range map[string][]byte{"db": data, "dump": dump} {
		for _, keep := range []bool{true, false} {
			for _, profile := range []string{"local", "sparse", "absent", "failed-copy"} {
				t.Run(fmt.Sprintf("%s/keep=%t/%s", format, keep, profile), func(t *testing.T) {
					sqliteImportPostgresDestination(t, dsn)
					sqliteImportTestCore(t)
					local := sqliteImportHostValues("destination")
					switch profile {
					case "sparse":
						local["webCertFile"], local["nodeMtlsClientKeyPem"] = "", ""
						for _, key := range []string{"webKeyFile", "subListen", "nodeMtlsClientCertSha256", "nodeMtlsCaCertPem", "nodeMtlsCaKeyPem", "nodeMtlsClientCAPem"} {
							delete(local, key)
						}
					case "absent":
						clear(local)
					}
					seedSQLiteImportSettings(t, local, "destination-template")
					if err := database.GetDB().Create(&model.Inbound{UserId: 101, Tag: "destination-inbound", Port: 62002, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp"}`}).Error; err != nil {
						t.Fatal(err)
					}
					if err := database.GetDB().Create(&model.ClientRecord{Email: "destination-client", SubID: "destination-sub", Enable: true}).Error; err != nil {
						t.Fatal(err)
					}
					if profile == "failed-copy" {
						for _, sql := range []string{
							`ALTER TABLE settings ADD CONSTRAINT reject_import CHECK (value <> 'source-template')`,
							`CREATE TABLE host_restore_audit (key text)`,
							`CREATE FUNCTION audit_host_restore() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO host_restore_audit VALUES (NEW.key); RETURN NEW; END $$`,
							`CREATE TRIGGER audit_host_restore AFTER UPDATE ON settings FOR EACH ROW EXECUTE FUNCTION audit_host_restore()`,
						} {
							if err := database.GetDB().Exec(sql).Error; err != nil {
								t.Fatal(err)
							}
						}
					}
					uploadPath := filepath.Join(t.TempDir(), "backup."+format)
					if err := os.WriteFile(uploadPath, uploadBytes, 0o600); err != nil {
						t.Fatal(err)
					}
					upload, err := os.Open(uploadPath)
					if err != nil {
						t.Fatal(err)
					}
					defer upload.Close()
					err = (&ServerService{}).ImportDB(upload, keep)
					want := maps.Clone(sourceValues)
					if profile == "failed-copy" {
						if err == nil || !strings.Contains(err.Error(), "Importing the SQLite data into PostgreSQL failed") {
							t.Fatalf("ImportDB error = %v, want a rolled-back migration failure", err)
						}
						want = maps.Clone(local)
						var updates int64
						if err := database.GetDB().Table("host_restore_audit").Count(&updates).Error; err != nil || updates != 0 {
							t.Fatalf("host restoration ran after failed migration: updates=%d, error=%v", updates, err)
						}
					} else {
						if err != nil {
							t.Fatalf("ImportDB: %v", err)
						}
						if keep {
							want = maps.Clone(local)
							for _, key := range []string{"nodeMtlsCaCertPem", "nodeMtlsCaKeyPem", "nodeMtlsClientCertPem", "nodeMtlsClientKeyPem", "nodeMtlsClientCAPem"} {
								if _, exists := local[key]; !exists {
									want[key] = sourceValues[key]
								}
							}
						}
					}
					var settings []model.Setting
					if err := database.GetDB().Find(&settings).Error; err != nil {
						t.Fatal(err)
					}
					got := make(map[string]string)
					for _, row := range settings {
						got[row.Key] = row.Value
					}
					for key := range sourceValues {
						value, exists := got[key]
						wantValue, wantExists := want[key]
						if exists != wantExists || value != wantValue {
							t.Errorf("%s = %q (present=%t), want %q (present=%t)", key, value, exists, wantValue, wantExists)
						}
					}
					username, template, count := "source-user", "source-template", int64(1)
					if profile == "failed-copy" {
						username, template, count = "destination-user", "destination-template", 0
					}
					var user model.User
					if err := database.GetDB().First(&user).Error; err != nil || user.Username != username {
						t.Errorf("user = %q, error=%v, want %q", user.Username, err, username)
					}
					if got["remarkTemplate"] != template {
						t.Errorf("ordinary setting = %q, want %q", got["remarkTemplate"], template)
					}
					for _, check := range []struct {
						model                       any
						column, source, destination string
					}{
						{&model.Inbound{}, "tag", "source-inbound", "destination-inbound"},
						{&model.ClientRecord{}, "email", "source-client", "destination-client"},
					} {
						for value, wantCount := range map[string]int64{check.source: count, check.destination: 1 - count} {
							var rows int64
							if err := database.GetDB().Model(check.model).Where(check.column+" = ?", value).Count(&rows).Error; err != nil || rows != wantCount {
								t.Errorf("%s count=%d, error=%v, want %d", value, rows, err, wantCount)
							}
						}
					}
				})
			}
		}
	}
}
