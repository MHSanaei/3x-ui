package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/xtls/xray-core/app/proxyman/command"
	"google.golang.org/grpc"
)

// acceptingHandler is a core HandlerService that takes every user operation.
type acceptingHandler struct{ command.HandlerServiceClient }

func (acceptingHandler) AlterInbound(context.Context, *command.AlterInboundRequest, ...grpc.CallOption) (*command.AlterInboundResponse, error) {
	return &command.AlterInboundResponse{}, nil
}

// coreReAddProbe behaves like runtime.Local: Xray protocols go through the real
// account builder, sidecar protocols only change through UpdateInbound.
type coreReAddProbe struct {
	fakeNodeRuntime
	mu       sync.Mutex
	addUsers []map[string]any
	addErrs  []error
	updated  []model.Inbound
}

func (p *coreReAddProbe) AddUser(_ context.Context, ib *model.Inbound, user map[string]any) error {
	if servedOutsideXray(ib.Protocol) {
		return nil
	}
	var core command.HandlerServiceClient = acceptingHandler{}
	err := (&xray.XrayAPI{HandlerServiceClient: &core}).AddUser(string(ib.Protocol), ib.Tag, user)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.addUsers = append(p.addUsers, user)
	p.addErrs = append(p.addErrs, err)
	return err
}

func (p *coreReAddProbe) UpdateInbound(_ context.Context, _, newIb *model.Inbound) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updated = append(p.updated, *newIb)
	return nil
}

// seedQuotaDisabledClient leaves c the way the quota job does: off in the
// inbound JSON, in its traffic row and in its record, with the quota used up.
func seedQuotaDisabledClient(t *testing.T, port int, proto model.Protocol, c model.Client) (*model.Inbound, *coreReAddProbe) {
	t.Helper()
	setupBulkDB(t)
	probe := &coreReAddProbe{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)
	svc := &ClientService{}
	c.Enable = false
	c.TotalGB = 1000
	ib := mkInbound(t, port, proto, clientsSettings(t, []model.Client{c}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	mkTraffic(t, ib.Id, c.Email, 600, 500, 1000, 0, false)
	forceRecordDisabled(t, svc, c.Email)
	return ib, probe
}

const resetProbePeerKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="

// The reset re-added a WireGuard peer without publicKey/allowedIPs, the core
// rejected it, and the reset restarted Xray for every connected user.
func TestResetTrafficOfQuotaDisabledWireGuardPeerNeedsNoRestart(t *testing.T) {
	const email = "wg-reset@x"
	ib, probe := seedQuotaDisabledClient(t, 51840, model.WireGuard, model.Client{
		Email: email, SubID: "sub-" + email, PublicKey: resetProbePeerKey,
		AllowedIPs: []string{"10.0.0.7/32"}, KeepAlive: model.KeepAlivePtr(25),
	})

	needRestart, err := (&ClientService{}).ResetTrafficByEmail(&InboundService{}, email)
	if err != nil {
		t.Fatalf("ResetTrafficByEmail: %v", err)
	}
	probe.mu.Lock()
	users, errs := probe.addUsers, probe.addErrs
	probe.mu.Unlock()
	if len(users) != 1 {
		t.Fatalf("core re-adds = %d, want exactly one for %s", len(users), email)
	}
	if errs[0] != nil {
		t.Fatalf("the core rejected the re-added peer: %v", errs[0])
	}
	if got, _ := users[0]["publicKey"].(string); got != resetProbePeerKey {
		t.Fatalf("re-added peer publicKey = %q, want %q", got, resetProbePeerKey)
	}
	if needRestart {
		t.Fatal("resetting a quota-disabled WireGuard peer asked for an Xray restart")
	}
	assertEnableEverywhere(t, &ClientService{}, &InboundService{}, ib.Id, email, true)
}

// An AmneziaWG peer reaches the device only through the inbound reconcile;
// the reset's AddUser is a no-op there and left the peer off the device.
func TestResetTrafficOfQuotaDisabledAmneziaWGPeerReachesTheDevice(t *testing.T) {
	const email = "awg-reset@x"
	ib, probe := seedQuotaDisabledClient(t, 51841, model.AmneziaWG, model.Client{
		Email: email, SubID: "sub-" + email, PublicKey: resetProbePeerKey,
		AllowedIPs: []string{"10.8.1.7/32"},
	})

	needRestart, err := (&ClientService{}).ResetTrafficByEmail(&InboundService{}, email)
	if err != nil {
		t.Fatalf("ResetTrafficByEmail: %v", err)
	}
	if needRestart {
		t.Fatal("resetting a quota-disabled AmneziaWG peer asked for an Xray restart")
	}
	probe.mu.Lock()
	updated := probe.updated
	probe.mu.Unlock()
	for _, pushed := range updated {
		if pushed.Id == ib.Id && settingsClientEnabled(t, pushed.Settings, email) {
			return
		}
	}
	t.Fatalf("no inbound reconcile carried %s back enabled; pushed %d updates", email, len(updated))
}

func settingsClientEnabled(t *testing.T, settings, email string) bool {
	t.Helper()
	var parsed struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("parse pushed settings: %v", err)
	}
	for _, c := range parsed.Clients {
		if c.Email == email {
			return c.Enable
		}
	}
	return false
}

// The stored JSON keeps keepAlive as a number, which the core's account builder
// rejects; every auto-renew of such a WireGuard peer restarted Xray.
func TestAutoRenewOfWireGuardPeerWithKeepAliveNeedsNoRestart(t *testing.T) {
	const email = "wg-renew@x"
	ib, probe := seedQuotaDisabledClient(t, 51842, model.WireGuard, model.Client{
		Email: email, SubID: "sub-" + email, PublicKey: resetProbePeerKey,
		AllowedIPs: []string{"10.0.0.8/32"}, KeepAlive: model.KeepAlivePtr(25),
		Reset: 30, ExpiryTime: time.Now().Add(-48 * time.Hour).UnixMilli(),
	})
	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"reset": 30, "expiry_time": time.Now().Add(-48 * time.Hour).UnixMilli()}).Error; err != nil {
		t.Fatalf("make the traffic row renewable: %v", err)
	}
	svc := &InboundService{}
	batch := newTrafficMutationBatch()
	if _, count, err := svc.autoRenewClients(db, batch); err != nil || count != 1 {
		t.Fatalf("autoRenewClients: renewed %d, err %v; want 1 renewal", count, err)
	}

	needRestart := svc.applyTrafficMutationBatch(batch)
	probe.mu.Lock()
	errs := probe.addErrs
	probe.mu.Unlock()
	if len(errs) != 1 || errs[0] != nil {
		t.Fatalf("core re-adds of %s on inbound %d: %v, want one accepted add", email, ib.Id, errs)
	}
	if needRestart {
		t.Fatal("auto-renewing a WireGuard peer asked for an Xray restart")
	}
}

// Bulk enable built its own account map without the WireGuard keys, so
// enabling a peer restarted Xray for every user.
func TestBulkEnableOfWireGuardPeerNeedsNoRestart(t *testing.T) {
	const email = "wg-bulk@x"
	_, probe := seedQuotaDisabledClient(t, 51843, model.WireGuard, model.Client{
		Email: email, SubID: "sub-" + email, PublicKey: resetProbePeerKey,
		AllowedIPs: []string{"10.0.0.9/32"},
	})
	liftQuota(t, email)

	_, needRestart, err := (&ClientService{}).BulkSetEnable(&InboundService{}, []string{email}, true)
	if err != nil {
		t.Fatalf("BulkSetEnable: %v", err)
	}
	probe.mu.Lock()
	errs := probe.addErrs
	probe.mu.Unlock()
	if len(errs) != 1 || errs[0] != nil {
		t.Fatalf("core adds of %s: %v, want one accepted add", email, errs)
	}
	if needRestart {
		t.Fatal("bulk-enabling a WireGuard peer asked for an Xray restart")
	}
}

// Sidecar AddUser/RemoveUser are no-ops, so a bulk toggle must reach the device
// through the inbound reconcile, in both directions, from a hand-disabled client.
func TestBulkSetEnableOfSidecarClientReachesTheDevice(t *testing.T) {
	cases := []struct {
		proto  model.Protocol
		port   int
		client model.Client
	}{
		{model.AmneziaWG, 51844, model.Client{PublicKey: resetProbePeerKey, AllowedIPs: []string{"10.8.1.9/32"}}},
		{model.MTProto, 51845, model.Client{Secret: "ee1234567890abcdef1234567890abcd7777772e636c6f7564666c6172652e636f6d"}},
		{model.TUIC, 51846, model.Client{ID: "11111111-1111-1111-1111-111111111146", Password: "tuic-pass"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.proto), func(t *testing.T) {
			email := string(tc.proto) + "-bulk@x"
			c := tc.client
			c.Email, c.SubID, c.Enable, c.TotalGB = email, "sub-"+email, true, 1000
			setupBulkDB(t)
			probe := &coreReAddProbe{}
			useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)
			ib := mkInbound(t, tc.port, tc.proto, clientsSettings(t, []model.Client{c}))
			if err := (&ClientService{}).SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
				t.Fatalf("SyncInbound: %v", err)
			}
			mkTraffic(t, ib.Id, email, 0, 0, 1000, 0, true)
			for _, enable := range []bool{false, true} {
				if _, _, err := (&ClientService{}).BulkSetEnable(&InboundService{}, []string{email}, enable); err != nil {
					t.Fatalf("BulkSetEnable(%v): %v", enable, err)
				}
				probe.mu.Lock()
				updated := probe.updated
				probe.updated = nil
				probe.mu.Unlock()
				reached := false
				for _, pushed := range updated {
					if pushed.Id == ib.Id && settingsClientEnabled(t, pushed.Settings, email) == enable {
						reached = true
					}
				}
				if !reached {
					t.Fatalf("bulk enable=%v never reached the %s sidecar; pushed %d updates", enable, tc.proto, len(updated))
				}
			}
		})
	}
}

// liftQuota leaves the client switched off by hand only, the state bulk enable
// turns back on; a depleted one stays off the runtime whatever its flag says.
func liftQuota(t *testing.T, email string) {
	t.Helper()
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"enable": true, "up": 0, "down": 0}).Error; err != nil {
		t.Fatalf("lift quota of %s: %v", email, err)
	}
}
