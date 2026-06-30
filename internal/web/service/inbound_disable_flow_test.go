package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

const visionTest = "xtls-rprx-vision"

// clientFlowsInSettings parses an inbound's settings JSON and returns a map of
// client email -> flow, for asserting what xray would read from the stored
// settings.
func clientFlowsInSettings(t *testing.T, settings string) map[string]string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	out := map[string]string{}
	clients, _ := parsed["clients"].([]any)
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		email, _ := cm["email"].(string)
		flow, _ := cm["flow"].(string)
		out[email] = flow
	}
	return out
}

// stripClientFlows must clear every client's flow and report whether it changed,
// leaving non-flow clients and malformed input untouched.
func TestStripClientFlows(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		wantChanged bool
		wantFlows   map[string]string
	}{
		{
			name:        "clears vision on all clients",
			in:          `{"clients":[{"email":"a","flow":"` + visionTest + `"},{"email":"b","flow":"` + visionTest + `"}]}`,
			wantChanged: true,
			wantFlows:   map[string]string{"a": "", "b": ""},
		},
		{
			name:        "mixed flows: clears only the non-empty",
			in:          `{"clients":[{"email":"a","flow":"` + visionTest + `"},{"email":"b","flow":""}]}`,
			wantChanged: true,
			wantFlows:   map[string]string{"a": "", "b": ""},
		},
		{
			name:        "no flows: unchanged",
			in:          `{"clients":[{"email":"a","flow":""},{"email":"b"}]}`,
			wantChanged: false,
			wantFlows:   map[string]string{"a": "", "b": ""},
		},
		{
			name:        "no clients: unchanged",
			in:          `{"decryption":"none"}`,
			wantChanged: false,
		},
		{
			name:        "malformed json: unchanged",
			in:          `{not json`,
			wantChanged: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed := stripClientFlows(tc.in)
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if !changed {
				if out != tc.in {
					t.Fatalf("unchanged input must be returned verbatim, got %q", out)
				}
				return
			}
			got := clientFlowsInSettings(t, out)
			for email, want := range tc.wantFlows {
				if got[email] != want {
					t.Errorf("flow[%s] = %q, want %q", email, got[email], want)
				}
			}
		})
	}
}

func initFlowTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return database.GetDB()
}

// AddInbound on a DisableFlow inbound must persist the column and clamp every
// client's flow to empty, both in settings and in the derived flow_override, so
// the live xray config never expects a flow the subscription suppresses.
func TestAddInbound_DisableFlowClampsClientFlow(t *testing.T) {
	initFlowTestDB(t)
	ibSvc := &InboundService{}

	in := &model.Inbound{
		Tag: "dis-add", Enable: true, Port: 52001, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"u1","email":"a@x","flow":"` + visionTest + `","subId":"s1","enable":true}]}`,
		DisableFlow:    true,
	}
	if _, _, err := ibSvc.AddInbound(in); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	got, err := ibSvc.GetInbound(in.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	if !got.DisableFlow {
		t.Error("DisableFlow not persisted on created inbound")
	}
	if f := clientFlowsInSettings(t, got.Settings)["a@x"]; f != "" {
		t.Errorf("settings flow = %q, want empty (clamped at creation)", f)
	}
	list, err := ibSvc.clientService.ListForInbound(nil, in.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if len(list) != 1 || list[0].Flow != "" {
		t.Errorf("flow_override = %#v, want empty (xray must not expect Vision)", list)
	}
}

// Toggling DisableFlow on via UpdateInbound (the #5689 path: editing an existing
// multi-inbound client's inbound) must persist the column, strip stored flows,
// and — crucially — survive the Vision-restore migration so it does not silently
// self-revert to Vision.
func TestUpdateInbound_DisableFlowPersistsStripsAndResistsRestore(t *testing.T) {
	db := initFlowTestDB(t)
	ibSvc := &InboundService{}
	cs := &ClientService{}

	const email = "shared@x"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0d001"

	// Sibling reality inbound (lowest id) where the client keeps Vision — this is
	// what EffectiveFlowsByEmails would otherwise propagate back onto the target.
	sibling := &model.Inbound{
		Tag: "sib", Enable: true, Port: 52101, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"` + visionTest + `","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(sibling).Error; err != nil {
		t.Fatalf("create sibling: %v", err)
	}
	sc, _ := ibSvc.GetClients(sibling)
	if err := cs.SyncInbound(nil, sibling.Id, sc); err != nil {
		t.Fatalf("sync sibling: %v", err)
	}

	// Target: a flow-eligible VLESS Reality inbound where the same client also has
	// Vision today. We will toggle DisableFlow on it.
	target := &model.Inbound{
		Tag: "tgt", Enable: true, Port: 52102, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"` + visionTest + `","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("create target: %v", err)
	}
	tc, _ := ibSvc.GetClients(target)
	if err := cs.SyncInbound(nil, target.Id, tc); err != nil {
		t.Fatalf("sync target: %v", err)
	}

	// Toggle DisableFlow on the target via the real update path.
	upd := *target
	upd.DisableFlow = true
	if _, _, err := ibSvc.UpdateInbound(&upd); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	// (a) column persisted
	reloaded, err := ibSvc.GetInbound(target.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	if !reloaded.DisableFlow {
		t.Fatal("DisableFlow did not persist through UpdateInbound (blocking regression)")
	}
	// (b) settings stripped
	if f := clientFlowsInSettings(t, reloaded.Settings)["shared@x"]; f != "" {
		t.Errorf("target settings flow = %q, want empty after disable", f)
	}
	// (c) flow_override empty
	list, err := cs.ListForInbound(nil, target.Id)
	if err != nil {
		t.Fatalf("ListForInbound(target): %v", err)
	}
	if len(list) != 1 || list[0].Flow != "" {
		t.Errorf("target flow_override = %#v, want empty", list)
	}

	// (d) the restore migration must NOT re-inject Vision onto the disabled inbound
	ibSvc.MigrationRestoreVisionFlow()
	reloaded2, err := ibSvc.GetInbound(target.Id)
	if err != nil {
		t.Fatalf("GetInbound after restore: %v", err)
	}
	if f := clientFlowsInSettings(t, reloaded2.Settings)["shared@x"]; f != "" {
		t.Errorf("after MigrationRestoreVisionFlow target flow = %q, want empty (must not self-revert)", f)
	}
	// Sibling still keeps its Vision (disable is per-inbound).
	sList, err := cs.ListForInbound(nil, sibling.Id)
	if err != nil {
		t.Fatalf("ListForInbound(sibling): %v", err)
	}
	if len(sList) != 1 || sList[0].Flow != visionTest {
		t.Errorf("sibling flow_override = %#v, want Vision preserved", sList)
	}
}
