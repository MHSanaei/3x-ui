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
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestTrafficSyncEnumerationSkipsDashboardQueries(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(nil) })
	db := database.GetDB()
	node := &model.Node{Name: "query-scope", Address: "127.0.0.1", Port: 2096}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(node).UpdateColumn("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{NodeID: &node.Id, Tag: "query-scope-in", Port: 44521, Protocol: model.VLESS, Settings: "{}"}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	client := &model.ClientRecord{Email: "query-scope@x"}
	if err := db.Create(client).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatal(err)
	}

	joins := 0
	const callback = "test:dashboard-query-scope"
	if err := db.Callback().Row().After("gorm:row").Register(callback, func(tx *gorm.DB) {
		sql := strings.ToLower(tx.Statement.SQL.String())
		if strings.Contains(sql, "join client_inbounds") && strings.Contains(sql, "node_id") {
			joins++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Row().Remove(callback) })
	NewNodeTrafficSyncJob().Run()
	if joins != 0 {
		t.Fatalf("sync loaded dashboard counts/status %d time(s)", joins)
	}

	nodes, err := (&service.NodeService{}).GetAll()
	if err != nil || len(nodes) != 1 {
		t.Fatalf("dashboard node list: %v, err=%v", nodes, err)
	}
	if nodes[0].ClientCount != 1 || joins != 2 {
		t.Fatalf("dashboard returned %d clients and used %d count/status queries, want 1 and 2", nodes[0].ClientCount, joins)
	}
}
