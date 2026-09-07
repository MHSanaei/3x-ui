package service

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// dirtyProbeRuntime reads a node's config_dirty flag at the moment an inbound
// pushes, i.e. while the edit's other inbounds are still unapplied.
type dirtyProbeRuntime struct {
	fakeNodeRuntime
	watch  int
	sawSet atomic.Bool
	probed atomic.Bool
}

func (d *dirtyProbeRuntime) UpdateUser(ctx context.Context, ib *model.Inbound, oldEmail string, c model.Client) error {
	var node model.Node
	if err := database.GetDB().Where("id = ?", d.watch).First(&node).Error; err == nil {
		d.sawSet.Store(node.ConfigDirty)
		d.probed.Store(true)
	}
	return d.fakeNodeRuntime.UpdateUser(ctx, ib, oldEmail, c)
}

// TestEditFlagsOtherNodesBeforeApplying pins the ordering: every node is flagged
// BEFORE any inbound applies, so no merge can slot into the fanout gap.
func TestEditFlagsOtherNodesBeforeApplying(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	mgr := useTestRuntimeManager(t)
	db := database.GetDB()

	const uuid = "eeeeeeee-1111-2222-3333-444444444444"
	probe := &dirtyProbeRuntime{}
	ids := fanoutNodeInbounds(t, mgr, probe, 2, 47200)

	// The watched node is the one whose apply is made to fail, so nothing but
	// the up-front marking can have flagged it when the other inbound pushes.
	var victim model.Inbound
	if err := db.Where("id = ?", ids[1]).First(&victim).Error; err != nil {
		t.Fatalf("read inbound %d: %v", ids[1], err)
	}
	probe.watch = *victim.NodeID

	if _, err := (&ClientService{}).Create(&InboundService{}, &ClientCreatePayload{
		Client:     model.Client{Email: "carol", ID: uuid, SubID: "sub-carol", Enable: true},
		InboundIds: ids,
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", ids[1]).
		Update("settings", `{"clients":`).Error; err != nil {
		t.Fatalf("corrupt inbound %d: %v", ids[1], err)
	}
	if err := db.Model(model.Node{}).Where("1 = 1").
		Update("config_dirty", false).Error; err != nil {
		t.Fatalf("clear config_dirty: %v", err)
	}

	rec := lookupClientRecord(t, "carol")
	if _, err := (&ClientService{}).Update(&InboundService{}, rec.Id, model.Client{
		Email: "carol-renamed", ID: uuid, SubID: "sub-carol", Enable: true,
	}, 0); err == nil {
		t.Fatal("Update on a corrupted inbound should report the failure")
	}

	if !probe.probed.Load() {
		t.Fatal("the healthy inbound never pushed, so the ordering was never observed")
	}
	if !probe.sawSet.Load() {
		t.Fatal("a node was still clean while another inbound of the same edit was applying: a snapshot merge in that gap would resurrect the pre-edit email")
	}
}

// TestEditFlagsNodesOutsideTheInboundFilter pins that the marking covers the
// client's whole attachment set, not just the inbounds the filter applies.
func TestEditFlagsNodesOutsideTheInboundFilter(t *testing.T) {
	setupBulkDB(t)
	startSerializedWriter(t)
	mgr := useTestRuntimeManager(t)
	db := database.GetDB()

	const uuid = "ffffffff-1111-2222-3333-444444444444"
	ids := fanoutNodeInbounds(t, mgr, &fakeNodeRuntime{}, 2, 47300)
	if _, err := (&ClientService{}).Create(&InboundService{}, &ClientCreatePayload{
		Client:     model.Client{Email: "dave", ID: uuid, SubID: "sub-dave", Enable: true},
		InboundIds: ids,
	}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	if err := db.Model(model.Node{}).Where("1 = 1").
		Update("config_dirty", false).Error; err != nil {
		t.Fatalf("clear config_dirty: %v", err)
	}

	// Edit scoped to the first inbound only; the second one's node still holds
	// the old email once the shared record is renamed.
	rec := lookupClientRecord(t, "dave")
	if _, err := (&ClientService{}).Update(&InboundService{}, rec.Id, model.Client{
		Email: "dave-renamed", ID: uuid, SubID: "sub-dave", Enable: true,
	}, 0, ids[0]); err != nil {
		t.Fatalf("filtered Update: %v", err)
	}

	var excluded model.Inbound
	if err := db.Where("id = ?", ids[1]).First(&excluded).Error; err != nil {
		t.Fatalf("read inbound %d: %v", ids[1], err)
	}
	var node model.Node
	if err := db.Where("id = ?", *excluded.NodeID).First(&node).Error; err != nil {
		t.Fatalf("read node %d: %v", *excluded.NodeID, err)
	}
	if !node.ConfigDirty {
		t.Fatal("a node left out of the inboundIds filter stayed clean after the shared record was renamed, so its stale snapshot would be merged")
	}
}
