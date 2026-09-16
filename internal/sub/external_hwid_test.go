package sub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// #6559: the Master panel must send a stable X-HWID when fetching external
// subscriptions, otherwise an HWID-limited donor answers 404.
func TestServerHwidStableAcrossCalls(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	first := serverHwid()
	if first == "" {
		t.Fatal("serverHwid returned empty")
	}
	if !strings.HasPrefix(first, "3x-ui-server-") {
		t.Fatalf("unexpected hwid format: %q", first)
	}
	if len(first) < 6 {
		t.Fatalf("hwid too short for donor minHwidLength: %q", first)
	}

	second := serverHwid()
	if second != first {
		t.Fatalf("hwid not stable: %q vs %q", first, second)
	}

	var row model.Setting
	if err := database.GetDB().Where("key = ?", serverHwidKey).First(&row).Error; err != nil {
		t.Fatalf("hwid not persisted: %v", err)
	}
	if row.Value != first {
		t.Fatalf("persisted hwid %q != returned %q", row.Value, first)
	}
}
