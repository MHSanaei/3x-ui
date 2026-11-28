package job

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/op/go-logging"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestBackgroundNodeJobs_SkipNodeDashboardStatQueries(t *testing.T) {
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })

	db := database.GetDB()
	disabled := &model.Node{Name: "disabled", Address: "127.0.0.1", Port: 2053, ApiToken: "tok", Enable: true, Status: "offline"}
	if err := db.Create(disabled).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := db.Model(disabled).Update("enable", false).Error; err != nil {
		t.Fatalf("disable node: %v", err)
	}
	nid := disabled.Id
	ib := &model.Inbound{Tag: "n-in", Enable: true, Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nid}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Id: 1, Email: "c@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: 1, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("create client_inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "c@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("create client_traffic: %v", err)
	}

	var queries []string
	recordSQL := func(tx *gorm.DB) {
		if sql := tx.Statement.SQL.String(); sql != "" {
			queries = append(queries, sql)
		}
	}
	const cbName = "test:job-node-enum-sql"
	if err := db.Callback().Query().After("gorm:query").Register(cbName, recordSQL); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register(cbName, recordSQL); err != nil {
		t.Fatalf("register row callback: %v", err)
	}
	if err := db.Callback().Raw().After("gorm:raw").Register(cbName, recordSQL); err != nil {
		t.Fatalf("register raw callback: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
		_ = db.Callback().Row().Remove(cbName)
		_ = db.Callback().Raw().Remove(cbName)
	})

	NewNodeTrafficSyncJob().Run()
	NewNodeHeartbeatJob().Run()

	for _, q := range queries {
		lower := strings.ToLower(q)
		if strings.Contains(lower, "inbounds.node_id is not null") ||
			strings.Contains(lower, "count(distinct client_inbounds.client_id)") ||
			strings.Contains(lower, "join client_traffics ct") {
			t.Errorf("background node job executed dashboard node-stats query: %s", q)
		}
	}
}
