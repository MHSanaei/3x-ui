package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBulkAdjustAppendsExternalLinksHTTP(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	clients := []model.ClientRecord{{Email: "alice"}, {Email: "bob"}, {Email: "unselected"}}
	if err := database.GetDB().Create(&clients).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewClientController(router.Group("/panel/api/clients"))
	body := `{"emails":["alice","bob"],"externalLinks":[{"kind":"link","value":"vless://uuid@example.com:443"},{"kind":"subscription","value":"https://provider.example/sub"}]}`
	for attempt := range 2 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/panel/api/clients/bulkAdjust", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		var response struct {
			Success bool `json:"success"`
			Obj     struct {
				Adjusted      int `json:"adjusted"`
				ExternalLinks struct {
					Added   int `json:"added"`
					Skipped int `json:"skipped"`
					Errors  int `json:"errors"`
				} `json:"externalLinks"`
			} `json:"obj"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		wantAdded, wantSkipped, wantAdjusted := 4, 0, 2
		if attempt == 1 {
			wantAdded, wantSkipped, wantAdjusted = 0, 4, 0
		}
		if w.Code != http.StatusOK || !response.Success || response.Obj.Adjusted != wantAdjusted || response.Obj.ExternalLinks.Added != wantAdded || response.Obj.ExternalLinks.Skipped != wantSkipped || response.Obj.ExternalLinks.Errors != 0 {
			t.Fatalf("attempt %d: response = %s, want adjusted=%d added=%d skipped=%d errors=0", attempt, w.Body.String(), wantAdjusted, wantAdded, wantSkipped)
		}
	}
	var rows []model.ClientExternalLink
	if err := database.GetDB().Order("client_id, sort_index").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0].ClientId != clients[0].Id || rows[1].ClientId != clients[0].Id || rows[2].ClientId != clients[1].Id || rows[3].ClientId != clients[1].Id {
		t.Fatalf("persisted links = %+v, want two rows per selected client only", rows)
	}
}

func TestBulkAdjustRejectsMixedAppendBeforeAnyChanges(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	client := model.ClientRecord{Email: "alice", LimitHwid: 2}
	if err := database.GetDB().Create(&client).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewClientController(router.Group("/panel/api/clients"))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/panel/api/clients/bulkAdjust", strings.NewReader(`{"emails":["alice"],"limitHwid":9,"externalLinks":[{"kind":"link","value":"trojan://pw@example.com:443"}]}`))
	r.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, r)
	var response struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Success || !strings.Contains(response.Msg, "externalLinks cannot be combined with other adjustments") {
		t.Fatalf("mixed request response = %s", w.Body.String())
	}
	var after model.ClientRecord
	if err := database.GetDB().First(&after, client.Id).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := database.GetDB().Model(&model.ClientExternalLink{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if after.LimitHwid != 2 || count != 0 {
		t.Fatalf("rejected request changed limitHwid/links = %d/%d, want 2/0", after.LimitHwid, count)
	}
}
