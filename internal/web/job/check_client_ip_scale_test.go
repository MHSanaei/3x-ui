package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/op/go-logging"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// setupScaleJobDB mirrors the service package's scale gating: Postgres via
// XUI_DB_TYPE/XUI_DB_DSN, SQLite via XUI_SCALE_TEST=1, skip otherwise.
func setupScaleJobDB(t *testing.T) {
	t.Helper()
	loggerInitOnce.Do(func() { xuilogger.InitLogger(logging.ERROR) })
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())

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

func scaleJobSizes(t *testing.T, def ...int) []int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("XUI_SCALE_SIZES"))
	if raw == "" {
		return def
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
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

func resetScaleJobTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if config.GetDBKind() == "postgres" {
		if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate: %v", err)
		}
	} else {
		for _, tbl := range []string{"inbounds", "clients", "client_inbounds"} {
			if err := db.Exec("DELETE FROM " + tbl).Error; err != nil {
				t.Fatalf("delete %s: %v", tbl, err)
			}
		}
		db.Exec("DELETE FROM sqlite_sequence")
	}
	if err := db.Where("1 = 1").Delete(&model.InboundClientIps{}).Error; err != nil {
		t.Fatalf("clear inbound client ips: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&model.NodeClientIp{}).Error; err != nil {
		t.Fatalf("clear node client ips: %v", err)
	}
}

// seedScaleIPDataset seeds n clients across numInbounds inbounds. Every client
// in the LAST inbound carries limitIp=3 (and 0 elsewhere), so hasLimitIp pays
// its full scan cost before finding a hit, and the returned emails all resolve
// to that last inbound for the processObserved measurement.
func seedScaleIPDataset(t *testing.T, n, numInbounds int) []string {
	t.Helper()
	db := database.GetDB()
	resetScaleJobTables(t, db)

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

	var limitedEmails []string
	per := n / numInbounds
	for i := range numInbounds {
		lo, hi := i*per, (i+1)*per
		if i == numInbounds-1 {
			hi = n
		}
		limitIp := 0
		if i == numInbounds-1 {
			limitIp = 3
		}
		clients := make([]model.Client, 0, hi-lo)
		records := make([]*model.ClientRecord, 0, hi-lo)
		for j := lo; j < hi; j++ {
			email := fmt.Sprintf("user-%07d@ipscale", j)
			clients = append(clients, model.Client{Email: email, LimitIP: limitIp, Enable: true})
			records = append(records, &model.ClientRecord{Email: email, LimitIP: limitIp, Enable: true})
			if limitIp > 0 {
				limitedEmails = append(limitedEmails, email)
			}
		}
		settings, err := json.Marshal(map[string][]model.Client{"clients": clients})
		if err != nil {
			t.Fatalf("marshal settings: %v", err)
		}
		ib := &model.Inbound{
			UserId:   1,
			Tag:      fmt.Sprintf("ipscale-%d-%d", n, i),
			Enable:   true,
			Port:     42000 + i,
			Protocol: model.VLESS,
			Settings: string(settings),
		}
		if err := tx.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound %d: %v", i, err)
		}
		if err := tx.CreateInBatches(records, 500).Error; err != nil {
			t.Fatalf("seed clients %d: %v", i, err)
		}
		links := make([]model.ClientInbound, len(records))
		for j := range records {
			links[j] = model.ClientInbound{ClientId: records[j].Id, InboundId: ib.Id}
		}
		if err := tx.CreateInBatches(links, 1000).Error; err != nil {
			t.Fatalf("seed client_inbounds %d: %v", i, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}
	committed = true
	db.Exec("ANALYZE")
	return limitedEmails
}

// TestCheckClientIpScale measures the @every 10s ip-limit job pieces: the
// hasLimitIp gate (settings LIKE scan + full JSON parse of every matching
// inbound) and processObserved with M online users (per-email inbound lookup,
// settings parse and autocommit save). Run twice: first scan half add / half
// update, second scan all update path.
func TestCheckClientIpScale(t *testing.T) {
	shapes := []struct {
		name     string
		inbounds int
		observed int
	}{{"single", 1, 50}, {"spread50", 50, 1000}}

	for _, n := range scaleJobSizes(t, 10000, 100000) {
		for _, shape := range shapes {
			t.Run(fmt.Sprintf("N=%d_%s", n, shape.name), func(t *testing.T) {
				setupScaleJobDB(t)
				limited := seedScaleIPDataset(t, n, shape.inbounds)
				m := min(shape.observed, len(limited))

				j := NewCheckClientIpJob()
				const reps = 3
				start := time.Now()
				for range reps {
					if !j.hasLimitIp() {
						t.Fatal("hasLimitIp = false, want true")
					}
				}
				t.Logf("N=%-7d shape=%-8s hasLimitIp=%v/call", n, shape.name, (time.Since(start) / reps).Round(time.Millisecond))

				now := time.Now().Unix()
				observed := make(map[string]map[string]int64, m)
				for i := range m {
					observed[limited[i]] = map[string]int64{
						fmt.Sprintf("10.0.%d.%d", i/250, i%250+1): now,
					}
				}
				for i := range m / 2 {
					seedClientIps(t, limited[i], []IPWithTimestamp{{IP: "10.99.0.1", Timestamp: now - 60}})
				}

				start = time.Now()
				j.processObserved(observed, true, true)
				firstScan := time.Since(start)
				start = time.Now()
				j.processObserved(observed, true, true)
				secondScan := time.Since(start)
				t.Logf("N=%-7d shape=%-8s processObserved M=%-5d first=%-10v second=%-10v (%.1fms/email)",
					n, shape.name, m, firstScan.Round(time.Millisecond), secondScan.Round(time.Millisecond),
					float64(secondScan.Milliseconds())/float64(m))

				var rows int64
				if err := database.GetDB().Model(&model.InboundClientIps{}).Count(&rows).Error; err != nil {
					t.Fatalf("count ip rows: %v", err)
				}
				if rows != int64(m) {
					t.Fatalf("inbound_client_ips rows = %d, want %d", rows, m)
				}
			})
		}
	}
}
