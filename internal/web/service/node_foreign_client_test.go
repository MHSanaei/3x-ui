package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func readClientUUID(t *testing.T, db *gorm.DB, email string) string {
	t.Helper()
	var row model.ClientRecord
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("read client %q: %v", email, err)
	}
	return row.UUID
}

// Emails are globally unique, so a node reporting one that belongs to a master
// inbound would otherwise overwrite its credentials and lock the real user out.
func TestNodeCannotClaimClientOfAnotherInbound(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	clientSvc := &ClientService{}

	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	const (
		victim     = "victim@x"
		nodeLocal  = "nodelocal@x"
		legitUUID  = "11111111-1111-1111-1111-111111111111"
		attackUUID = "99999999-9999-9999-9999-999999999999"
	)

	master := &model.Inbound{
		UserId: 1, Tag: "master-in", Enable: true, Port: 40001, Protocol: model.VLESS,
		Settings: fmt.Sprintf(`{"clients":[{"email":%q,"id":%q,"enable":true}]}`, victim, legitUUID),
	}
	if err := db.Create(master).Error; err != nil {
		t.Fatalf("create master inbound: %v", err)
	}
	masterClients, err := svc.GetClients(master)
	if err != nil {
		t.Fatalf("parse master clients: %v", err)
	}
	if err := clientSvc.SyncInbound(db, master.Id, masterClients); err != nil {
		t.Fatalf("attach master client: %v", err)
	}
	if got := readClientUUID(t, db, victim); got != legitUUID {
		t.Fatalf("setup: master client uuid = %q, want %q", got, legitUUID)
	}

	createNodeInbound(t, db, 1, "n1-in", 41001)
	hostile := fmt.Sprintf(`{"clients":[{"email":%q,"id":%q,"enable":true},{"email":%q,"id":%q,"enable":true}]}`,
		victim, attackUUID, nodeLocal, attackUUID)
	syncNodeWithSettings(t, svc, 1, "n1-in", hostile,
		xray.ClientTraffic{Email: victim, Enable: true},
		xray.ClientTraffic{Email: nodeLocal, Enable: true})

	if got := readClientUUID(t, db, victim); got != legitUUID {
		t.Fatalf("node overwrote a master client's uuid: got %q, want %q", got, legitUUID)
	}
	nodeAttached, err := clientSvc.ListForInbound(db, nodeInboundID(t, db, "n1-in"))
	if err != nil {
		t.Fatalf("list node clients: %v", err)
	}
	for _, c := range nodeAttached {
		if c.Email == victim {
			t.Fatal("node inbound adopted a client that belongs to a master inbound")
		}
	}

	// The node's own client must still be adopted, or the guard has replaced one
	// bug with a worse one.
	if got := readClientUUID(t, db, nodeLocal); got != attackUUID {
		t.Fatalf("node-owned client not adopted: uuid = %q, want %q", got, attackUUID)
	}
}

func nodeInboundID(t *testing.T, db *gorm.DB, tag string) int {
	t.Helper()
	var ib model.Inbound
	if err := db.Where("tag = ?", tag).First(&ib).Error; err != nil {
		t.Fatalf("read inbound %q: %v", tag, err)
	}
	return ib.Id
}
