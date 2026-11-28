package service

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeClientReconcileRollback(t *testing.T) {
	testNodeClientReconcileRollback(t)
}

func TestNodeClientReconcileRollbackPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	testNodeClientReconcileRollback(t)
}

func testNodeClientReconcileRollback(t *testing.T) {
	t.Helper()
	db := initTrafficTestDB(t)
	node := &model.Node{Name: "rollback-7219", Address: "127.0.0.1", Port: 2096, ApiToken: "test", Enable: true}
	seedNodeRow(t, db, node)
	svc := &InboundService{}

	newClient := func(email string) model.Client {
		return model.Client{ID: uuid.NewString(), Email: email, SubID: email, Password: email, Enable: true}
	}
	shared := newClient("shared@rollback")
	shared.Flow = "xtls-rprx-vision"
	otherTuic := newClient("other-tuic@rollback")
	removed := newClient("removed@rollback")
	attempted := newClient("attempted@rollback")
	healthy := newClient("healthy@rollback")
	newHealthy := newClient("new-healthy@rollback")

	nodeInbound := func(tag string, port int, protocol model.Protocol, clients ...model.Client) *model.Inbound {
		return &model.Inbound{Tag: tag, Port: port, Protocol: protocol, Enable: true, Settings: clientsSettings(t, clients)}
	}
	merge := func(target *model.Inbound, up int64, siblings ...model.Client) {
		t.Helper()
		tuic := nodeInbound("rollback-tuic", 45101, model.TUIC, shared, otherTuic)
		sibling := nodeInbound("rollback-healthy", 45103, model.VLESS, siblings...)
		sibling.Up = up
		sibling.ClientStats = []xray.ClientTraffic{{Email: healthy.Email, Up: up, Enable: true}}
		snap := &runtime.TrafficSnapshot{Inbounds: []*model.Inbound{tuic, target, sibling}}
		if _, err := svc.SetRemoteTraffic(node.Id, snap, false, false); err != nil {
			t.Fatalf("merge snapshot: %v", err)
		}
	}
	loadInbound := func(tag string) model.Inbound {
		t.Helper()
		var ib model.Inbound
		if err := db.Where("node_id = ? AND tag = ?", node.Id, tag).Take(&ib).Error; err != nil {
			t.Fatal(err)
		}
		return ib
	}
	loadLinks := func(id int) []model.ClientInbound {
		t.Helper()
		var links []model.ClientInbound
		if err := db.Where("inbound_id = ?", id).Order("client_id").Find(&links).Error; err != nil {
			t.Fatal(err)
		}
		return links
	}

	const tag = "rollback-vless"
	merge(nodeInbound(tag, 45102, model.VLESS, shared, removed), 0, healthy)
	before := loadInbound(tag)
	beforeLinks := loadLinks(before.Id)
	beforeClient := lookupClientRecord(t, shared.Email)
	if len(beforeLinks) != 2 || linkCount(t, db, shared.Email) != 2 {
		t.Fatal("fixture requires two VLESS links and a shared VLESS/TUIC identity")
	}

	collision := shared
	collision.ID, collision.Flow = otherTuic.ID, ""
	rejected := nodeInbound(tag, 45102, model.VLESS, collision, attempted)
	rejected.Remark = "unaccepted config"
	merge(rejected, 11, healthy, newHealthy)

	gotClient := lookupClientRecord(t, shared.Email)
	if gotClient.UUID != beforeClient.UUID || gotClient.Flow != beforeClient.Flow {
		t.Errorf("failed sync persisted shared client UUID=%q flow=%q, want UUID=%q flow=%q",
			gotClient.UUID, gotClient.Flow, beforeClient.UUID, beforeClient.Flow)
	}
	if got := loadLinks(before.Id); !slices.Equal(got, beforeLinks) {
		t.Errorf("failed sync persisted VLESS links %+v, want %+v", got, beforeLinks)
	}
	if got, _ := countClientRows(t, db, attempted.Email); got != 0 {
		t.Errorf("failed sync persisted %d new client rows, want 0", got)
	}
	if got := loadInbound(tag); got.Settings != before.Settings || got.Remark != before.Remark {
		t.Errorf("failed sync persisted adopted settings or remark for %q", tag)
	}
	if got := readOrphanMark(t, db, removed.Email); got != 0 {
		t.Errorf("failed inbound's client was orphan-marked at %d", got)
	}

	sibling := loadInbound("rollback-healthy")
	if sibling.Up != 11 || linkCount(t, db, newHealthy.Email) != 1 {
		t.Errorf("healthy sibling did not commit its traffic and new client: up=%d", sibling.Up)
	}
	var baseline model.NodeClientTraffic
	if err := db.Where("node_id = ? AND email = ?", node.Id, healthy.Email).Take(&baseline).Error; err != nil {
		t.Fatal(err)
	}
	var usage xray.ClientTraffic
	if err := db.Where("email = ?", healthy.Email).Take(&usage).Error; err != nil {
		t.Fatal(err)
	}
	if baseline.Up != 11 || usage.Up != 11 {
		t.Errorf("healthy client baseline=%d traffic=%d, want 11/11", baseline.Up, usage.Up)
	}

	corrected := nodeInbound(tag, 45102, model.VLESS, shared, attempted)
	corrected.Remark = "accepted config"
	merge(corrected, 11, healthy, newHealthy)
	got := loadInbound(tag)
	if got.Settings != corrected.Settings || got.Remark != corrected.Remark || linkCount(t, db, attempted.Email) != 1 {
		t.Error("valid follow-up snapshot did not reconcile the previously rejected inbound")
	}
	if linkCount(t, db, removed.Email) != 0 || readOrphanMark(t, db, removed.Email) == 0 {
		t.Error("valid follow-up snapshot did not detach and orphan-mark the removed client")
	}
}

func TestNodeRefusedFirstAdoptionKeepsNodeSettings(t *testing.T) {
	testNodeRefusedFirstAdoptionKeepsNodeSettings(t)
}

func TestNodeRefusedFirstAdoptionKeepsNodeSettingsPostgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	testNodeRefusedFirstAdoptionKeepsNodeSettings(t)
}

func testNodeRefusedFirstAdoptionKeepsNodeSettings(t *testing.T) {
	t.Helper()
	db := initTrafficTestDB(t)
	var nodeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nodeCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	host, portString, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	node := &model.Node{
		Name: "first-adoption-7219", Scheme: "http", Address: host, Port: port, BasePath: "/",
		ApiToken: "test", Enable: true, AllowPrivateAddress: true,
	}
	seedNodeRow(t, db, node)
	remote := runtime.NewRemote(node, nil)
	useTestRuntimeManager(t).SetRuntimeOverride(node.Id, remote)
	svc := &InboundService{}

	newClient := func(email string) model.Client {
		return model.Client{ID: uuid.NewString(), Email: email, SubID: email, Password: email, Enable: true}
	}
	shared := newClient("shared@first-adoption")
	otherTuic := newClient("other-tuic@first-adoption")
	attempted := newClient("attempted@first-adoption")
	healthy := newClient("healthy@first-adoption")

	nodeInbound := func(tag string, port int, protocol model.Protocol, clients ...model.Client) *model.Inbound {
		return &model.Inbound{
			Tag: tag, Port: port, Protocol: protocol, Enable: true, Settings: clientsSettings(t, clients),
			TrafficReset: "never", TrafficResetDay: 1,
		}
	}
	merge := func(up int64, extra ...*model.Inbound) {
		t.Helper()
		sibling := nodeInbound("first-healthy", 45113, model.VLESS, healthy)
		sibling.Up = up
		sibling.ClientStats = []xray.ClientTraffic{{Email: healthy.Email, Up: up, Enable: true}}
		inbounds := []*model.Inbound{nodeInbound("first-tuic", 45111, model.TUIC, shared, otherTuic)}
		inbounds = append(append(inbounds, extra...), sibling)
		if _, err := svc.SetRemoteTraffic(node.Id, &runtime.TrafficSnapshot{Inbounds: inbounds}, false, false); err != nil {
			t.Fatalf("merge snapshot: %v", err)
		}
	}
	storedFor := func(tag string) []model.Inbound {
		t.Helper()
		var rows []model.Inbound
		if err := db.Where("node_id = ? AND tag = ?", node.Id, tag).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		return rows
	}

	merge(0)
	const tag = "first-vless"
	collision := shared
	collision.ID = otherTuic.ID
	refused := nodeInbound(tag, 45112, model.VLESS, collision, attempted)
	sharedBefore := lookupClientRecord(t, shared.Email)
	merge(11, refused)

	// A reconcile pushes the stored blob, so anything but the node's own would cost it clients.
	rows := storedFor(tag)
	if len(rows) != 1 || rows[0].Settings != refused.Settings {
		t.Fatalf("refused first adoption stored %d rows for %q, want one holding the node's settings", len(rows), tag)
	}
	if got, _ := countClientRows(t, db, attempted.Email); got != 0 {
		t.Errorf("refused first adoption persisted %d client rows, want 0", got)
	}
	var links int64
	if err := db.Model(&model.ClientInbound{}).Where("inbound_id = ?", rows[0].Id).Count(&links).Error; err != nil {
		t.Fatal(err)
	}
	if got := lookupClientRecord(t, shared.Email); links != 0 || got.UUID != sharedBefore.UUID {
		t.Errorf("refused first adoption left %d links and shared UUID %q, want 0 and %q", links, got.UUID, sharedBefore.UUID)
	}
	// The node refuses the blob it reported inconsistently, and a failed push keeps it dirty for good.
	payload, err := svc.buildInboundForNodePush(db, &rows[0])
	if err != nil {
		t.Fatal(err)
	}
	nodeCalls.Store(0)
	pushed, err := remote.ReconcileInbound(t.Context(), payload, true)
	if err != nil || pushed || nodeCalls.Load() != 0 {
		t.Errorf("reconcile of the refused inbound pushed=%t err=%v after %d node calls, want nothing sent", pushed, err, nodeCalls.Load())
	}
	var sibling model.Inbound
	if err := db.Where("node_id = ? AND tag = ?", node.Id, "first-healthy").Take(&sibling).Error; err != nil {
		t.Fatal(err)
	}
	var usage xray.ClientTraffic
	if err := db.Where("email = ?", healthy.Email).Take(&usage).Error; err != nil {
		t.Fatal(err)
	}
	if sibling.Up != 11 || usage.Up != 11 {
		t.Errorf("healthy sibling up=%d client up=%d, want 11/11", sibling.Up, usage.Up)
	}

	corrected := nodeInbound(tag, 45112, model.VLESS, shared, attempted)
	merge(11, corrected)
	rows = storedFor(tag)
	if len(rows) != 1 || rows[0].Settings != corrected.Settings || linkCount(t, db, attempted.Email) != 1 {
		t.Fatalf("corrected snapshot stored %d rows for %q, want one adopted inbound with its client linked", len(rows), tag)
	}
}
