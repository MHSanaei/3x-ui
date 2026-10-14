package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// inboundOpsProbe records every handler and user operation the panel sends the
// core, in order, and fails a remove or the first add the way xray does.
type inboundOpsProbe struct {
	fakeNodeRuntime
	mu        sync.Mutex
	ops       []string
	removeErr error
	addErr    error
}

func (p *inboundOpsProbe) record(op string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ops = append(p.ops, op)
}

func (p *inboundOpsProbe) AddInbound(context.Context, *model.Inbound) error {
	p.record("add-inbound")
	return nil
}

func (p *inboundOpsProbe) DelInbound(context.Context, *model.Inbound) error {
	p.record("del-inbound")
	return nil
}

func (p *inboundOpsProbe) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	p.record("update-inbound")
	return nil
}

func (p *inboundOpsProbe) AddUser(_ context.Context, _ *model.Inbound, user map[string]any) error {
	email, _ := user["email"].(string)
	p.record("add-user " + email)
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.addErr
	p.addErr = nil
	return err
}

func (p *inboundOpsProbe) RemoveUser(_ context.Context, _ *model.Inbound, email string) error {
	p.record("remove-user " + email)
	return p.removeErr
}

func (p *inboundOpsProbe) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.ops...)
}

func hotUsersClient(email, id string, enable bool) model.Client {
	return model.Client{Email: email, ID: id, Enable: enable, CreatedAt: 1, UpdatedAt: 1}
}

// A master pushes the whole inbound for every client it disables or renews; the
// node used to delete and re-add the handler, dropping every live connection.
func TestNodeInboundPushChangingOnlyClientsKeepsTheHandler(t *testing.T) {
	const (
		idA = "5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a301"
		idB = "5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a302"
	)
	active := []model.Client{hotUsersClient("a", idA, true), hotUsersClient("b", idB, true)}
	bDisabled := []model.Client{hotUsersClient("a", idA, true), hotUsersClient("b", idB, false)}

	cases := []struct {
		name           string
		stored, pushed []model.Client
		portShift      int
		enabling       bool
		removeErr      error
		addErr         error
		wantOps        []string
	}{
		{
			name: "disabling one client removes only that user", stored: active, pushed: bDisabled,
			wantOps: []string{"remove-user b"},
		},
		{
			name: "renewing a disabled client adds only that user", stored: bDisabled, pushed: active,
			wantOps: []string{"add-user b"},
		},
		{
			name: "a port change still rebuilds the handler", stored: active, pushed: active, portShift: 1,
			wantOps: []string{"del-inbound", "add-inbound"},
		},
		{
			name: "enabling a disabled inbound adds its handler", stored: active, pushed: active, enabling: true,
			wantOps: []string{"del-inbound", "add-inbound"},
		},
		{
			name: "a user the core no longer holds is not a rebuild", stored: active, pushed: bDisabled,
			removeErr: errors.New("failed to remove user: User b not found."),
			wantOps:   []string{"remove-user b"},
		},
		{
			name: "a handler the core lost is rebuilt", stored: active, pushed: bDisabled,
			removeErr: errors.New("failed to remove user: failed to get handler: hot-users > handler not found: hot-users"),
			wantOps:   []string{"remove-user b", "del-inbound", "add-inbound"},
		},
		{
			name: "a user the core already holds is replaced", stored: bDisabled, pushed: active,
			addErr:  errors.New("User b already exists."),
			wantOps: []string{"add-user b", "remove-user b", "add-user b"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			// A node keeps this default; the master's own setting decides node restarts.
			setRestartOnClientDisable(t, true)
			probe := &inboundOpsProbe{removeErr: tc.removeErr, addErr: tc.addErr}
			useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)

			seedInboundConflict(t, "hot-users", "0.0.0.0", 50081, model.VLESS, `{"network":"tcp"}`, clientsSettings(t, tc.stored))
			stored := loadInboundByTag(t, "hot-users")
			if err := (&ClientService{}).SyncInbound(nil, stored.Id, tc.stored); err != nil {
				t.Fatalf("SyncInbound: %v", err)
			}
			if tc.enabling {
				if err := database.GetDB().Model(stored).Update("enable", false).Error; err != nil {
					t.Fatalf("disable stored inbound: %v", err)
				}
			}

			pushed := *stored
			pushed.Settings = clientsSettings(t, tc.pushed)
			pushed.Port += tc.portShift
			pushed.Enable = true
			_, needRestart, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&pushed)
			if err != nil {
				t.Fatalf("UpdateInbound: %v", err)
			}
			if got := probe.recorded(); !slices.Equal(got, tc.wantOps) {
				t.Fatalf("core operations = %q, want %q", got, tc.wantOps)
			}
			if needRestart {
				t.Fatal("needRestart = true, want the push applied without restarting the core")
			}
		})
	}
}

// AmneziaWG has its own in-place reconfigure; deleting and re-adding the inbound
// rebuilt the device and dropped every peer's session on each pushed client change.
func TestNodeAmneziaWGPushReconfiguresTheDeviceInPlace(t *testing.T) {
	setupConflictDB(t)
	probe := &inboundOpsProbe{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)

	settings := awgRelayWindowSettings(t, "hot-awg")
	seedInboundConflict(t, "hot-awg", "0.0.0.0", 51830, model.AmneziaWG, "", settings)
	stored := loadInboundByTag(t, "hot-awg")
	clients, err := (&InboundService{}).GetClients(stored)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, stored.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	pushed := *stored
	pushed.Settings = strings.Replace(settings, `"enable":true`, `"enable":false`, 1)
	if _, _, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&pushed); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if got, want := probe.recorded(), []string{"update-inbound"}; !slices.Equal(got, want) {
		t.Fatalf("runtime operations = %q, want %q", got, want)
	}
}

// A master re-sends every inbound after it restarts; an unchanged one must not
// touch the core, even for a protocol whose users have no per-user API path.
func TestNodeInboundUnchangedPushLeavesTheCoreAlone(t *testing.T) {
	setupConflictDB(t)
	probe := &inboundOpsProbe{}
	useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)

	clients := []model.Client{{Email: "ss-a", Password: "a-pass", Enable: true, CreatedAt: 1, UpdatedAt: 1}}
	settings := `{"method":"chacha20-ietf-poly1305","network":"tcp,udp","clients":[{"method":"chacha20-ietf-poly1305","password":"a-pass","email":"ss-a","enable":true,"created_at":1,"updated_at":1}]}`
	seedInboundConflict(t, "hot-ss", "0.0.0.0", 50082, model.Shadowsocks, `{"network":"tcp"}`, settings)
	stored := loadInboundByTag(t, "hot-ss")
	if err := (&ClientService{}).SyncInbound(nil, stored.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	pushed := *stored
	if _, _, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&pushed); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if got := probe.recorded(); len(got) != 0 {
		t.Fatalf("core operations = %q, want none for an unchanged inbound", got)
	}
}
