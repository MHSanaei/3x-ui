package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

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
// account builder, AmneziaWG and MTProto only change through UpdateInbound.
type coreReAddProbe struct {
	fakeNodeRuntime
	mu       sync.Mutex
	addUsers []map[string]any
	addErrs  []error
	updated  []model.Inbound
}

func (p *coreReAddProbe) AddUser(_ context.Context, ib *model.Inbound, user map[string]any) error {
	if ib.Protocol == model.AmneziaWG || ib.Protocol == model.MTProto {
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
