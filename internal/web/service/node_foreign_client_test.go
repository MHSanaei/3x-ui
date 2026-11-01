package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
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

func seedMasterInbound(t *testing.T, db *gorm.DB, tag string, port int, clientsJSON string) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Port: port, Protocol: model.VLESS,
		Settings: `{"clients":[` + clientsJSON + `]}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound %q: %v", tag, err)
	}
	clients, err := (&InboundService{}).GetClients(ib)
	if err != nil {
		t.Fatalf("parse clients of %q: %v", tag, err)
	}
	if err := (&ClientService{}).SyncInbound(db, ib.Id, clients); err != nil {
		t.Fatalf("attach clients of %q: %v", tag, err)
	}
	return ib
}

func readClientRecord(t *testing.T, db *gorm.DB, email string) model.ClientRecord {
	t.Helper()
	var row model.ClientRecord
	if err := db.Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("read client %q: %v", email, err)
	}
	return row
}

func countClientsWhere(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.ClientRecord{}).Where(query, args...).Count(&n).Error; err != nil {
		t.Fatalf("count clients: %v", err)
	}
	return n
}

func storedInboundSettings(t *testing.T, db *gorm.DB, tag string) string {
	t.Helper()
	var ib model.Inbound
	if err := db.Where("tag = ?", tag).First(&ib).Error; err != nil {
		t.Fatalf("read inbound %q: %v", tag, err)
	}
	return ib.Settings
}

func nodeHostsClient(t *testing.T, db *gorm.DB, tag, email string) bool {
	t.Helper()
	attached, err := (&ClientService{}).ListForInbound(db, nodeInboundID(t, db, tag))
	if err != nil {
		t.Fatalf("list clients of %q: %v", tag, err)
	}
	for _, c := range attached {
		if c.Email == email {
			return true
		}
	}
	return false
}

// A client shared with a master inbound is rendered into the master's own Xray, so the
// node's copy must not move its credential, reverse tag or subscription id.
func TestNodeCannotRewriteClientSharedWithMaster(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	const legitUUID = "11111111-1111-4111-8111-111111111111"
	shared := fmt.Sprintf(`{"email":"shared@x","id":%q,"subId":"shared-sub","enable":true}`, legitUUID)
	seedMasterInbound(t, db, "master-in", 40001, shared)
	nid := 1
	nodeIb := &model.Inbound{
		UserId: 1, Tag: "n1-in", Enable: true, Port: 41001, Protocol: model.VLESS, NodeID: &nid,
		Settings: `{"clients":[` + shared + `]}`,
	}
	if err := db.Create(nodeIb).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}
	clients, _ := svc.GetClients(nodeIb)
	if err := (&ClientService{}).SyncInbound(db, nodeIb.Id, clients); err != nil {
		t.Fatalf("attach node clients: %v", err)
	}

	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"shared@x","id":"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz","subId":"picked-by-node","reverse":{"tag":"direct"},"enable":true}]}`,
		xray.ClientTraffic{Email: "shared@x", Enable: true})

	row := readClientRecord(t, db, "shared@x")
	if row.UUID != legitUUID || row.SubID != "shared-sub" || row.Reverse != "" {
		t.Fatalf("node rewrote a shared client: uuid=%q subId=%q reverse=%q", row.UUID, row.SubID, row.Reverse)
	}
	stored := storedInboundSettings(t, db, "n1-in")
	if strings.Contains(stored, "zzzz") || strings.Contains(stored, "picked-by-node") || strings.Contains(stored, "direct") {
		t.Fatalf("node inbound stored the node's values for a shared client: %s", stored)
	}
	if !nodeHostsClient(t, db, "n1-in", "shared@x") {
		t.Fatal("the shared client lost its node link")
	}
}

// A refused claim must not be stored (auto-renew re-syncs the settings later) nor move the
// client's lifecycle; usage under an email the master also serves still adds up across panels.
func TestNodeRefusedClaimIsNotStoredOrCounted(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	master := seedMasterInbound(t, db, "master-in", 40001, `{"email":"victim@x","id":"11111111-1111-4111-8111-111111111111","subId":"victim-sub","enable":true}`)
	if err := db.Create(&xray.ClientTraffic{InboundId: master.Id, Email: "victim@x", Enable: true}).Error; err != nil {
		t.Fatalf("seed victim traffic: %v", err)
	}
	createNodeInbound(t, db, 1, "n1-in", 41001)

	hostile := `{"clients":[{"email":"victim@x","id":"","subId":"picked-by-node","enable":true},{"email":"victim@x ","subId":"padded","enable":true},{"email":"VICTIM@X","subId":"cased","enable":true},{"email":"own@x","enable":true}]}`
	for _, up := range []int64{100, 600} {
		syncNodeWithSettings(t, svc, 1, "n1-in", hostile,
			xray.ClientTraffic{Email: "victim@x", Up: up, Enable: false, ExpiryTime: 1},
			xray.ClientTraffic{Email: "victim@x ", Up: up, Enable: true},
			xray.ClientTraffic{Email: "own@x", Enable: true})
	}

	if stored := storedInboundSettings(t, db, "n1-in"); strings.Contains(strings.ToLower(stored), "victim") {
		t.Fatalf("refused claim stored on the node inbound: %s", stored)
	}
	if row := readClientRecord(t, db, "victim@x"); row.SubID != "victim-sub" {
		t.Fatalf("victim subId = %q, want victim-sub", row.SubID)
	}
	if n := countClientsWhere(t, db, "LOWER(email) LIKE ?", "victim%"); n != 1 {
		t.Fatalf("a padded or case-variant claim created a look-alike record: %d victim rows", n)
	}
	if got := readTraffic(t, db, "victim@x"); got.Up != 500 || !got.Enable || got.ExpiryTime != 0 {
		t.Fatalf("victim traffic up=%d enable=%v expiry=%d, want the node's usage (500) and the master's lifecycle (true, 0)", got.Up, got.Enable, got.ExpiryTime)
	}
	var lookalike int64
	db.Model(xray.ClientTraffic{}).Where("email <> ? AND LOWER(email) LIKE ?", "victim@x", "victim%").Count(&lookalike)
	if lookalike != 0 {
		t.Fatalf("a padded claim's stats created %d look-alike traffic rows", lookalike)
	}
	if !nodeHostsClient(t, db, "n1-in", "own@x") {
		t.Fatal("the node's own client was not adopted")
	}
}

// A record with no inbound is the admin's (detached, or its inbound deleted), not a
// node's; only a node whose own merge orphaned it may take it back.
func TestNodeCannotAdoptDetachedClient(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	seedNodeRow(t, db, &model.Node{Id: 2, Name: "n2", Address: "127.0.0.2", Port: 2096, ApiToken: "tok2", Enable: true})
	master := seedMasterInbound(t, db, "master-in", 40001,
		`{"email":"detached@x","id":"11111111-1111-4111-8111-111111111111","subId":"d-sub","enable":true},`+
			`{"email":"mine@x","id":"22222222-2222-4222-8222-222222222222","enable":true},`+
			`{"email":"theirs@x","id":"33333333-3333-4333-8333-333333333333","enable":true},`+
			`{"email":"legacy@x","id":"44444444-4444-4444-8444-444444444444","enable":true}`)
	if err := db.Exec("DELETE FROM client_inbounds WHERE inbound_id = ?", master.Id).Error; err != nil {
		t.Fatalf("detach: %v", err)
	}
	// legacy@x was marked before sync_orphaned_by existed, so its mark names no node.
	for email, node := range map[string]int{"mine@x": 1, "theirs@x": 2, "legacy@x": 0} {
		if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).
			Updates(map[string]any{"sync_orphaned_at": 1, "sync_orphaned_by": node}).Error; err != nil {
			t.Fatalf("mark %s orphaned: %v", email, err)
		}
	}
	createNodeInbound(t, db, 1, "n1-in", 41001)

	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"detached@x","subId":"picked","enable":true},{"email":"mine@x","enable":true},{"email":"theirs@x","subId":"picked2","enable":true},{"email":"legacy@x","enable":true}]}`)

	if row := readClientRecord(t, db, "detached@x"); row.SubID != "d-sub" || nodeHostsClient(t, db, "n1-in", "detached@x") {
		t.Fatalf("node adopted a detached client: subId=%q", row.SubID)
	}
	if nodeHostsClient(t, db, "n1-in", "theirs@x") {
		t.Fatal("node adopted a client another node's merge orphaned")
	}
	if !nodeHostsClient(t, db, "n1-in", "mine@x") {
		t.Fatal("node could not take back a client its own merge orphaned")
	}
	if !nodeHostsClient(t, db, "n1-in", "legacy@x") {
		t.Fatal("a client orphaned before the upgrade could not be taken back and would be reaped")
	}
}

// The subscription server selects by subId alone, so a node's client must not join
// the subscription of a customer it does not host.
func TestNodeCannotClaimForeignSubID(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	seedMasterInbound(t, db, "master-in", 40001, `{"email":"customer@x","id":"11111111-1111-4111-8111-111111111111","subId":"cust-sub","enable":true}`)
	createNodeInbound(t, db, 1, "n1-in", 41001)

	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"node-own@x","id":"66666666-6666-4666-8666-666666666666","subId":"cust-sub","enable":true}]}`)

	if n := countClientsWhere(t, db, "sub_id = ?", "cust-sub"); n != 1 {
		t.Fatalf("node joined a customer's subscription: %d records hold cust-sub", n)
	}
}

// A shared client's tunnel keys are rendered into the master's own WireGuard inbound, so a
// node tunnel must not own them even when its inbound id is the lower one.
func TestNodeTunnelDoesNotOwnSharedPeerKeys(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	const masterKey = "mJ3F8Yb3Kx6cdx6Sx4WbJ3qQDXB5cN3d0pH6m4kq0Ug="
	const nodeKey = "nK9Pq1Zr6s8Lw2Vb4Xc7Dm0Fh5Jt3Ny9Ru1Ea6Go2Is="
	entry := func(key, ip string) string {
		return fmt.Sprintf(`{"email":"shared@x","publicKey":%q,"allowedIPs":[%q],"enable":true}`, key, ip)
	}
	nid := 1
	nodeIb := &model.Inbound{
		UserId: 1, Tag: "n1-wg", Enable: true, Port: 41002, Protocol: model.WireGuard, NodeID: &nid,
		Settings: `{"clients":[` + entry(masterKey, "10.1.0.2/32") + `]}`,
	}
	masterIb := &model.Inbound{
		UserId: 1, Tag: "master-wg", Enable: true, Port: 40002, Protocol: model.WireGuard,
		Settings: `{"clients":[` + entry(masterKey, "10.0.0.2/32") + `]}`,
	}
	for _, ib := range []*model.Inbound{nodeIb, masterIb} {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create %q: %v", ib.Tag, err)
		}
		clients, err := svc.GetClients(ib)
		if err != nil {
			t.Fatalf("parse %q: %v", ib.Tag, err)
		}
		if err := (&ClientService{}).SyncInbound(db, ib.Id, clients); err != nil {
			t.Fatalf("attach %q: %v", ib.Tag, err)
		}
	}

	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{{
		Tag: "n1-wg", Enable: true, Port: 41002, Protocol: model.WireGuard,
		Settings:    `{"clients":[` + entry(nodeKey, "10.1.0.2/32") + `]}`,
		ClientStats: []xray.ClientTraffic{{Email: "shared@x", Enable: true}},
	}}}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !nodeHostsClient(t, db, "n1-wg", "shared@x") {
		t.Fatal("the shared client lost its node tunnel link")
	}
	if got := readClientRecord(t, db, "shared@x").PublicKey; got != masterKey {
		t.Fatalf("node tunnel replaced a shared client's public key: %q", got)
	}
}

// An honest node's copy of a shared client differs only in timestamps and comment; rewriting
// it to the record's JSON would churn the stored settings on every tick.
func TestNodeSharedClientInAgreementIsNotRewritten(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})

	shared := `{"email":"shared@x","id":"11111111-1111-4111-8111-111111111111","subId":"shared-sub","comment":"vip","enable":true}`
	seedMasterInbound(t, db, "master-in", 40001, shared)
	nid := 1
	nodeIb := &model.Inbound{
		UserId: 1, Tag: "n1-in", Enable: true, Port: 41001, Protocol: model.VLESS, NodeID: &nid,
		Settings: `{"clients":[` + shared + `]}`,
	}
	if err := db.Create(nodeIb).Error; err != nil {
		t.Fatalf("create node inbound: %v", err)
	}
	clients, _ := svc.GetClients(nodeIb)
	if err := (&ClientService{}).SyncInbound(db, nodeIb.Id, clients); err != nil {
		t.Fatalf("attach node clients: %v", err)
	}

	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"shared@x","id":"11111111-1111-4111-8111-111111111111","subId":"shared-sub","enable":true,"created_at":1,"updated_at":2}]}`,
		xray.ClientTraffic{Email: "shared@x", Enable: true})

	if stored := storedInboundSettings(t, db, "n1-in"); !strings.Contains(stored, `"updated_at":2`) {
		t.Fatalf("a node in agreement with the master had its entry rewritten: %s", stored)
	}
}

// The refused emails are the node's choice, so the state that keeps their warning to one
// line must not keep every name a node ever sent.
func TestNodeRefusedClaimLogStateIsBounded(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInbound(t, db, 1, "n1-in", 41001)

	for i := range 50 {
		syncNodeWithSettings(t, svc, 1, "n1-in", fmt.Sprintf(`{"clients":[{"email":"rotating-%d@x ","enable":true}]}`, i))
	}
	held, _ := reportedRefusedClaims.Load(1)
	if got := len(held.(map[string]struct{})); got != 1 {
		t.Fatalf("refused-claim log state holds %d emails for one node, want 1", got)
	}
}

// A subscription renders every client holding its subId, so a node creating many clients
// under one subId multiplied the cost of a single request without bound.
func TestNodeClientsCannotShareASubID(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInbound(t, db, 1, "n1-in", 41001)

	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"a@x","subId":"one-sub","enable":true},{"email":"b@x","subId":"one-sub","enable":true},{"email":"c@x","subId":"c-sub","enable":true}]}`)
	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"a@x","subId":"one-sub","enable":true},{"email":"c@x","subId":"c-sub","enable":true},{"email":"d@x","subId":"one-sub","enable":true}]}`)

	if n := countClientsWhere(t, db, "sub_id = ?", "one-sub"); n != 1 {
		t.Fatalf("%d records hold one subId, want 1", n)
	}
	if !nodeHostsClient(t, db, "n1-in", "c@x") {
		t.Fatal("a client with its own subId was not adopted")
	}
}

// Every reported stream, sniffing and remark is rendered once per client and external
// proxy of a subscription, so an oversized one let a node make one request cost gigabytes.
func TestNodeOversizeInboundIsNotTakenIn(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	createNodeInbound(t, db, 1, "n1-in", 41001)

	proxies := make([]string, maxNodeExternalProxies+1)
	for i := range proxies {
		proxies[i] = fmt.Sprintf(`{"dest":"h%d.example","port":443,"forceTls":"same"}`, i)
	}
	stream := `{"network":"tcp","externalProxy":[` + strings.Join(proxies, ",") + `]}`
	snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{
		{Tag: "n1-in", Enable: true, Port: 41001, Protocol: model.VLESS, Settings: `{"clients":[]}`, StreamSettings: stream},
		{Tag: "n1-new", Enable: true, Port: 41002, Protocol: model.VLESS, Settings: `{"clients":[]}`, Remark: strings.Repeat("r", maxNodeRemarkBytes+1)},
	}}
	if _, err := svc.setRemoteTrafficLocked(1, snap, false, false); err != nil {
		t.Fatalf("sync: %v", err)
	}

	var stored model.Inbound
	if err := db.Where("tag = ?", "n1-in").First(&stored).Error; err != nil {
		t.Fatalf("the oversized report removed the existing inbound: %v", err)
	}
	if strings.Contains(stored.StreamSettings, "externalProxy") {
		t.Fatal("an inbound with too many external proxies was taken in")
	}
	var created int64
	db.Model(&model.Inbound{}).Where("tag = ?", "n1-new").Count(&created)
	if created != 0 {
		t.Fatal("an inbound with an oversized remark was adopted")
	}
}

// SQLite's LOWER folds ASCII only, so a lookup by lowered email missed a stored non-ASCII
// capital and the node's claim on that master-only client went through untouched.
func TestNodeCannotClaimNonASCIICapitalEmail(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}
	seedNodeRow(t, db, &model.Node{Id: 1, Name: "n1", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true})
	seedMasterInbound(t, db, "master-in", 40001, `{"email":"Иван@x","id":"11111111-1111-4111-8111-111111111111","subId":"ivan-sub","enable":true}`)
	createNodeInbound(t, db, 1, "n1-in", 41001)

	syncNodeWithSettings(t, svc, 1, "n1-in",
		`{"clients":[{"email":"Иван@x","subId":"picked-by-node","enable":true},{"email":"иван@x","subId":"lower","enable":true}]}`)

	if row := readClientRecord(t, db, "Иван@x"); row.SubID != "ivan-sub" {
		t.Fatalf("node rewrote a master-only non-ASCII client: subId=%q", row.SubID)
	}
	if n := countClientsWhere(t, db, "email = ?", "иван@x"); n != 0 {
		t.Fatal("a case variant of a non-ASCII email created a look-alike record")
	}
}
