package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func hostColumns() []string {
	return []string{
		"id", "group_id", "inbound_id", "sort_order", "remark", "server_description", "is_disabled", "is_hidden", "tags",
		"address", "port",
		"security", "sni", "host_header", "path", "alpn", "fingerprint",
		"override_sni_from_address", "keep_sni_blank", "pinned_peer_cert_sha256",
		"verify_peer_cert_by_name", "allow_insecure", "ech_config_list",
		"mux_params", "sockopt_params", "final_mask", "vless_route",
		"exclude_from_sub_types", "mihomo_ip_version", "mihomo_x25519", "shuffle_host", "node_guids",
		"created_at", "updated_at",
	}
}

func assertHostSchema(t *testing.T) {
	t.Helper()
	m := GetDB().Migrator()
	if !m.HasTable("hosts") {
		t.Fatalf("hosts table not created by initModels")
	}
	for _, col := range hostColumns() {
		if !m.HasColumn(&model.Host{}, col) {
			t.Fatalf("hosts table missing column %q", col)
		}
	}
}

// TestHostAutoMigrateCreatesColumns verifies the hosts table and every expected
// column exist after initModels (SQLite).
func TestHostAutoMigrateCreatesColumns(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	assertHostSchema(t)
}

// TestHostAutoMigrateCreatesColumns_Postgres is the dual-driver counterpart.
func TestHostAutoMigrateCreatesColumns_Postgres(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres schema test")
	}
	if err := InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	assertHostSchema(t)
}

// TestPruneOrphanedHosts verifies a host whose inbound_id has no matching inbound
// is removed by the prune step.
func TestPruneOrphanedHosts(t *testing.T) {
	if err := InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	db := GetDB()

	orphan := &model.Host{InboundId: 99999, Remark: "orphan"}
	if err := db.Create(orphan).Error; err != nil {
		t.Fatalf("create orphan host: %v", err)
	}
	if err := pruneOrphanedHosts(); err != nil {
		t.Fatalf("pruneOrphanedHosts: %v", err)
	}
	var cnt int64
	if err := db.Model(&model.Host{}).Where("id = ?", orphan.Id).Count(&cnt).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("orphan host not pruned, count=%d", cnt)
	}
}
