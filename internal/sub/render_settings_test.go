package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func seedRenderSettings(t testing.TB, rows ...model.Setting) {
	t.Helper()
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
}

func isRenderSettingQuery(db *gorm.DB) bool {
	if db.Statement.Table != "settings" {
		return false
	}
	for _, arg := range db.Statement.Vars {
		switch arg {
		case "subHostSortMode", "datepicker", "subShowIdentityOnAllLinks", "subInfoNodeEnable",
			"subExpiredTemplate", "subTrafficDepletedTemplate", "subCalendarExpireInclusive", "timeLocation":
			return true
		}
	}
	return false
}

func countRenderSettingQueries(t testing.TB) *atomic.Int64 {
	t.Helper()
	var reads atomic.Int64
	db := database.GetDB()
	const name = "test:count_render_settings"
	if err := db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if isRenderSettingQuery(tx) {
			reads.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
	return &reads
}

func TestSubscriptionRenderSettingsQueryBudget(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "render-settings", "render-link", 4932, 1, wsTLSStream)
	seedRenderSettings(t, model.Setting{Key: "subCalendarExpireInclusive", Value: "1"},
		model.Setting{Key: "timeLocation", Value: "UTC"})
	gin.SetMode(gin.TestMode)
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
	reads := countRenderSettingQueries(t)

	for _, format := range []string{"sub", "json", "clash"} {
		t.Run(format, func(t *testing.T) {
			reads.Store(0)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "http://sub.example.com/"+format+"/render-settings", nil))
			body := res.Body.String()
			if format == "sub" {
				raw, err := base64.StdEncoding.DecodeString(body)
				if err != nil {
					t.Fatalf("decode subscription: %v", err)
				}
				body = string(raw)
			}
			if res.Code != http.StatusOK || !strings.Contains(body, "render-link") {
				t.Fatalf("expected rendered inbound, status=%d body=%q", res.Code, body)
			}
			if got := reads.Load(); got != 1 {
				t.Fatalf("render settings queries = %d, want 1", got)
			}
		})
	}
}

func TestSubscriptionRenderSettingsValues(t *testing.T) {
	seedSubDB(t)
	logger.InitLogger(logging.ERROR)
	base := NewSubService("")
	checkDefaults := func(req *SubService) {
		t.Helper()
		if req.hostsFirst || req.datepicker != "gregorian" || req.showIdentityOnAllLinks || req.subInfoNodeEnable ||
			req.subExpiredTemplate != service.DefaultSubExpiredTemplate ||
			req.subTrafficDepletedTemplate != service.DefaultSubTrafficDepletedTemplate ||
			req.subCalendarExpireInclusive || req.calendarExpireLocation != nil {
			t.Fatalf("wrong defaults: date=%q expired=%q depleted=%q hosts=%v identity=%v info=%v inclusive=%v tz=%v",
				req.datepicker, req.subExpiredTemplate, req.subTrafficDepletedTemplate, req.hostsFirst,
				req.showIdentityOnAllLinks, req.subInfoNodeEnable, req.subCalendarExpireInclusive, req.calendarExpireLocation)
		}
	}
	checkDefaults(base.ForRequest("sub.example.com"))

	seedRenderSettings(t,
		model.Setting{Key: "subHostSortMode", Value: "hosts-first"},
		model.Setting{Key: "subHostSortMode", Value: "inbound-first"},
		model.Setting{Key: "datepicker", Value: ""},
		model.Setting{Key: "datepicker", Value: "jalali"},
		model.Setting{Key: "subShowIdentityOnAllLinks", Value: "TRUE"},
		model.Setting{Key: "subInfoNodeEnable", Value: "invalid"},
		model.Setting{Key: "subExpiredTemplate", Value: ""},
		model.Setting{Key: "subTrafficDepletedTemplate", Value: "out-of-quota"},
		model.Setting{Key: "subCalendarExpireInclusive", Value: "T"},
		model.Setting{Key: "timeLocation", Value: "invalid-zone"},
	)
	base.datepicker = "jalali"
	base.subExpiredTemplate = "custom-expired"
	first := base.ForRequest("sub.example.com")
	if !first.hostsFirst || first.datepicker != "jalali" || !first.showIdentityOnAllLinks || first.subInfoNodeEnable ||
		first.subExpiredTemplate != "custom-expired" || first.subTrafficDepletedTemplate != "out-of-quota" ||
		!first.subCalendarExpireInclusive || first.calendarExpireLocation != time.Local {
		t.Fatalf("legacy values were not preserved: date=%q expired=%q depleted=%q hosts=%v identity=%v info=%v inclusive=%v tz=%v",
			first.datepicker, first.subExpiredTemplate, first.subTrafficDepletedTemplate, first.hostsFirst,
			first.showIdentityOnAllLinks, first.subInfoNodeEnable, first.subCalendarExpireInclusive, first.calendarExpireLocation)
	}
	// A blank timezone is UTC, whereas a missing timezone uses Local.
	db := database.GetDB()
	if err := db.Exec("UPDATE settings SET value = ? WHERE key = ?", "", "timeLocation").Error; err != nil {
		t.Fatal(err)
	}
	if req := base.ForRequest("sub.example.com"); req.calendarExpireLocation != time.UTC {
		t.Fatalf("empty timezone = %v, want UTC", req.calendarExpireLocation)
	}
	if err := db.Exec("DELETE FROM settings WHERE key = ?", "subExpiredTemplate").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE settings SET value = ? WHERE key = ?", "", "subCalendarExpireInclusive").Error; err != nil {
		t.Fatal(err)
	}
	var requests sync.WaitGroup
	for i := range 8 {
		requests.Go(func() {
			host := fmt.Sprintf("sub%d.example.com", i)
			req := base.ForRequest(host)
			if req.address != host || req.subExpiredTemplate != service.DefaultSubExpiredTemplate ||
				req.subCalendarExpireInclusive || req.calendarExpireLocation != nil {
				t.Errorf("request %s: address=%q expired=%q inclusive=%v tz=%v", host, req.address,
					req.subExpiredTemplate, req.subCalendarExpireInclusive, req.calendarExpireLocation)
			}
		})
	}
	requests.Wait()
	if base.address != "" || base.subExpiredTemplate != "custom-expired" || first.subExpiredTemplate != "custom-expired" {
		t.Fatal("later requests mutated the base or prior request")
	}
}

func TestSubscriptionRenderSettingsPartialReadFailure(t *testing.T) {
	seedSubDB(t)
	seedRenderSettings(t, model.Setting{Key: "datepicker", Value: "jalali"},
		model.Setting{Key: "subExpiredTemplate", Value: "custom-expired"},
		model.Setting{Key: "subTrafficDepletedTemplate", Value: "custom-depleted"},
		model.Setting{Key: "subCalendarExpireInclusive", Value: "true"},
		model.Setting{Key: "timeLocation", Value: "UTC"})
	db := database.GetDB()
	const name = "test:fail_some_render_settings"
	if err := db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != "settings" {
			return
		}
		if _, single := tx.Statement.Dest.(*model.Setting); !single {
			tx.Where("missing_test_column = 1")
			return
		}
		where, _ := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
		for _, expr := range where.Exprs {
			if condition, ok := expr.(clause.Expr); ok && len(condition.Vars) == 1 &&
				(condition.Vars[0] == "subExpiredTemplate" || condition.Vars[0] == "timeLocation") {
				tx.Where("missing_test_column = 1")
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })
	req := NewSubService("").ForRequest("sub.example.com")
	if req.datepicker != "jalali" || req.subExpiredTemplate != service.DefaultSubExpiredTemplate ||
		req.subTrafficDepletedTemplate != "custom-depleted" || !req.subCalendarExpireInclusive || req.calendarExpireLocation != nil {
		t.Fatalf("partial failure: date=%q expired=%q depleted=%q inclusive=%v tz=%v",
			req.datepicker, req.subExpiredTemplate, req.subTrafficDepletedTemplate, req.subCalendarExpireInclusive, req.calendarExpireLocation)
	}
}

// A wildcard-listen inbound resolves its link host from subDomain/webDomain; reading
// those per link cost two more settings queries for every link in the subscription.
func TestSubscriptionRequestReadsSettingsOncePerRequest(t *testing.T) {
	seedSubDB(t)
	db := database.GetDB()
	for i := range 3 {
		ib := seedSubInbound(t, "one-read", fmt.Sprintf("wild-%d", i), 4940+i, i+1, wsTLSStream)
		if err := db.Model(ib).Update("listen", "").Error; err != nil {
			t.Fatal(err)
		}
	}
	seedRenderSettings(t, model.Setting{Key: "webDomain", Value: "panel.example.com"})
	gin.SetMode(gin.TestMode)
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
	var reads atomic.Int64
	const name = "test:count_settings"
	if err := db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			reads.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(name) })

	for _, format := range []string{"sub", "json", "clash"} {
		t.Run(format, func(t *testing.T) {
			reads.Store(0)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "http://203.0.113.9/"+format+"/one-read", nil))
			body := res.Body.String()
			if format == "sub" {
				raw, err := base64.StdEncoding.DecodeString(body)
				if err != nil {
					t.Fatalf("decode subscription: %v", err)
				}
				body = string(raw)
			}
			if got := strings.Count(body, "panel.example.com"); res.Code != http.StatusOK || got != 3 {
				t.Fatalf("status=%d, links on the configured host = %d, want 3; body=%q", res.Code, got, body)
			}
			if got := reads.Load(); got != 1 {
				t.Fatalf("settings queries = %d, want 1", got)
			}
		})
	}
}
