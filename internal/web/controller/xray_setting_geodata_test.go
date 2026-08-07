package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"
)

// TestGetGeodataCategoriesEndpoint exercises the real HTTP route (gin
// routing, controller wiring, JSON envelope) rather than just the
// underlying service function -- a genuine end-to-end check of
// GET /panel/api/xray/getGeodataCategories given no DB is available in
// this environment (go-sqlite3 needs cgo) to run the full panel binary.
func TestGetGeodataCategoriesEndpoint(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)

	writeFixture(t, filepath.Join(dir, "geosite.dat"), &geodata.GeoSiteList{
		Entry: []*geodata.GeoSite{{Code: "YOUTUBE"}},
	})
	writeFixture(t, filepath.Join(dir, "geosite_roscom.dat"), &geodata.GeoSiteList{
		Entry: []*geodata.GeoSite{{Code: "SOME-CODE"}},
	})
	writeFixture(t, filepath.Join(dir, "geoip.dat"), &geodata.GeoIPList{
		Entry: []*geodata.GeoIP{{Code: "PRIVATE"}},
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewXraySettingController(router.Group("/panel/api"))

	req := httptest.NewRequest(http.MethodGet, "/panel/api/xray/getGeodataCategories", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}

	body := resp.Body.String()
	for _, want := range []string{`"success":true`, `"geosite:youtube"`, `"ext:geosite_roscom.dat:some-code"`, `"geoip:private"`} {
		if !strings.Contains(body, want) {
			t.Errorf("response body %s does not contain %s", body, want)
		}
	}
}

func writeFixture(t *testing.T, path string, msg proto.Message) {
	t.Helper()
	data, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal fixture %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
