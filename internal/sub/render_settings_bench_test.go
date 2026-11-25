package sub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
)

func BenchmarkSubscriptionRenderSettings(b *testing.B) {
	logger.InitLogger(logging.ERROR)
	if os.Getenv("XUI_DB_TYPE") == "postgres" {
		cleanup, err := testpg.IsolatePackage("subscription_render_bench")
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(cleanup)
	}
	seedSubDB(b)
	rows := []model.Setting{
		{Key: "subHostSortMode", Value: "hosts-first"},
		{Key: "datepicker", Value: "gregorian"},
		{Key: "subShowIdentityOnAllLinks", Value: "true"},
		{Key: "subInfoNodeEnable", Value: "false"},
		{Key: "subExpiredTemplate", Value: "Expired"},
		{Key: "subTrafficDepletedTemplate", Value: "Depleted"},
		{Key: "subCalendarExpireInclusive", Value: "true"},
		{Key: "timeLocation", Value: "UTC"},
	}
	for i := range 182 {
		rows = append(rows, model.Setting{Key: fmt.Sprintf("fixture-%03d", i), Value: "fixture-value"})
	}
	seedRenderSettings(b, rows...)
	for i := range 12 {
		seedSubInbound(b, "bench-sub", fmt.Sprintf("fixture-%02d", i), 4900+i, i, wsTLSStream)
	}
	gin.SetMode(gin.TestMode)
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
	base := NewSubService("")
	reads := countRenderSettingQueries(b)

	for _, route := range []string{"prepare", "sub", "json", "clash"} {
		b.Run(route, func(b *testing.B) {
			request := func() {
				if route == "prepare" {
					_ = base.ForRequest("sub.example.com")
					return
				}
				res := httptest.NewRecorder()
				router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "http://sub.example.com/"+route+"/bench-sub", nil))
				if res.Code != http.StatusOK {
					b.Errorf("%s: HTTP %d: %s", route, res.Code, res.Body.String())
				}
			}
			for range 8 {
				request()
			}
			reads.Store(0)
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					request()
				}
			})
			b.StopTimer()
			b.ReportMetric(float64(reads.Load())/float64(b.N), "render-sql/op")
		})
	}
}
