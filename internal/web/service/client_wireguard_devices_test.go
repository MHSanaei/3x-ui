package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupWireguardDeviceClient(t *testing.T, email string, ports ...int) (*ClientService, []int) {
	t.Helper()
	setupBulkDB(t)
	svc := &ClientService{}
	ids := []int{}
	for _, port := range ports {
		ib := mkInbound(t, port, model.WireGuard, wgServerSettings())
		payload := &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, []model.Client{{Email: email, Enable: true}})}
		if len(ids) == 0 {
			if _, err := svc.AddInboundClient(&InboundService{}, payload); err != nil {
				t.Fatal(err)
			}
		} else if _, err := svc.AttachByEmail(&InboundService{}, email, []int{ib.Id}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, ib.Id)
	}
	return svc, ids
}

func TestWireguardDevicesCRUDAndLimit(t *testing.T) {
	svc, _ := setupWireguardDeviceClient(t, "devices@wg", 51910, 51911)
	initial, err := svc.WireguardDevices("devices@wg")
	if err != nil {
		t.Fatal(err)
	}
	if initial.Limit != 5 || len(initial.Inbounds) != 2 {
		t.Fatalf("initial list: %+v", initial)
	}
	primary := initial.Inbounds[0].Devices[0]
	if err := svc.SetWireguardDeviceLimit("devices@wg", 2); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddWireguardDevice("devices@wg", "Phone"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddWireguardDevice("devices@wg", "Laptop"); err == nil {
		t.Fatal("device limit was bypassed")
	}
	devices, err := svc.WireguardDevices("devices@wg")
	if err != nil {
		t.Fatal(err)
	}
	device := devices.Inbounds[0].Devices[1]
	if device.Id == "" || device.PublicKey == primary.PublicKey || device.PrivateKey == primary.PrivateKey {
		t.Fatal("device needs its own identity")
	}
	if device.AllowedIPs[0] == primary.AllowedIPs[0] {
		t.Fatal("device address collided with primary")
	}
	if devices.Inbounds[1].Devices[1].Id != device.Id {
		t.Fatal("device identity differs between inbounds")
	}
	if err := svc.SetWireguardDeviceLimit("devices@wg", 1); err == nil {
		t.Fatal("lowered limit below current count")
	}
	enabled := false
	name := "Work phone"
	if err := svc.UpdateWireguardDevice("devices@wg", device.Id, WireguardDevicePatch{Name: &name, Enable: &enabled}); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.WireguardDevices("devices@wg")
	if err != nil {
		t.Fatal(err)
	}
	for _, ib := range updated.Inbounds {
		if ib.Devices[1].Enable || ib.Devices[1].Name != name {
			t.Fatal("device patch was not applied to every inbound")
		}
	}
	if err := svc.DeleteWireguardDevice("devices@wg", device.Id); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddWireguardDevice("devices@wg", "Laptop"); err != nil {
		t.Fatal("deleted device did not release its slot:", err)
	}
	final, err := svc.WireguardDevices("devices@wg")
	if err != nil {
		t.Fatal(err)
	}
	if final.Inbounds[0].Devices[0].PrivateKey != primary.PrivateKey {
		t.Fatal("legacy primary config changed")
	}
	if final.Inbounds[0].Devices[1].AllowedIPs[0] != device.AllowedIPs[0] {
		t.Fatal("deleted address was not reusable")
	}
}

func TestWireguardDeviceWritesRollbackAcrossInbounds(t *testing.T) {
	svc, ids := setupWireguardDeviceClient(t, "rollback@wg", 51912, 51913)
	// A stale attachment must roll back the first inbound too.
	var ib model.Inbound
	if err := database.GetDB().First(&ib, ids[0]).Error; err != nil {
		t.Fatal(err)
	}
	original := ib.Settings
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ids[1]).Update("settings", `{"clients":[]}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.AddWireguardDevice("rollback@wg", "Phone"); err == nil {
		t.Fatal("missing settings client was accepted")
	}
	if err := database.GetDB().First(&ib, ids[0]).Error; err != nil {
		t.Fatal(err)
	}
	if ib.Settings != original {
		t.Fatal("failed fanout left a partially created device")
	}
}

func TestWireguardDeviceConcurrentAddsRespectLimit(t *testing.T) {
	svc, _ := setupWireguardDeviceClient(t, "concurrent@wg", 51914)
	if err := svc.SetWireguardDeviceLimit("concurrent@wg", 2); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		group.Go(func() { results <- svc.AddWireguardDevice("concurrent@wg", "Phone") })
	}
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful adds, want 1", successes)
	}
}

func TestWireguardDeviceMetadataEditPreservesDevices(t *testing.T) {
	svc, ids := setupWireguardDeviceClient(t, "metadata@wg", 51915)
	if err := svc.AddWireguardDevice("metadata@wg", "Phone"); err != nil {
		t.Fatal(err)
	}
	before, err := svc.WireguardDevices("metadata@wg")
	if err != nil {
		t.Fatal(err)
	}
	payload := &model.Inbound{Id: ids[0], Settings: clientsSettings(t, []model.Client{{Email: "metadata@wg", Enable: true, Comment: "updated"}})}
	if _, err := svc.UpdateInboundClient(&InboundService{}, payload, "metadata@wg"); err != nil {
		t.Fatal(err)
	}
	after, err := svc.WireguardDevices("metadata@wg")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before.Inbounds[0].Devices)
	b, _ := json.Marshal(after.Inbounds[0].Devices)
	if string(a) != string(b) {
		t.Fatalf("metadata update changed devices")
	}
}

// oldNodeRuntime is a node whose build ignores a client's extra WireGuard devices.
type oldNodeRuntime struct{ fakeNodeRuntime }

const oldNodeRefusal = "node old does not support WireGuard client devices"

func (*oldNodeRuntime) CheckWireguardDevices(context.Context) error {
	return errors.New(oldNodeRefusal)
}

// A device the node would never serve must be refused before it is stored: saved, the
// panel would hand out its config and leave the node waiting for a push it rejects.
func TestWireguardDeviceRefusedBeforeSaveOnNodeWithoutSupport(t *testing.T) {
	svc, ids := setupWireguardDeviceClient(t, "node@wg", 51930)
	mgr := useTestRuntimeManager(t)
	db := database.GetDB()
	node := &model.Node{Name: "old", Address: "127.0.0.1", Port: 2096, ApiToken: "tok", Enable: true, Status: "online"}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", ids[0]).Update("node_id", node.Id).Error; err != nil {
		t.Fatal(err)
	}
	deviceCount := func() int {
		t.Helper()
		list, err := svc.WireguardDevices("node@wg")
		if err != nil {
			t.Fatal(err)
		}
		return len(list.Inbounds[0].Devices)
	}
	nodeDirty := func() bool {
		t.Helper()
		_, _, dirty, _, err := (&NodeService{}).NodeSyncState(node.Id)
		if err != nil {
			t.Fatal(err)
		}
		return dirty
	}

	mgr.SetRuntimeOverride(node.Id, &oldNodeRuntime{})
	if err := svc.AddWireguardDevice("node@wg", "Phone"); err == nil || err.Error() != oldNodeRefusal {
		t.Fatalf("add on an old node: err = %v, want %q", err, oldNodeRefusal)
	}
	if got := deviceCount(); got != 1 || nodeDirty() {
		t.Fatalf("refused add left %d devices, node dirty = %v; want 1 and false", got, nodeDirty())
	}

	mgr.SetRuntimeOverride(node.Id, &fakeNodeRuntime{})
	if err := svc.AddWireguardDevice("node@wg", "Phone"); err != nil {
		t.Fatalf("add on a current node: %v", err)
	}
	if got := deviceCount(); got != 2 || !nodeDirty() {
		t.Fatalf("accepted add left %d devices, node dirty = %v; want 2 and true", got, nodeDirty())
	}
}
