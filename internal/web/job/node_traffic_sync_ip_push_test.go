package job

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// A node's IP-limit job only reads rows for its own clients, so pushing the
// whole table made every node store and echo back the entire fleet's IPs.
func TestNodeTrafficSyncPushesOnlyHostedClientIps(t *testing.T) {
	xuilogger.InitLogger(logging.ERROR)
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	service.StartTrafficWriter()
	t.Cleanup(service.StopTrafficWriter)
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	var mu sync.Mutex
	pushed := map[string][]string{}
	now := time.Now().Unix()
	for i, email := range []string{"a@node", "b@node"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case strings.HasSuffix(r.URL.Path, "inbounds/list"):
				settings := fmt.Sprintf(`{"clients":[{"email":%q,"id":"0000000%d-0000-4000-8000-000000000000","enable":true}],"decryption":"none"}`, email, i)
				ib, _ := json.Marshal([]map[string]any{{
					"id": 1, "tag": fmt.Sprintf("in-%d", 20000+i), "port": 20000 + i, "protocol": "vless", "enable": true,
					"settings": settings, "streamSettings": `{"network":"tcp"}`, "sniffing": `{}`,
					"clientStats": []map[string]any{{"email": email, "enable": true}},
				}})
				_, _ = w.Write([]byte(`{"success":true,"obj":` + string(ib) + `}`))
				return
			case strings.HasSuffix(r.URL.Path, "server/clientIps") && r.Method == http.MethodPost:
				var rows []model.InboundClientIps
				_ = json.NewDecoder(r.Body).Decode(&rows)
				mu.Lock()
				for _, row := range rows {
					pushed[email] = append(pushed[email], row.ClientEmail)
				}
				mu.Unlock()
			}
			_, _ = w.Write([]byte(`{"success":true}`))
		}))
		t.Cleanup(srv.Close)
		host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
		portNum, _ := strconv.Atoi(port)
		if err := database.GetDB().Create(&model.Node{
			Name: email, Scheme: "http", Address: host, Port: portNum, BasePath: "/", ApiToken: "tok",
			Enable: true, Status: "online", AllowPrivateAddress: true, TlsVerifyMode: "verify",
		}).Error; err != nil {
			t.Fatalf("create node: %v", err)
		}
		if err := database.GetDB().Create(&model.InboundClientIps{
			ClientEmail: email, Ips: fmt.Sprintf(`[{"ip":"10.0.0.%d","timestamp":%d}]`, i+1, now),
		}).Error; err != nil {
			t.Fatalf("seed client ips: %v", err)
		}
	}

	NewNodeTrafficSyncJob().Run()

	for _, email := range []string{"a@node", "b@node"} {
		if got := pushed[email]; !slices.Equal(got, []string{email}) {
			t.Errorf("node hosting %s received IP rows for %v, want only [%s]", email, got, email)
		}
	}
}
