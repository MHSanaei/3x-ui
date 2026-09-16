package sub

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// base64("aes-256-gcm:clientpw"), the SIP002 userinfo both spellings share. The
// panel emits this node as `plugin=obfs-local;obfs=http`, and Clash has no way
// to represent it, so both the inbound path and the link path drop it.
const clashDroppedExternalLink = "ss://YWVzLTI1Ni1nY206Y2xpZW50cHc@198.51.100.9:8443?type=tcp&headerType=http&host=test#obfs"

const tcpObfsStream = `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"http","request":{"path":["/"],"headers":{"Host":["test"]}}}}}`

func clashSubRouter(t *testing.T) *gin.Engine {
	t.Helper()
	oldDistFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldDistFS })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSUBController(
		router.Group("/"),
		WithSUBJsonEnabled(true),
		WithSUBClashEnabled(true),
		WithSUBEncryption(false),
	)
	return router
}

func fetchClashSub(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "sub.example.com"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func seedClashQuotaSub(t *testing.T, subID string, expiry int64) {
	t.Helper()
	db := database.GetDB()
	seedSubInbound(t, subID, "A", 10001, 1, wsTLSStream)
	if err := db.Create(&xray.ClientTraffic{Email: "A@e", Up: 11, Down: 22, Total: 1024, ExpiryTime: expiry}).Error; err != nil {
		t.Fatalf("seed A traffic: %v", err)
	}
	rec := &model.ClientRecord{Email: "B@e", SubID: subID, UUID: "22222222-2222-4222-8222-222222222222", Enable: true, ExpiryTime: expiry}
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed B client: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "B@e", Up: 100, Down: 200, Total: 2048, ExpiryTime: expiry}).Error; err != nil {
		t.Fatalf("seed B traffic: %v", err)
	}
	seedClientExternalLink(t, rec.Id, model.ClientExternalLink{Kind: model.ExternalLinkKindLink, Value: clashDroppedExternalLink, SortIndex: 1})
}

// B's node cannot be represented in Clash, but B still owns quota: the header
// must keep counting B's traffic, not quietly serve A's numbers alone.
func TestClashQuotaHeaderCountsDroppedExternalLink(t *testing.T) {
	initSubDB(t)
	subID := "clash-quota-drop"
	expiry := time.Now().Add(24 * time.Hour).UnixMilli()
	seedClashQuotaSub(t, subID, expiry)

	w := fetchClashSub(t, clashSubRouter(t), "/clash/"+subID+"?view=raw")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "198.51.100.9") {
		t.Fatalf("unrepresentable node leaked into the profile: %s", w.Body.String())
	}
	wantHeader := fmt.Sprintf("upload=111; download=222; total=3072; expire=%d", expiry/1000)
	if got := w.Header().Get("Subscription-Userinfo"); got != wantHeader {
		t.Fatalf("Subscription-Userinfo = %q, want %q", got, wantHeader)
	}
}

// Nothing to serve is answered the same way whether the unrepresentable node is
// an inbound or an external link — the drop must not depend on where it came from.
func TestClashAllUnrepresentableNodesAnswerAlike(t *testing.T) {
	statuses := make(map[string]int, 2)
	for _, source := range []string{"inbound", "external-link"} {
		t.Run(source, func(t *testing.T) {
			initSubDB(t)
			if source == "inbound" {
				seedSubInbound(t, "clash-parity", "obfs", 10001, 1, tcpObfsStream)
			} else {
				rec := &model.ClientRecord{Email: "B@e", SubID: "clash-parity", UUID: "22222222-2222-4222-8222-222222222222", Enable: true}
				if err := database.GetDB().Create(rec).Error; err != nil {
					t.Fatalf("seed client: %v", err)
				}
				seedClientExternalLink(t, rec.Id, model.ClientExternalLink{Kind: model.ExternalLinkKindLink, Value: clashDroppedExternalLink, SortIndex: 1})
			}
			w := fetchClashSub(t, clashSubRouter(t), "/clash/clash-parity?view=raw")
			if w.Body.Len() != 0 {
				t.Fatalf("body = %q, want empty", w.Body.String())
			}
			statuses[source] = w.Code
		})
	}
	if statuses["inbound"] != statuses["external-link"] {
		t.Fatalf("status differs by node source: %v", statuses)
	}
}
