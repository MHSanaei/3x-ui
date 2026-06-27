package sub

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// statsForClient recovers a client's usage by email when the client_traffics row
// is orphaned — its inbound_id points at an inbound that was deleted and
// recreated, so the preloaded ClientStats and the statsByEmail index both miss.
// Before the email fallback, {{TRAFFIC_USED}} stayed at 0 for such pre-existing
// clients while the sub-info header was correct (#5567).
func TestStatsForClient_OrphanedInboundIdFallback(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const email = "old-client@example.com"
	const total = int64(100) * gb

	db := database.GetDB()
	if err := db.Create(&xray.ClientTraffic{
		InboundId: 999,
		Email:     email,
		Up:        15 * gb,
		Down:      5 * gb,
		Total:     total,
		Enable:    true,
	}).Error; err != nil {
		t.Fatalf("seed orphaned traffic: %v", err)
	}

	s := &SubService{statsByEmail: map[string]xray.ClientTraffic{}}
	inbound := &model.Inbound{Id: 1, Remark: "DE"}
	client := model.Client{Email: email, TotalGB: total, Enable: true}

	st := s.statsForClient(inbound, client)
	if used := st.Up + st.Down; used != 20*gb {
		t.Fatalf("statsForClient used = %d, want %d (email fallback)", used, 20*gb)
	}
	if _, ok := s.statsByEmail[email]; !ok {
		t.Fatalf("email fallback must cache the row into statsByEmail")
	}
	if got := remarkVarValue("TRAFFIC_USED", remarkContext{stats: st}); got != "20.00GB" {
		t.Fatalf("TRAFFIC_USED = %q, want 20.00GB", got)
	}
}
