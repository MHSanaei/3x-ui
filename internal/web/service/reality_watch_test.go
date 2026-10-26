package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// stubRealityWatch makes probe answer from feasible (target -> ok) and the real test from
// works; both record what they were asked so a test can tell what was tried.
func stubRealityWatch(t *testing.T, feasible, works map[string]bool) *[]string {
	t.Helper()
	var tried []string
	prevProbe, prevReal := realityWatchProbe, realityWatchRealTest
	realityWatchProbe = func(_ *ServerService, target, _ string, _ int) *RealityScanResult {
		if feasible[target] {
			return &RealityScanResult{Target: target, Feasible: true}
		}
		return &RealityScanResult{Target: target, Reason: "TLS handshake failed: EOF"}
	}
	realityWatchRealTest = func(_ context.Context, target, _ string) error {
		tried = append(tried, target)
		if works[target] {
			return nil
		}
		return errors.New("no reply through REALITY")
	}
	resetRealityWatchState()
	t.Cleanup(func() {
		realityWatchProbe, realityWatchRealTest = prevProbe, prevReal
		resetRealityWatchState()
	})
	return &tried
}

func realityInbound(t *testing.T, port int, target, sni string) *model.Inbound {
	t.Helper()
	stream := `{"network":"tcp","security":"reality","realitySettings":{"show":false,"xver":0,"target":"` +
		target + `","serverNames":["` + sni + `"],"privateKey":"k","shortIds":["ab"]}}`
	return mkInboundStream(t, port, model.VLESS, `{"clients":[],"decryption":"none"}`, stream)
}

func realityTargetOf(t *testing.T, id int) (string, []string) {
	t.Helper()
	ib, err := (&InboundService{}).GetInbound(id)
	if err != nil {
		t.Fatal(err)
	}
	var stream struct {
		RealitySettings struct {
			Target      string   `json:"target"`
			ServerNames []string `json:"serverNames"`
			PrivateKey  string   `json:"privateKey"`
		} `json:"realitySettings"`
	}
	if err := json.Unmarshal([]byte(ib.StreamSettings), &stream); err != nil {
		t.Fatal(err)
	}
	if stream.RealitySettings.PrivateKey != "k" {
		t.Fatalf("switch lost the other REALITY settings: %s", ib.StreamSettings)
	}
	return stream.RealitySettings.Target, stream.RealitySettings.ServerNames
}

// A target that fails once is often a blip; switching on it would move every client to a new
// SNI for nothing. Only the third failed check in a row switches, and only to a spare that
// passed both the server check and a real REALITY connection.
func TestRealityWatchSwitchesAfterThreeFailedChecks(t *testing.T) {
	setupBulkDB(t)
	tried := stubRealityWatch(t,
		map[string]bool{"b.example:443": true, "c.example:443": true},
		map[string]bool{"b.example:443": false, "c.example:443": true})
	ib := realityInbound(t, 4431, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Auto: true, Spares: []string{"b.example", "c.example:443"}}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	for check := 1; check <= 2; check++ {
		s.Tick(context.Background())
		if target, _ := realityTargetOf(t, ib.Id); target != "a.example:443" {
			t.Fatalf("after %d failed checks target = %s, want it kept", check, target)
		}
	}
	s.Tick(context.Background())
	target, names := realityTargetOf(t, ib.Id)
	if target != "c.example:443" || !reflect.DeepEqual(names, []string{"c.example"}) {
		t.Fatalf("after the third failed check target = %s %v, want c.example:443 [c.example]", target, names)
	}
	if !reflect.DeepEqual(*tried, []string{"b.example:443", "c.example:443"}) {
		t.Errorf("real-tested %v, want b (failed) then c", *tried)
	}
	st := s.Status(ib.Id)
	if !reflect.DeepEqual(st.Spares, []string{"b.example:443", "a.example:443"}) {
		t.Errorf("spares after the switch = %v, want b kept and the old target last", st.Spares)
	}
	if len(st.Events) != 1 || st.Events[0].From != "a.example:443" || st.Events[0].To != "c.example:443" {
		t.Errorf("events = %+v, want one switch a -> c", st.Events)
	}
}

// With no spare that works the inbound must stay as it is: a switch would only trade a
// broken target for another broken one. The outage is logged once, not every check.
func TestRealityWatchKeepsTheTargetWhenNoSpareWorks(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true}, nil)
	ib := realityInbound(t, 4432, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Auto: true, Spares: []string{"b.example:443"}}); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		s.Tick(context.Background())
	}
	if target, _ := realityTargetOf(t, ib.Id); target != "a.example:443" {
		t.Fatalf("target = %s, want a.example:443 kept", target)
	}
	st := s.Status(ib.Id)
	if st.Fails != 5 || len(st.Events) != 1 || st.Events[0].To != "" {
		t.Fatalf("fails %d, events %+v; want 5 fails and one 'no spare works' event", st.Fails, st.Events)
	}
}

// A good check in between starts the count again; auto off and node inbounds never switch.
func TestRealityWatchOnlySwitchesSustainedFailuresItOwns(t *testing.T) {
	setupBulkDB(t)
	feasible := map[string]bool{"b.example:443": true}
	stubRealityWatch(t, feasible, map[string]bool{"b.example:443": true})
	ib := realityInbound(t, 4433, "a.example:443", "a.example")
	manual := realityInbound(t, 4434, "a.example:443", "a.example")
	node := realityInbound(t, 4435, "a.example:443", "a.example")
	nodeID := 7
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", node.Id).Update("node_id", nodeID).Error; err != nil {
		t.Fatal(err)
	}
	s := &RealityWatchService{}
	for id, auto := range map[int]bool{ib.Id: true, manual.Id: false, node.Id: true} {
		if err := s.SaveConfig(id, RealityWatchEntry{Auto: auto, Spares: []string{"b.example:443"}}); err != nil {
			t.Fatal(err)
		}
	}

	s.Tick(context.Background())
	s.Tick(context.Background())
	feasible["a.example:443"] = true
	s.Tick(context.Background())
	delete(feasible, "a.example:443")
	s.Tick(context.Background())
	s.Tick(context.Background())
	if target, _ := realityTargetOf(t, ib.Id); target != "a.example:443" {
		t.Fatalf("2 fails, ok, 2 fails switched to %s; the good check must reset the count", target)
	}
	s.Tick(context.Background())
	for _, tc := range []struct {
		name string
		id   int
		want string
	}{{"auto", ib.Id, "b.example:443"}, {"auto off", manual.Id, "a.example:443"}, {"node inbound", node.Id, "a.example:443"}} {
		if target, _ := realityTargetOf(t, tc.id); target != tc.want {
			t.Errorf("%s: target = %s, want %s", tc.name, target, tc.want)
		}
	}
	if st := s.Status(manual.Id); st.Fails != 3 {
		t.Errorf("auto off still shows the checks: fails = %d, want 3 since the good one", st.Fails)
	}
	if st := s.Status(node.Id); st.Fails != 0 || st.CheckedAt != 0 {
		t.Errorf("node inbound was checked from this server (fails %d); its REALITY runs on the node", st.Fails)
	}
}

// The admin's switch button goes through the same real test: a spare that fails it is refused.
func TestRealityWatchManualSwitchNeedsTheRealTest(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true, "c.example:443": true},
		map[string]bool{"c.example:443": true})
	ib := realityInbound(t, 4436, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{"b.example:443", "c.example:443"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SwitchTo(context.Background(), ib.Id, "b.example:443"); !errors.Is(err, errRealityWatchTargetFails) {
		t.Fatalf("switch to a spare failing the real test: err = %v, want errRealityWatchTargetFails", err)
	}
	if target, _ := realityTargetOf(t, ib.Id); target != "a.example:443" {
		t.Fatalf("refused switch still changed the target to %s", target)
	}
	if ev, err := s.SwitchTo(context.Background(), ib.Id, "c.example:443"); err != nil || ev.To != "c.example:443" {
		t.Fatalf("switch to c: %+v, %v", ev, err)
	}
	if target, _ := realityTargetOf(t, ib.Id); target != "c.example:443" {
		t.Fatalf("target = %s, want c.example:443", target)
	}
}

func TestRealityWatchSaveConfigNormalizesSpares(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, nil, nil)
	ib := realityInbound(t, 4437, "a.example:443", "a.example")
	s := &RealityWatchService{}
	err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{" B.example ", "b.example:443", "c.example:8443", "a.example:443"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Status(ib.Id).Spares; !reflect.DeepEqual(got, []string{"b.example:443", "c.example:8443"}) {
		t.Errorf("spares = %v, want lower-cased, :443 added, repeats and the current target dropped", got)
	}
	for _, bad := range []string{"not a host", "a.example:99999", `x"; rm`} {
		if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{bad}}); !errors.Is(err, errRealityWatchBadSpare) {
			t.Errorf("spare %q: err = %v, want errRealityWatchBadSpare", bad, err)
		}
	}
	plain := mkInboundStream(t, 4438, model.VLESS, `{"clients":[]}`, `{"network":"tcp","security":"tls"}`)
	if err := s.SaveConfig(plain.Id, RealityWatchEntry{Auto: true}); !errors.Is(err, errRealityWatchNotReality) {
		t.Errorf("watch on a TLS inbound: err = %v, want errRealityWatchNotReality", err)
	}
}
