package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/ha"
)

func seedHANode(t *testing.T, name string, dirty bool, dirtyAt int64) *model.Node {
	t.Helper()
	n := &model.Node{Name: name, Address: name + ".example", Port: 2053, Enable: true, ConfigDirty: dirty, ConfigDirtyAt: dirtyAt}
	if err := database.GetDB().Create(n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func nodeDirty(t *testing.T, id int) (bool, int64) {
	t.Helper()
	_, _, dirty, at, err := (&NodeService{}).NodeSyncState(id)
	if err != nil {
		t.Fatal(err)
	}
	return dirty, at
}

func activateHA(t *testing.T) {
	t.Helper()
	ha.SetActive(ha.NewMonitor(ha.Config{Self: "de"}, ha.Self{}, ha.Probes{}, 0))
	t.Cleanup(func() { ha.SetActive(nil) })
}

// The new master never trusts what the old one remembered pushing: every node
// must take a full reconcile on the first sync pass.
func TestPrepareHAMasterMarksEveryNodeDirty(t *testing.T) {
	setupConflictDB(t)
	clean := seedHANode(t, "clean", false, 0)
	off := seedHANode(t, "off", false, 0)
	if err := database.GetDB().Model(off).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	pending := seedHANode(t, "pending", true, 1000)

	if err := PrepareHAMaster(); err != nil {
		t.Fatalf("PrepareHAMaster: %v", err)
	}
	for _, n := range []*model.Node{clean, off, pending} {
		if dirty, _ := nodeDirty(t, n.Id); !dirty {
			t.Fatalf("node %s must be dirty after promotion", n.Name)
		}
	}
	// A reconcile the old master started for dirtyAt=1000 must not clear the
	// mark the new master just set.
	if err := (&NodeService{}).ClearNodeDirty(pending.Id, 1000); err != nil {
		t.Fatal(err)
	}
	if dirty, at := nodeDirty(t, pending.Id); !dirty || at <= 1000 {
		t.Fatalf("stale clear won: dirty=%v at=%d", dirty, at)
	}
}

func TestCheckHAReadyRefusesLocalInbounds(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "local-off", "", 18443, model.VLESS, `{"network":"tcp"}`, `{"clients":[]}`)
	if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", "local-off").Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	err := CheckHAReady()
	if err == nil || !strings.Contains(err.Error(), "local inbound") {
		t.Fatalf("a local inbound, even disabled, would move with the master; got %v", err)
	}
}

func TestCheckHAReadyRefusesUndecryptableNodeTokens(t *testing.T) {
	setupConflictDB(t)
	enableNodeTokenEncryption(t)
	if err := (&NodeService{}).Create(&model.Node{Name: "n1", Address: "n1.example", Port: 2053, ApiToken: "secret", Enable: true}); err != nil {
		t.Fatal(err)
	}
	if err := CheckHAReady(); err != nil {
		t.Fatalf("matching key: %v", err)
	}

	var other [32]byte
	other[0] = 0xAA
	wrong, err := nodetoken.NewCodec(nodetoken.ModeRequired, &nodetoken.Keyring{ActiveID: "t1", Keys: map[string][32]byte{"t1": other}})
	if err != nil {
		t.Fatal(err)
	}
	off, _ := nodetoken.NewCodec(nodetoken.ModeOff, nil)
	for name, codec := range map[string]*nodetoken.Codec{"another key": wrong, "encryption off": off} {
		nodetoken.Init(codec)
		err := CheckHAReady()
		if err == nil || !strings.Contains(err.Error(), "node token") {
			t.Fatalf("%s: a machine without the shared node-token key cannot drive the nodes; got %v", name, err)
		}
	}
}

func TestAddInboundWithoutNodeRefusedInHA(t *testing.T) {
	setupConflictDB(t)
	activateHA(t)
	ib := &model.Inbound{
		Tag: "local-new", Port: 18444, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp"}`,
	}
	_, _, err := (&InboundService{}).AddInbound(ib)
	if err == nil || !strings.Contains(err.Error(), "node") {
		t.Fatalf("HA panels host no inbounds of their own; got %v", err)
	}
	var count int64
	database.GetDB().Model(&model.Inbound{}).Where("tag = ?", "local-new").Count(&count)
	if count != 0 {
		t.Fatal("refused inbound must not be stored")
	}
}
