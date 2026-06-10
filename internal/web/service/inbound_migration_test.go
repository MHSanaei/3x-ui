package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestMigrationRequirements_BackfillsClientTrafficsWithMultiDomainInbound guards the
// PostgreSQL fix where the externalProxy detection query (executed via .Scan) errored on
// json_extract and rolled back the whole transaction — including the client_traffics
// backfill at inbound.go:3093-3106, leaving clients with no traffic rows. A MultiDomain
// inbound is present so that query returns rows and the function runs to completion; both
// the backfill and the MultiDomain→ExternalProxy migration must then commit.
func TestMigrationRequirements_BackfillsClientTrafficsWithMultiDomainInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const backfillEmail = "needsbackfill@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c010"

	// Inbound A: a client present only in settings.clients, with no client_traffics row.
	clientInbound := &model.Inbound{
		UserId:         1,
		Tag:            "a-tag",
		Enable:         true,
		Port:           30001,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"email":"` + backfillEmail + `","id":"` + uid + `","enable":true}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(clientInbound).Error; err != nil {
		t.Fatalf("create client inbound: %v", err)
	}

	// Inbound B: a legacy MultiDomain inbound whose tag carries the 0.0.0.0: prefix.
	// Its presence makes the externalProxy query return rows, so the function does not
	// early-return and reaches the tag-cleanup statement.
	multiDomainInbound := &model.Inbound{
		UserId:         1,
		Tag:            "inbound-0.0.0.0:30002",
		Enable:         true,
		Port:           30002,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"security":"tls","tlsSettings":{"settings":{"domains":[{"domain":"example.com"}]}}}`,
	}
	if err := db.Create(multiDomainInbound).Error; err != nil {
		t.Fatalf("create multidomain inbound: %v", err)
	}

	var before int64
	if err := db.Model(xray.ClientTraffic{}).Count(&before).Error; err != nil {
		t.Fatalf("count client_traffics before: %v", err)
	}
	if before != 0 {
		t.Fatalf("expected no client_traffics before migration, got %d", before)
	}

	svc := InboundService{}
	svc.MigrationRequirements()

	// The backfill must have committed: the settings-only client now owns a row.
	// Before the fix this was rolled back whenever the externalProxy detection query
	// errored (it does on Postgres via json_extract), so the MultiDomain inbound below
	// is deliberately present to make that query return rows and run to completion.
	var ct xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", backfillEmail).First(&ct).Error; err != nil {
		t.Fatalf("client_traffics row not backfilled for %s: %v", backfillEmail, err)
	}

	// The MultiDomain→ExternalProxy migration must have committed too: the detection
	// query ran (.Scan executes it) and the loop rewrote the inbound's streamSettings.
	var refreshed model.Inbound
	if err := db.First(&refreshed, multiDomainInbound.Id).Error; err != nil {
		t.Fatalf("reload multidomain inbound: %v", err)
	}
	if !strings.Contains(refreshed.StreamSettings, "externalProxy") {
		t.Errorf("MultiDomain migration did not commit; streamSettings = %q", refreshed.StreamSettings)
	}
}
