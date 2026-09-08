package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The bulk ops walked their inbounds one node round-trip at a time, so a client
// spanning several nodes cost the SUM of every node's latency. Each test below
// times out on the barrier unless the pushes overlap.

func TestBulkDeleteAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	bar := newApplyBarrier(inboundFanoutConcurrency)
	seedClientAcrossNodes(t, bar, nodes, 46101, "bulkdel@x", "aaaaaaaa-1111-2222-3333-444444444444")

	bar.arm()
	if _, _, err := (&ClientService{}).BulkDelete(&InboundService{}, []string{"bulkdel@x"}, false); err != nil {
		t.Fatalf("BulkDelete across %d node inbounds: %v", nodes, err)
	}
	if got := bar.deleteClient.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

func TestBulkSetEnableAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	bar := newApplyBarrier(inboundFanoutConcurrency)
	seedClientAcrossNodes(t, bar, nodes, 46201, "bulkena@x", "bbbbbbbb-1111-2222-3333-444444444444")

	bar.arm()
	if _, _, err := (&ClientService{}).BulkSetEnable(&InboundService{}, []string{"bulkena@x"}, false); err != nil {
		t.Fatalf("BulkSetEnable across %d node inbounds: %v", nodes, err)
	}
	if got := bar.updateUser.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

func TestBulkAdjustAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const email = "bulkadj@x"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	mgr := useTestRuntimeManager(t)
	ids := fanoutNodeInbounds(t, mgr, bar, nodes, 46301)
	// An expiry to extend, or BulkAdjust reports the client ineligible and
	// never reaches a node at all.
	if _, err := (&ClientService{}).Create(&InboundService{}, &ClientCreatePayload{
		Client: model.Client{
			Email: email, ID: "cccccccc-1111-2222-3333-444444444444", SubID: "sub-" + email,
			Enable: true, ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli(),
		},
		InboundIds: ids,
	}); err != nil {
		t.Fatalf("seed Create across %d node inbounds: %v", nodes, err)
	}

	bar.arm()
	if _, _, err := (&ClientService{}).BulkAdjust(&InboundService{}, []string{email}, 1, 0, ""); err != nil {
		t.Fatalf("BulkAdjust across %d node inbounds: %v", nodes, err)
	}
	if got := bar.updateUser.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

func TestBulkAttachAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const email = "bulkatt@x"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	mgr := useTestRuntimeManager(t)
	// Seeded on the first inbound only, so the other nodes are all attach work.
	ids := fanoutNodeInbounds(t, mgr, bar, nodes, 46501)
	if _, err := (&ClientService{}).Create(&InboundService{}, &ClientCreatePayload{
		Client:     model.Client{Email: email, ID: "eeeeeeee-1111-2222-3333-444444444444", SubID: "sub-" + email, Enable: true},
		InboundIds: ids[:1],
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	bar.arm()
	if _, _, err := (&ClientService{}).BulkAttach(&InboundService{}, []string{email}, ids[1:]); err != nil {
		t.Fatalf("BulkAttach across %d node inbounds: %v", nodes-1, err)
	}
	if got := bar.addClient.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

func TestBulkCreateAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	bar := newApplyBarrier(inboundFanoutConcurrency)
	mgr := useTestRuntimeManager(t)
	ids := fanoutNodeInbounds(t, mgr, bar, nodes, 46601)

	bar.arm()
	if _, _, err := (&ClientService{}).BulkCreate(&InboundService{}, []ClientCreatePayload{{
		Client:     model.Client{Email: "bulknew@x", ID: "ffffffff-1111-2222-3333-444444444444", SubID: "sub-bulknew", Enable: true},
		InboundIds: ids,
	}}); err != nil {
		t.Fatalf("BulkCreate across %d node inbounds: %v", nodes, err)
	}
	if got := bar.addClient.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

func TestBulkDetachAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const email = "bulkdet@x"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	recID := seedClientAcrossNodes(t, bar, nodes, 46401, email, "dddddddd-1111-2222-3333-444444444444")
	ids, err := (&ClientService{}).GetInboundIdsForRecord(recID)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}

	bar.arm()
	if _, _, err := (&ClientService{}).BulkDetach(&InboundService{}, []string{email}, ids); err != nil {
		t.Fatalf("BulkDetach across %d node inbounds: %v", nodes, err)
	}
	if got := bar.deleteUser.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

// TestApplyClientFieldAcrossNodesPushesConcurrently covers the field-edit path
// the Telegram bot and the LDAP job use (enable toggle, ip/expiry/traffic reset).
func TestApplyClientFieldAcrossNodesPushesConcurrently(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)

	const nodes = inboundFanoutConcurrency + 1
	const email = "fieldedit@x"
	bar := newApplyBarrier(inboundFanoutConcurrency)
	seedClientAcrossNodes(t, bar, nodes, 46701, email, "99999999-1111-2222-3333-444444444444")

	bar.arm()
	if _, err := (&ClientService{}).ResetClientIpLimitByEmail(&InboundService{}, email, 3); err != nil {
		t.Fatalf("ResetClientIpLimitByEmail across %d node inbounds: %v", nodes, err)
	}
	if got := bar.updateUser.Load(); got == 0 {
		t.Fatalf("no node push reached the barrier at all")
	}
	if got := bar.maxPar.Load(); got != inboundFanoutConcurrency {
		t.Fatalf("peak node pushes in flight = %d, want overlap at the %d cap (barrier timed out: %v)",
			got, inboundFanoutConcurrency, bar.expired.Load())
	}
}

// TestBulkCreateSerializesTunnelAddressAllocation pins that a bulk create over
// WireGuard inbounds does not overlap: allocation reads every inbound's used-set
// before writing, so two concurrent picks collide and the second is refused.
func TestBulkCreateSerializesTunnelAddressAllocation(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	db := database.GetDB()

	ids := make([]int, 0, 2)
	for i := range 2 {
		ib := &model.Inbound{
			UserId: 1, Enable: true, Port: 51820 + i,
			Tag: fmt.Sprintf("wg-%d", i), Protocol: model.WireGuard,
			Settings: `{"clients":[],"mtu":1420,"secretKey":"QO3O1V0m0Sm1yQ0hVvJ0kM0kQe0mYq0Wc0Zk0Xs0Zm8=","peers":[]}`,
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create wg inbound: %v", err)
		}
		ids = append(ids, ib.Id)
	}

	res, _, err := (&ClientService{}).BulkCreate(&InboundService{}, []ClientCreatePayload{
		{Client: model.Client{Email: "a@wg", SubID: "sa", Enable: true}, InboundIds: []int{ids[0]}},
		{Client: model.Client{Email: "b@wg", SubID: "sb", Enable: true}, InboundIds: []int{ids[1]}},
	})
	if err != nil {
		t.Fatalf("BulkCreate over two wg inbounds: %v", err)
	}
	if res.Created != 2 {
		t.Fatalf("created = %d, want 2 — concurrent allocation handed out one address twice: %+v", res.Created, res.Skipped)
	}
}
