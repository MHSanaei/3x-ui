package service

import (
	"context"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// reverseUserProbe records the account maps the panel pushes to the core: the
// last place a stored reverse tag can be dropped before it reaches a listener.
type reverseUserProbe struct {
	fakeNodeRuntime
	mu    sync.Mutex
	users []map[string]any
}

func (p *reverseUserProbe) AddUser(_ context.Context, _ *model.Inbound, user map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.users = append(p.users, user)
	return nil
}

func (p *reverseUserProbe) recorded() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]map[string]any(nil), p.users...)
}

const reverseProbeID = "5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a333"

func reverseProbeClient(email string, enable bool) model.Client {
	return model.Client{Email: email, ID: reverseProbeID, Enable: enable, Reverse: &model.ClientReverse{Tag: "portal"}}
}

// seedReverseProbeInbound seeds one local vless inbound holding a single reverse
// client, and the recording runtime every local apply of it lands on.
func seedReverseProbeInbound(t *testing.T, tag string, port int, enable bool) (*model.Inbound, string, *reverseUserProbe) {
	t.Helper()
	setupConflictDB(t)
	mgr := useTestRuntimeManager(t)
	probe := &reverseUserProbe{}
	mgr.SetLocalRuntimeOverride(probe)

	email := tag + "@example.test"
	client := reverseProbeClient(email, enable)
	seedInboundConflict(t, tag, "0.0.0.0", port, model.VLESS, `{"network":"tcp"}`, clientsSettings(t, []model.Client{client}))
	inbound := loadInboundByTag(t, tag)
	if err := (&ClientService{}).SyncInbound(nil, inbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	return inbound, email, probe
}

// assertReverseReAdd fails unless the core was handed the client's own tag: the
// handler is gone the moment RemoveUser runs, and only the tag rebuilds it.
func assertReverseReAdd(t *testing.T, probe *reverseUserProbe, email string) {
	t.Helper()
	users := probe.recorded()
	if len(users) == 0 {
		t.Fatalf("%s was never re-added to the core, so its reverse tag was never checked", email)
	}
	found := false
	for _, user := range users {
		if got, _ := user["email"].(string); got != email {
			continue
		}
		found = true
		tag, _ := user["reverse"].(*model.ClientReverse)
		if tag == nil || tag.Tag != "portal" {
			t.Fatalf("the re-add of %s carries reverse %#v, want its stored tag portal", email, user["reverse"])
		}
	}
	if !found {
		t.Fatalf("no re-add of %s reached the core: %v", email, users)
	}
}

// The panel's most ordinary action on a reverse client: editing it removes the
// account and adds it back, and the core rebuilds nothing without the tag.
func TestClientEditKeepsTheReverseTag(t *testing.T) {
	_, email, probe := seedReverseProbeInbound(t, "rev-edit", 50071, true)
	rec := lookupClientRecord(t, email)

	edited := reverseProbeClient(email, true)
	edited.Comment = "edited after the tunnel was up"
	if _, err := (&ClientService{}).Update(&InboundService{}, rec.Id, edited, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertReverseReAdd(t, probe, email)
}

func TestBulkReEnableKeepsTheReverseTag(t *testing.T) {
	_, email, probe := seedReverseProbeInbound(t, "rev-bulk", 50072, false)

	if _, _, err := (&ClientService{}).BulkSetEnable(&InboundService{}, []string{email}, true); err != nil {
		t.Fatalf("BulkSetEnable: %v", err)
	}
	assertReverseReAdd(t, probe, email)
}

// The route an operator hits most often: a client that exhausted its quota is
// removed, then re-added by the reset that renews it.
func TestTrafficResetKeepsTheReverseTag(t *testing.T) {
	inbound, email, probe := seedReverseProbeInbound(t, "rev-quota", 50073, true)
	depleteClientTraffic(t, inbound.Id, email)

	if _, err := (&InboundService{}).ResetClientTraffic(inbound.Id, email); err != nil {
		t.Fatalf("ResetClientTraffic: %v", err)
	}
	assertReverseReAdd(t, probe, email)
}

func TestAddingClientsKeepsTheReverseTag(t *testing.T) {
	inbound, _, probe := seedReverseProbeInbound(t, "rev-add", 50074, true)
	const added = "rev-add-second@example.test"
	second := reverseProbeClient(added, true)
	second.ID = "7c3fad07-4b1c-4d66-9f83-7db2f3c8b444"

	if _, err := (&ClientService{}).AddInboundClient(&InboundService{}, &model.Inbound{
		Id:       inbound.Id,
		Protocol: model.VLESS,
		Settings: clientsSettings(t, []model.Client{second}),
	}); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}
	assertReverseReAdd(t, probe, added)
}

// depleteClientTraffic leaves the client enabled in settings but out of quota,
// the state a traffic reset re-adds it from.
func depleteClientTraffic(t *testing.T, inboundId int, email string) {
	t.Helper()
	db := database.GetDB()
	res := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"enable": false, "up": 1, "down": 1})
	if res.Error != nil {
		t.Fatalf("deplete traffic: %v", res.Error)
	}
	if res.RowsAffected == 0 {
		if err := db.Create(&xray.ClientTraffic{InboundId: inboundId, Email: email, Enable: false, Up: 1, Down: 1}).Error; err != nil {
			t.Fatalf("create depleted traffic: %v", err)
		}
	}
}
