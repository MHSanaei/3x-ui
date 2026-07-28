package sub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/op/go-logging"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const scaleTargetSubId = "scale-target-sub"

// setupScaleSubDB mirrors the service package's scale gating: Postgres via
// XUI_DB_TYPE/XUI_DB_DSN, SQLite via XUI_SCALE_TEST=1, skip otherwise.
func setupScaleSubDB(t *testing.T) {
	t.Helper()
	xuilogger.InitLogger(logging.ERROR)

	if os.Getenv("XUI_DB_TYPE") == "postgres" && strings.TrimSpace(os.Getenv("XUI_DB_DSN")) != "" {
		if err := database.InitDB(""); err != nil {
			t.Fatalf("InitDB(postgres): %v", err)
		}
		t.Cleanup(func() { _ = database.CloseDB() })
		return
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("XUI_SCALE_TEST"))) {
	case "1", "true", "yes":
		if err := database.InitDB(filepath.Join(t.TempDir(), "scale.db")); err != nil {
			t.Fatalf("InitDB(sqlite): %v", err)
		}
		t.Cleanup(func() { _ = database.CloseDB() })
		return
	}
	t.Skip("set XUI_SCALE_TEST=1 (sqlite) or XUI_DB_TYPE=postgres + XUI_DB_DSN (postgres) to run the scale benchmark")
}

func scaleSubSizes(t *testing.T, def ...int) []int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("XUI_SCALE_SIZES"))
	if raw == "" {
		return def
	}
	var out []int
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 {
			t.Fatalf("XUI_SCALE_SIZES: invalid size %q", part)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func resetScaleSubTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if config.GetDBKind() == "postgres" {
		if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate: %v", err)
		}
	} else {
		for _, tbl := range []string{"inbounds", "clients", "client_inbounds", "client_traffics"} {
			if err := db.Exec("DELETE FROM " + tbl).Error; err != nil {
				t.Fatalf("delete %s: %v", tbl, err)
			}
		}
		db.Exec("DELETE FROM sqlite_sequence")
	}
	if err := db.Where("1 = 1").Delete(&model.ClientExternalLink{}).Error; err != nil {
		t.Fatalf("clear client_external_links: %v", err)
	}
}

// seedScaleSubDataset seeds one VLESS inbound holding n clients (the sub
// server's worst case: matchingClients parses the whole settings blob and
// getInboundsBySubId preloads every ClientStats row). Three clients share
// scaleTargetSubId; everyone else gets a unique subId.
func seedScaleSubDataset(t *testing.T, n int) {
	t.Helper()
	db := database.GetDB()
	resetScaleSubTables(t, db)

	clients := make([]model.Client, n)
	exp := time.Now().AddDate(1, 0, 0).UnixMilli()
	targets := map[int]bool{n / 4: true, n / 2: true, 3 * n / 4: true}
	for i := range n {
		subId := fmt.Sprintf("sub-%07d", i)
		if targets[i] {
			subId = scaleTargetSubId
		}
		clients[i] = model.Client{
			ID:         uuid.NewString(),
			Email:      fmt.Sprintf("user-%07d@subscale", i),
			SubID:      subId,
			Enable:     true,
			ExpiryTime: exp,
			TotalGB:    100 << 30,
		}
	}

	settingsMap := map[string]any{"clients": clients, "decryption": "none"}
	settings, err := json.Marshal(settingsMap)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin seed tx: %v", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	ib := &model.Inbound{
		UserId:         1,
		Tag:            fmt.Sprintf("subscale-%d", n),
		Remark:         "subscale",
		Enable:         true,
		Listen:         "203.0.113.1",
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       string(settings),
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := tx.Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	records := make([]*model.ClientRecord, n)
	for i := range clients {
		records[i] = clients[i].ToRecord()
	}
	if err := tx.CreateInBatches(records, 500).Error; err != nil {
		t.Fatalf("seed clients: %v", err)
	}
	links := make([]model.ClientInbound, n)
	for i := range records {
		links[i] = model.ClientInbound{ClientId: records[i].Id, InboundId: ib.Id}
	}
	if err := tx.CreateInBatches(links, 1000).Error; err != nil {
		t.Fatalf("seed client_inbounds: %v", err)
	}
	traffics := make([]xray.ClientTraffic, n)
	for i := range clients {
		traffics[i] = xray.ClientTraffic{
			InboundId:  ib.Id,
			Email:      clients[i].Email,
			Enable:     true,
			Total:      clients[i].TotalGB,
			ExpiryTime: clients[i].ExpiryTime,
		}
	}
	if err := tx.CreateInBatches(traffics, 1000).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}
	committed = true
	db.Exec("ANALYZE")
}

// TestGetSubsScale measures one subscription fetch (raw and JSON format) for a
// 3-client subId living inside an n-client inbound, plus a subId miss — the
// per-request cost every subscriber pays.
func TestGetSubsScale(t *testing.T) {
	for _, n := range scaleSubSizes(t, 10000, 100000) {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			setupScaleSubDB(t)
			seedScaleSubDataset(t, n)

			svc := &SubService{}
			const reps = 5
			start := time.Now()
			var links []string
			for range reps {
				var err error
				links, _, _, _, err = svc.GetSubs(scaleTargetSubId, "sub.example.com")
				if err != nil {
					t.Fatalf("GetSubs: %v", err)
				}
			}
			rawDur := time.Since(start) / reps
			if len(links) != 3 {
				t.Fatalf("GetSubs links = %d, want 3", len(links))
			}

			jsonSvc := NewSubJsonService("", "", "", &SubService{})
			start = time.Now()
			for range reps {
				body, _, err := jsonSvc.GetJson(scaleTargetSubId, "sub.example.com", false)
				if err != nil {
					t.Fatalf("GetJson: %v", err)
				}
				if body == "" {
					t.Fatal("GetJson returned empty body")
				}
			}
			jsonDur := time.Since(start) / reps

			start = time.Now()
			for range reps {
				missLinks, _, _, _, err := svc.GetSubs("no-such-sub", "sub.example.com")
				if err != nil {
					t.Fatalf("GetSubs miss: %v", err)
				}
				if len(missLinks) != 0 {
					t.Fatalf("GetSubs miss links = %d, want 0", len(missLinks))
				}
			}
			missDur := time.Since(start) / reps

			t.Logf("N=%-7d raw=%-10v json=%-10v miss=%v",
				n, rawDur.Round(time.Millisecond), jsonDur.Round(time.Millisecond), missDur.Round(time.Millisecond))
		})
	}
}
