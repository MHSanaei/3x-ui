package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func setupEnablePortTest(t *testing.T) {
	t.Helper()
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
}

func disableInboundRow(t *testing.T, id int) {
	t.Helper()
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", id).Update("enable", false).Error; err != nil {
		t.Fatalf("disable row %d: %v", id, err)
	}
}

// Saving a row while it is disabled skips every port guard, so enabling it later
// was the one path that could still put two inbounds on one socket.
func TestSetInboundEnableRefusesAPortAnotherEnabledInboundServes(t *testing.T) {
	setupEnablePortTest(t)
	seedInboundConflict(t, "holder", "0.0.0.0", 44431, model.VLESS, `{"network":"tcp"}`, `{}`)
	seedInboundConflict(t, "sleeper", "0.0.0.0", 44431, model.VLESS, `{"network":"tcp"}`, `{}`)
	sleeper := loadInboundByTag(t, "sleeper")
	disableInboundRow(t, sleeper.Id)

	_, err := (&InboundService{}).SetInboundEnable(sleeper.Id, true)
	if err == nil {
		t.Fatal("enabling a row onto a port another enabled inbound serves must be refused")
	}
	if !strings.Contains(err.Error(), "holder") {
		t.Fatalf("the refusal must name the row that owns the port; got %q", err)
	}
	if after := loadInboundByTag(t, "sleeper"); after.Enable {
		t.Fatal("a refused enable must not write the flag")
	}
}

// The enable path must keep the tcp/udp coexistence the rest of the guards
// allow, or it would refuse half of the working setups out there.
func TestSetInboundEnableAllowsTCPUDPCoexistence(t *testing.T) {
	setupEnablePortTest(t)
	seedInboundConflict(t, "udp-holder", "0.0.0.0", 44432, model.Hysteria, ``, `{}`)
	seedInboundConflict(t, "tcp-sleeper", "0.0.0.0", 44432, model.VLESS, `{"network":"tcp"}`, `{}`)
	sleeper := loadInboundByTag(t, "tcp-sleeper")
	disableInboundRow(t, sleeper.Id)

	if _, err := (&InboundService{}).SetInboundEnable(sleeper.Id, true); err != nil {
		t.Fatalf("a tcp inbound must be able to enable onto a udp-only row's port: %v", err)
	}
	if after := loadInboundByTag(t, "tcp-sleeper"); !after.Enable {
		t.Fatal("the row must be enabled")
	}
}

// Nodes run their own Xray, so a node row sharing a local port is legal and must
// stay enableable.
func TestSetInboundEnableAllowsANodeRowOnALocalPort(t *testing.T) {
	setupEnablePortTest(t)
	seedInboundConflict(t, "local-holder", "0.0.0.0", 44433, model.VLESS, `{"network":"tcp"}`, `{}`)

	node := &model.Node{Name: "n1", Address: "127.0.0.1", Port: 2096, Scheme: "https", Enable: true, Status: "online"}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	seedInboundConflictNode(t, "node-row", "0.0.0.0", 44433, model.VLESS, `{"network":"tcp"}`, `{}`, &node.Id)
	row := loadInboundByTag(t, "node-row")
	disableInboundRow(t, row.Id)

	if _, err := (&InboundService{}).SetInboundEnable(row.Id, true); err != nil {
		t.Fatalf("a node row must be enableable regardless of a local row's port: %v", err)
	}
}
