package service

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func recordExecutedSQL(t *testing.T, db *gorm.DB) *[]string {
	t.Helper()
	var queries []string
	appendSQL := func(tx *gorm.DB) {
		if sql := tx.Statement.SQL.String(); sql != "" {
			queries = append(queries, sql)
		}
	}
	const cbName = "test:record-executed-sql"
	if err := db.Callback().Query().After("gorm:query").Register(cbName, appendSQL); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	if err := db.Callback().Row().After("gorm:row").Register(cbName, appendSQL); err != nil {
		t.Fatalf("register row callback: %v", err)
	}
	if err := db.Callback().Raw().After("gorm:raw").Register(cbName, appendSQL); err != nil {
		t.Fatalf("register raw callback: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(cbName)
		_ = db.Callback().Row().Remove(cbName)
		_ = db.Callback().Raw().Remove(cbName)
	})
	return &queries
}

func TestListOperational_SkipsDashboardQueriesAndPreservesOrderAndTokens(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	db := database.GetDB()
	svc := NodeService{}

	// Insert higher id before lower id so ordering by id asc is verified.
	n2 := &model.Node{Id: 2, Name: "second", Address: "10.0.0.2", Port: 2053, ApiToken: "secret-two", Guid: "guid-2"}
	n1 := &model.Node{Id: 1, Name: "first", Address: "10.0.0.1", Port: 2053, ApiToken: "secret-one", Guid: "guid-1"}
	for _, n := range []*model.Node{n2, n1} {
		if err := svc.Create(n); err != nil {
			t.Fatalf("create node %d: %v", n.Id, err)
		}
	}
	// An undecryptable token must keep its node row and surface as LastError.
	n3 := &model.Node{Id: 3, Name: "broken", Address: "10.0.0.3", Port: 2053, ApiToken: "secret-three", Guid: "guid-3"}
	if err := svc.Create(n3); err != nil {
		t.Fatalf("create broken token node: %v", err)
	}
	stored := rawStoredToken(t, n3.Id)
	body := strings.LastIndexByte(stored, ':') + 1
	replacement := byte('A')
	if stored[body] == replacement {
		replacement = 'B'
	}
	corrupt := stored[:body] + string(replacement) + stored[body+1:]
	if err := db.Model(&model.Node{}).Where("id = ?", n3.Id).Update("api_token", corrupt).Error; err != nil {
		t.Fatalf("corrupt broken token node: %v", err)
	}

	nid := 1
	ib := &model.Inbound{Tag: "n1-in", Enable: true, Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`, NodeID: &nid}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.ClientRecord{Id: 1, Email: "user@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: 1, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("create client_inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: "user@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("create client_traffic: %v", err)
	}

	queries := recordExecutedSQL(t, db)

	nodes, err := svc.ListOperational()
	if err != nil {
		t.Fatalf("ListOperational: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("len(nodes) = %d, want 3", len(nodes))
	}
	if nodes[0].Id != 1 || nodes[1].Id != 2 || nodes[2].Id != 3 {
		t.Fatalf("node order = [%d, %d, %d], want [1, 2, 3]", nodes[0].Id, nodes[1].Id, nodes[2].Id)
	}
	if nodes[0].ApiToken != "secret-one" || nodes[1].ApiToken != "secret-two" {
		t.Errorf("decrypted tokens = [%q, %q], want [secret-one, secret-two]", nodes[0].ApiToken, nodes[1].ApiToken)
	}
	if nodes[2].ApiToken != "" || !strings.Contains(nodes[2].LastError, "token decrypt failed") {
		t.Errorf("broken token node = (token=%q, lastError=%q), want empty token and decrypt error", nodes[2].ApiToken, nodes[2].LastError)
	}
	if nodes[0].InboundCount != 0 || nodes[0].ClientCount != 0 || nodes[0].ActiveCount != 0 {
		t.Errorf("ListOperational should leave dashboard stats at zero, got %+v", nodes[0])
	}
	for _, q := range *queries {
		lower := strings.ToLower(q)
		if strings.Contains(lower, "inbounds") || strings.Contains(lower, "client_inbounds") || strings.Contains(lower, "client_traffics") {
			t.Errorf("ListOperational executed unexpected dashboard SQL: %s", q)
		}
	}

	enriched, err := svc.GetNodeTree()
	if err != nil {
		t.Fatalf("GetNodeTree: %v", err)
	}
	if enriched[0].InboundCount != 1 || enriched[0].ClientCount != 1 || enriched[0].ActiveCount != 1 {
		t.Errorf("GetNodeTree stats = (inbound=%d, client=%d, active=%d), want (1, 1, 1)",
			enriched[0].InboundCount, enriched[0].ClientCount, enriched[0].ActiveCount)
	}
}

func TestGetNodeTree_TransitiveSkipsDuplicateStatusClassification(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	svc := NodeService{}

	if err := db.Create(&model.Node{
		Id: 1, Name: "Node2", Address: "10.0.0.2", Port: 2053,
		ApiToken: "t", Guid: "node2-guid", Status: "online",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	nid := 1
	ib := &model.Inbound{
		Tag: "n1-own", Enable: true, Port: 443, Protocol: model.VLESS,
		Settings: `{"clients":[]}`, NodeID: &nid, OriginNodeGuid: "node2-guid",
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	for i, email := range []string{"a@example.com", "b@example.com"} {
		cid := i + 1
		if err := db.Create(&model.ClientRecord{Id: cid, Email: email, Enable: true}).Error; err != nil {
			t.Fatalf("create client: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: cid, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("create client_inbound: %v", err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: ib.Id, Email: email, Enable: true}).Error; err != nil {
			t.Fatalf("create client_traffic: %v", err)
		}
	}

	nodeDescendantsMu.Lock()
	nodeDescendantsCache[1] = []model.NodeSummary{{
		Guid: "node3-guid", ParentGuid: "node2-guid", Name: "Node3", Address: "10.0.0.3", Status: "online",
	}}
	nodeDescendantsMu.Unlock()
	t.Cleanup(func() {
		nodeDescendantsMu.Lock()
		nodeDescendantsCache = map[int][]model.NodeSummary{}
		nodeDescendantsMu.Unlock()
	})

	queries := recordExecutedSQL(t, db)
	tree, err := svc.GetNodeTree()
	if err != nil {
		t.Fatalf("GetNodeTree: %v", err)
	}
	statusJoinCalls := 0
	for _, q := range *queries {
		if strings.Contains(strings.ToLower(q), "client_traffics") {
			statusJoinCalls++
		}
	}
	if statusJoinCalls != 1 {
		t.Errorf("client_traffics status join count = %d, want 1", statusJoinCalls)
	}
	var node2 *model.Node
	for _, n := range tree {
		if n.Guid == "node2-guid" {
			node2 = n
			break
		}
	}
	if node2 == nil {
		t.Fatal("node2 missing from tree")
		return
	}
	if node2.ClientCount != 2 || node2.ActiveCount != 2 || node2.InboundCount != 1 {
		t.Errorf("node2 counts = (client=%d, active=%d, inbound=%d), want (2, 2, 1)",
			node2.ClientCount, node2.ActiveCount, node2.InboundCount)
	}
}
