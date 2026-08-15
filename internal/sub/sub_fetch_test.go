package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestRecordSubscriptionFetch(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()

	clients := []model.ClientRecord{
		{Email: "alpha@example.com", SubID: "sub-alpha", Enable: true},
		{Email: "bravo@example.com", SubID: "sub-bravo", Enable: true},
	}
	for i := range clients {
		if err := db.Create(&clients[i]).Error; err != nil {
			t.Fatalf("create client %s: %v", clients[i].Email, err)
		}
		if err := db.Create(&xray.ClientTraffic{Email: clients[i].Email}).Error; err != nil {
			t.Fatalf("create traffic %s: %v", clients[i].Email, err)
		}
	}

	before := time.Now().UnixMilli()
	if err := (&SubService{}).RecordSubscriptionFetch("sub-alpha"); err != nil {
		t.Fatalf("RecordSubscriptionFetch: %v", err)
	}

	var alpha, bravo xray.ClientTraffic
	if err := db.Where("email = ?", "alpha@example.com").First(&alpha).Error; err != nil {
		t.Fatalf("load alpha traffic: %v", err)
	}
	if err := db.Where("email = ?", "bravo@example.com").First(&bravo).Error; err != nil {
		t.Fatalf("load bravo traffic: %v", err)
	}
	if alpha.LastSubFetch < before {
		t.Fatalf("alpha lastSubFetch = %d, want >= %d", alpha.LastSubFetch, before)
	}
	if bravo.LastSubFetch != 0 {
		t.Fatalf("bravo lastSubFetch = %d, want 0", bravo.LastSubFetch)
	}

	if err := (&SubService{}).RecordSubscriptionFetch("unknown"); err != nil {
		t.Fatalf("unknown subId: %v", err)
	}
	if err := (&SubService{}).RecordSubscriptionFetch(""); err != nil {
		t.Fatalf("empty subId: %v", err)
	}
}

func TestRecordSubscriptionFetchStatusGate(t *testing.T) {
	initSubDB(t)
	db := database.GetDB()
	client := &model.ClientRecord{Email: "alpha@example.com", SubID: "sub-alpha", Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: client.Email}).Error; err != nil {
		t.Fatalf("create traffic: %v", err)
	}

	controller := &SUBController{subService: &SubService{}}
	notFoundRecorder := httptest.NewRecorder()
	notFound, _ := gin.CreateTestContext(notFoundRecorder)
	notFound.Request = httptest.NewRequest(http.MethodGet, "/sub/sub-alpha", nil)
	notFound.Params = gin.Params{{Key: "subid", Value: "sub-alpha"}}
	notFound.Status(http.StatusNotFound)
	controller.recordSubscriptionFetch(notFound)

	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", client.Email).First(&traffic).Error; err != nil {
		t.Fatalf("load traffic after 404: %v", err)
	}
	if traffic.LastSubFetch != 0 {
		t.Fatalf("404 updated lastSubFetch to %d", traffic.LastSubFetch)
	}

	okRecorder := httptest.NewRecorder()
	ok, _ := gin.CreateTestContext(okRecorder)
	ok.Request = httptest.NewRequest(http.MethodGet, "/sub/sub-alpha", nil)
	ok.Params = gin.Params{{Key: "subid", Value: "sub-alpha"}}
	ok.Status(http.StatusOK)
	controller.recordSubscriptionFetch(ok)
	if err := db.Where("email = ?", client.Email).First(&traffic).Error; err != nil {
		t.Fatalf("load traffic after 200: %v", err)
	}
	if traffic.LastSubFetch == 0 {
		t.Fatal("200 did not update lastSubFetch")
	}
}
