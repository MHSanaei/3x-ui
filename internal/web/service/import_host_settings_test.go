package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// An imported database carries the source machine's listen addresses,
// certificates and node identity. Keeping this machine's own values is what
// stops the panel from becoming unreachable on its own address after a restore.
func TestImportKeepsHostBoundSettings(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	mine := map[string]string{
		"webPort":               "8443",
		"webCertFile":           "/etc/ssl/this-host.pem",
		"webBasePath":           "/mine/",
		"subURI":                "https://this-host.example/sub/",
		"panelGuid":             "this-host-guid",
		"nodeMtlsClientCertPem": "this-host-leaf",
	}
	for key, value := range mine {
		if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	// A setting that belongs to the configuration, not the machine.
	if err := db.Create(&model.Setting{Key: "remarkTemplate", Value: "mine"}).Error; err != nil {
		t.Fatal(err)
	}

	kept := captureHostBoundSettings()
	if len(kept.values) != len(mine) {
		t.Fatalf("captured %d host settings, want %d: %v", len(kept.values), len(mine), kept.values)
	}

	// Stand in for the import: every row now holds the source machine's value.
	for key := range mine {
		if err := db.Model(&model.Setting{}).Where("key = ?", key).
			Update("value", "from-imported-file").Error; err != nil {
			t.Fatalf("overwrite %s: %v", key, err)
		}
	}
	if err := db.Model(&model.Setting{}).Where("key = ?", "remarkTemplate").
		Update("value", "from-imported-file").Error; err != nil {
		t.Fatal(err)
	}

	restoreHostBoundSettings(kept)

	for key, want := range mine {
		var got model.Setting
		if err := db.Where("key = ?", key).First(&got).Error; err != nil {
			t.Fatalf("read back %s: %v", key, err)
		}
		if got.Value != want {
			t.Fatalf("setting %s = %q after import, want this machine's %q", key, got.Value, want)
		}
	}

	var carried model.Setting
	if err := db.Where("key = ?", "remarkTemplate").First(&carried).Error; err != nil {
		t.Fatal(err)
	}
	if carried.Value != "from-imported-file" {
		t.Fatalf("remarkTemplate = %q, want the imported value: only host-bound keys may survive", carried.Value)
	}
}

// The destination usually has no row at all for the certificate paths and the
// node identity — the built-in default applies. The imported row must go, or
// the panel quietly adopts the source machine's certificate path.
func TestImportDropsHostBoundSettingsThisMachineNeverHad(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	kept := captureHostBoundSettings()

	for _, key := range []string{"webCertFile", "subCertFile", "nodeMtlsClientCertPem"} {
		if err := db.Create(&model.Setting{Key: key, Value: "from-imported-file"}).Error; err != nil {
			t.Fatalf("seed imported %s: %v", key, err)
		}
	}

	restoreHostBoundSettings(kept)

	for _, key := range []string{"webCertFile", "subCertFile", "nodeMtlsClientCertPem"} {
		var count int64
		if err := db.Model(&model.Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("imported %s survived although this machine had no row for it", key)
		}
	}
}
