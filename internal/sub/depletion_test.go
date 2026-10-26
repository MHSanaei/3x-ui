package sub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSharedSubscriptionRequiresEveryKnownDepletion(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	now := time.Now().UnixMilli()
	rows := []xray.ClientTraffic{
		{Email: "first", Up: 100, Total: 100, DepletedAt: now - 2000, DepletedReason: "traffic"},
		{Email: "last", ExpiryTime: now - 1000, DepletedAt: now - 1000, DepletedReason: "expiry"},
		{Email: "healthy", Up: 1, Total: 100},
		{Email: "unknown", Up: 100, Total: 100},
		{Email: "recovered", Up: 1, Total: 100, DepletedAt: now - 500, DepletedReason: "traffic"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		emails []string
		at     int64
		reason string
	}{
		{"all depleted", []string{"first", "last", "first"}, now - 1000, "both"},
		{"single", []string{"first", "first"}, now - 2000, "traffic"},
		{"healthy member", []string{"first", "healthy"}, 0, ""},
		{"unknown event", []string{"first", "unknown"}, 0, "traffic"},
		{"missing row", []string{"first", "missing"}, 0, ""},
		{"stale marker", []string{"first", "recovered"}, 0, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			traffic, _ := (&SubService{}).AggregateTrafficByEmails(tt.emails)
			if traffic.DepletedAt != tt.at || traffic.DepletedReason != tt.reason {
				t.Fatalf("depletion = %d/%q, want %d/%q", traffic.DepletedAt, traffic.DepletedReason, tt.at, tt.reason)
			}
		})
	}
}

func TestSharedSubscriptionDepletionBatchesAndDeduplicatesIdentities(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	const first = int64(1700000000000)
	rows := make([]xray.ClientTraffic, 401)
	emails := make([]string, 0, 403)
	for i := range rows {
		rows[i] = xray.ClientTraffic{Email: fmt.Sprintf("shared-batch-%03d", i), Up: 1, Total: 1, DepletedAt: first + int64(i), DepletedReason: "traffic"}
		emails = append(emails, rows[i].Email)
	}
	if err := db.CreateInBatches(&rows, 100).Error; err != nil {
		t.Fatal(err)
	}
	emails = append(emails, emails[0], emails[0])
	queries := 0
	const callback = "shared-depletion-query-budget"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(*gorm.DB) { queries++ }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	got, _ := (&SubService{}).AggregateTrafficByEmails(emails)
	if got.DepletedAt != first+400 || got.DepletedReason != "traffic" || got.Up != 401 || got.Total != 401 {
		t.Fatalf("shared aggregate = %+v", got)
	}
	if queries != 3 {
		t.Fatalf("401 members used %d queries, want two traffic batches and the existing limit lookup", queries)
	}
}

func TestSubscriptionInfoAndTemplateExposeDepletion(t *testing.T) {
	oldFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldFS })
	initSubDB(t)
	seedInfoEndpointSub(t, "depletion-info", "depleted@x")
	db := database.GetDB()
	at := time.Now().Add(-time.Minute).UnixMilli()
	if err := db.Create(&xray.ClientTraffic{Email: "depleted@x", Up: 100, Total: 100, DepletedAt: at, DepletedReason: "traffic"}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewSUBController(router.Group("/"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/depletion-info?format=info", nil))
	var info map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || info["depletedAt"] != float64(at) || info["depletedReason"] != "traffic" {
		t.Fatalf("info response = %d %s", w.Code, w.Body.String())
	}
	theme := t.TempDir()
	if err := os.WriteFile(filepath.Join(theme, "index.html"), []byte(`{{ .depletedAt }}|{{ .depletedReason }}|{{ .sId }}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Setting{Key: "subThemeDir", Value: theme}).Error; err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sub/depletion-info", nil)
	req.Header.Set("Accept", "text/html")
	router.ServeHTTP(w, req)
	if got := w.Body.String(); got != fmt.Sprintf("%d|traffic|depletion-info", at) {
		t.Fatalf("template = %q", got)
	}
	if err := db.Create(&model.ClientRecord{Email: "hidden-member", SubID: "depletion-info", Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sub/depletion-info?format=info", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info["depletedAt"] != float64(0) || info["depletedReason"] != "" {
		t.Fatalf("a member without links must prevent whole-subscription depletion: %s", w.Body.String())
	}
}
