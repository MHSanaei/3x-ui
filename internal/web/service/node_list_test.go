package service

import (
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

func countReadStatements(t *testing.T) *atomic.Int32 {
	t.Helper()
	var reads atomic.Int32
	count := func(*gorm.DB) { reads.Add(1) }
	db := database.GetDB()
	const hook = "test:count_read_statements"
	if err := db.Callback().Query().After("gorm:query").Register(hook, count); err != nil {
		t.Fatalf("register query hook: %v", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register(hook, count); err != nil {
		t.Fatalf("register row hook: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(hook)
		_ = db.Callback().Row().Remove(hook)
	})
	return &reads
}

// The heartbeat and traffic-sync jobs load the nodes every 5s. Through GetAll each
// load also joined every node-hosted client to its traffic row, only to discard it.
func TestNodeListReadsOnlyTheNodeRows(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	if err := db.Create(&model.Node{Id: 1, Name: "n", Address: "10.0.0.1", Port: 2053, Guid: "g"}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	nid := 1
	inbound := &model.Inbound{Tag: "n1-in", Enable: true, Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nid}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Id: 1, Email: "a@x", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: 1, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("attach client: %v", err)
	}

	reads := countReadStatements(t)
	nodes, err := (&NodeService{}).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "n" {
		t.Fatalf("List = %+v, want the one seeded node", nodes)
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("List issued %d read statements, want 1", got)
	}
}
