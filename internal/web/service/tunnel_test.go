package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

type observingTunnelRuntime struct {
	applied []wattunnel.Spec
	durable bool
}

func (r *observingTunnelRuntime) ApplyTunnel(_ context.Context, s wattunnel.Spec) (wattunnel.Status, error) {
	var row model.ManagedTunnel
	if err := database.GetDB().Where("guid = ? AND revision = ?", s.GUID, s.Revision).First(&row).Error; err != nil {
		r.durable = false
	}
	r.applied = append(r.applied, s)
	return wattunnel.Status{State: "stopped", Revision: s.Revision}, nil
}

func (*observingTunnelRuntime) TunnelCapabilities(context.Context) (wattunnel.Capabilities, error) {
	return wattunnel.Capabilities{Supported: true, Version: wattunnel.Version, RawSocket: true}, nil
}

func initTunnelTest(t *testing.T) *observingTunnelRuntime {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	old := tunnelRuntimeFor
	oldStatuses := tunnelStatuses
	tunnelStatuses = map[string]wattunnel.Status{}
	rt := &observingTunnelRuntime{durable: true}
	tunnelRuntimeFor = func(*int) (runtime.TunnelRuntime, error) { return rt, nil }
	t.Cleanup(func() { tunnelRuntimeFor = old; tunnelStatuses = oldStatuses })
	return rt
}

func tunnelRequest() TunnelMutation {
	return TunnelMutation{Name: "test tunnel", Secret: "a-secret-not-in-api", Config: wattunnel.Config{Role: "client", Carrier: "tcp", Endpoint: "192.0.2.1", Port: 4443, Ports: []wattunnel.Port{{Listen: 18443, Target: 443, Protocol: "tcp"}}}}
}

func TestTunnelDurableStateAndSecretIsolation(t *testing.T) {
	rt := initTunnelTest(t)
	svc := &TunnelService{}
	req := tunnelRequest()
	view, err := svc.Save(context.Background(), 0, req)
	if err != nil {
		t.Fatal(err)
	}
	var row model.ManagedTunnel
	if err = database.GetDB().First(&row, view.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.Secret, "enc:v1:") || strings.Contains(row.Secret, req.Secret) {
		t.Fatal("stored plaintext secret")
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), req.Secret) || strings.Contains(string(raw), "enc:v1:") {
		t.Fatal("API exposed secret")
	}
	req.Revision = view.Revision
	req.Secret = ""
	req.Name = "renamed"
	view, err = svc.Save(context.Background(), view.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.Export(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := wattunnel.DecodeToken(token.Token)
	if err != nil || key != "a-secret-not-in-api" {
		t.Fatal("omitted edit key did not preserve original")
	}
	_, err = svc.Action(context.Background(), view.ID, "delete")
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	last := rt.applied[len(rt.applied)-1]
	if !last.Deleted || last.Enabled || !rt.durable {
		t.Fatalf("not durable or resurrected: %+v", last)
	}
	if _, err = svc.Save(context.Background(), view.ID, req); err == nil {
		t.Fatal("edited deleted tunnel")
	}
}

func TestTunnelCommitFailureDoesNotApplyRuntime(t *testing.T) {
	rt := initTunnelTest(t)
	db := database.GetDB()
	const callback = "tunnel-test-reject"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "managed_tunnels" {
			tx.AddError(errors.New("injected tunnel transaction failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
	_, err := (&TunnelService{}).Save(context.Background(), 0, tunnelRequest())
	if err == nil || !strings.Contains(err.Error(), "injected tunnel transaction failure") {
		t.Fatalf("wrong error: %v", err)
	}
	if len(rt.applied) != 0 {
		t.Fatal("runtime changed before durable write")
	}
}

func TestTunnelSyncRejectsForeignOwnerAndKeepsTombstone(t *testing.T) {
	initTunnelTest(t)
	svc := &TunnelService{}
	req := tunnelRequest()
	spec := wattunnel.Spec{GUID: uuid.NewString(), Owner: uuid.NewString(), Name: req.Name, Revision: 1, Config: req.Config, Secret: req.Secret}
	if _, err := svc.Sync(context.Background(), spec, "token:1"); err != nil {
		t.Fatal(err)
	}
	spec.Revision = 2
	if _, err := svc.Sync(context.Background(), spec, "token:2"); err == nil {
		t.Fatal("foreign identity changed tunnel")
	}
	spec.Deleted = true
	spec.Enabled = false
	if _, err := svc.Sync(context.Background(), spec, "token:1"); err != nil {
		t.Fatal(err)
	}
	spec.Revision = 1
	spec.Deleted = false
	if _, err := svc.Sync(context.Background(), spec, "token:1"); err == nil {
		t.Fatal("stale update resurrected tunnel")
	}
	var row model.ManagedTunnel
	if err := database.GetDB().Where("guid = ?", spec.GUID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.Deleted || row.Revision != 2 {
		t.Fatalf("lost tombstone: %+v", row)
	}
}

func TestTunnelPortConflictInBothDirections(t *testing.T) {
	initTunnelTest(t)
	svc := &TunnelService{}
	req := tunnelRequest()
	view, err := svc.Save(context.Background(), 0, req)
	if err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{Port: 18443, Protocol: model.Tunnel, Settings: `{"allowedNetwork":"tcp"}`, Listen: "0.0.0.0"}
	conflict, err := checkPortConflictTx(database.GetDB(), ib, 0)
	if err != nil {
		t.Fatal(err)
	}
	if conflict == nil || conflict.Tag != "tunnel:test tunnel" {
		t.Fatalf("inbound missed tunnel: %+v", conflict)
	}
	if _, err = svc.Action(context.Background(), view.ID, "delete"); err != nil {
		t.Fatal(err)
	}
	ib.Tag = "existing"
	if err = database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Save(context.Background(), 0, req); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("tunnel missed inbound: %v", err)
	}
	req.Config.Ports[0].Protocol = "udp"
	if _, err = svc.Save(context.Background(), 0, req); err != nil {
		t.Fatalf("TCP/UDP incorrectly collide: %v", err)
	}
}

func TestTunnelConcurrentEditsAndOfflineIntent(t *testing.T) {
	initTunnelTest(t)
	svc := &TunnelService{}
	req := tunnelRequest()
	view, err := svc.Save(context.Background(), 0, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Revision = view.Revision
	req.Secret = ""
	results := make(chan error, 2)
	for range 2 {
		go func() { _, e := svc.Save(context.Background(), view.ID, req); results <- e }()
	}
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("optimistic revision accepted zero or two concurrent edits: %v / %v", first, second)
	}
	tunnelRuntimeFor = func(*int) (runtime.TunnelRuntime, error) { return nil, errors.New("node offline") }
	views, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	req.Revision = views[0].Revision
	req.Enabled = true
	req.Name = "saved offline"
	view, err = svc.Save(context.Background(), view.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Enabled || view.Name != "saved offline" || view.Status.State != "error" {
		t.Fatalf("offline intent lost: %+v", view)
	}
	if _, err = svc.Action(context.Background(), view.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	var row model.ManagedTunnel
	if err = database.GetDB().First(&row, view.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Enabled {
		t.Fatal("offline stop not durable")
	}
}

func TestTunnelTombstoneBeforeFirstSecret(t *testing.T) {
	initTunnelTest(t)
	svc := &TunnelService{}
	dead := wattunnel.Spec{GUID: uuid.NewString(), Owner: "other-panel", Revision: 2, Deleted: true}
	if _, err := svc.Sync(context.Background(), dead, "token:1"); err != nil {
		t.Fatal(err)
	}
	req := tunnelRequest()
	if _, err := svc.Save(context.Background(), 0, req); err != nil {
		t.Fatalf("tombstone prevented keyring initialization: %v", err)
	}
}

func TestTunnelCommitFailureDoesNotApply(t *testing.T) {
	rt := initTunnelTest(t)
	db := database.GetDB()
	if err := db.Callback().Create().After("gorm:create").Register("test:rollback-tunnel", func(tx *gorm.DB) {
		if tx.Statement.Table == "managed_tunnels" {
			tx.Exec("ROLLBACK")
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test:rollback-tunnel") })
	if _, err := (&TunnelService{}).Save(context.Background(), 0, tunnelRequest()); err == nil {
		t.Fatal("commit failure reported success")
	}
	if len(rt.applied) != 0 {
		t.Fatal("uncommitted tunnel reached runtime")
	}
	var count int64
	if err := db.Model(&model.ManagedTunnel{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rolled back tunnel remained in database")
	}
}

func TestTunnelStopWithoutKeyring(t *testing.T) {
	rt := initTunnelTest(t)
	svc := &TunnelService{}
	view, err := svc.Save(context.Background(), 0, tunnelRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(wattunnel.KeyDir(), "keyring.json")); err != nil {
		t.Fatal(err)
	}
	before := len(rt.applied)
	if _, err = svc.Action(context.Background(), view.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	if len(rt.applied) != before+1 || rt.applied[before].Enabled {
		t.Fatal("missing keyring prevented process shutdown")
	}
	if _, err = svc.Export(view.ID); err == nil {
		t.Fatal("missing keyring silently recovered a secret")
	}
}

func TestTunnelReservesDerivedRelayPorts(t *testing.T) {
	for _, protocol := range []model.Protocol{model.TUIC, model.AmneziaWG} {
		t.Run(string(protocol), func(t *testing.T) {
			initTunnelTest(t)
			id := 77
			port := tuic.SOCKSPortForInbound(id)
			if protocol == model.AmneziaWG {
				port = amneziawgnet.SOCKSPortForInbound(id)
			}
			req := tunnelRequest()
			req.Config.Ports[0].Listen = port
			if _, err := (&TunnelService{}).Save(context.Background(), 0, req); err != nil {
				t.Fatal(err)
			}
			conflict, err := checkPortConflictTx(database.GetDB(), &model.Inbound{Id: id, Protocol: protocol, Port: 55555}, id)
			if err != nil {
				t.Fatal(err)
			}
			if conflict == nil {
				t.Fatal("derived relay collided with tunnel listener")
			}
		})
	}
}

func TestPanelPortCannotReplaceTunnelListener(t *testing.T) {
	initTunnelTest(t)
	req := tunnelRequest()
	if _, err := (&TunnelService{}).Save(context.Background(), 0, req); err != nil {
		t.Fatal(err)
	}
	if err := (&SettingService{}).SetPort(req.Config.Ports[0].Listen); err == nil {
		t.Fatal("panel port replaced a tunnel listener")
	}
}

func TestXrayAPIPortCannotReplaceTunnelListener(t *testing.T) {
	initTunnelTest(t)
	req := tunnelRequest()
	if _, err := (&TunnelService{}).Save(context.Background(), 0, req); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{"inbounds": []any{map[string]any{"tag": "api", "listen": "127.0.0.1", "port": req.Config.Ports[0].Listen, "protocol": "dokodemo-door", "settings": map[string]any{"address": "127.0.0.1"}}}}
	changed, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = (&XraySettingService{}).SaveXraySetting(string(changed)); err == nil || !strings.Contains(err.Error(), "tunnel") {
		t.Fatalf("Xray API port not rejected as tunnel conflict: %v", err)
	}
}
