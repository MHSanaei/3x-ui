package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

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

// A single failure is often a blip, and a switch moves every client to a new SNI, so only the
// third failed check in a row switches, and only to a spare that passed both checks.
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
	// the editor fills its form from this, or its Save would put the old target back
	if st := s.Status(ib.Id); st.Target != "c.example:443" || !reflect.DeepEqual(st.ServerNames, []string{"c.example"}) {
		t.Errorf("status after the switch: target %s names %v", st.Target, st.ServerNames)
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

// A target on this machine or the LAN (a local web server fronting REALITY, a bare port, a
// unix socket) is blocked by the probe's SSRF guard; that is not a failure to act on.
func TestRealityWatchLeavesLocalTargetsAlone(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true}, map[string]bool{"b.example:443": true})
	prev := realityWatchProbe
	realityWatchProbe = func(s *ServerService, target, sni string, xver int) *RealityScanResult {
		if target == "local.example:443" {
			return &RealityScanResult{Target: target, PrivateTarget: true, Reason: "connection failed: private address blocked"}
		}
		return prev(s, target, sni, xver)
	}
	s := &RealityWatchService{}
	var ids []int
	for i, target := range []string{"127.0.0.1:8443", "8443", "/run/web.sock", "local.example:443"} {
		ib := realityInbound(t, 4440+i, target, "a.example")
		if err := s.SaveConfig(ib.Id, RealityWatchEntry{Auto: true, Spares: []string{"b.example:443"}}); err != nil {
			t.Fatalf("SaveConfig(%s): %v", target, err)
		}
		ids = append(ids, ib.Id)
	}
	for range 4 {
		s.Tick(context.Background())
	}
	for _, id := range ids {
		target, _ := realityTargetOf(t, id)
		if target == "b.example:443" || s.Status(id).Fails != 0 {
			t.Errorf("local target %s: switched or counted as failing (fails %d)", target, s.Status(id).Fails)
		}
	}
}

// duringRealTest runs edit while the spare's real test is in flight, the slow part of a switch.
func duringRealTest(t *testing.T, edit func()) {
	t.Helper()
	prev := realityWatchRealTest
	realityWatchRealTest = func(ctx context.Context, target, sni string) error {
		edit()
		return prev(ctx, target, sni)
	}
	t.Cleanup(func() { realityWatchRealTest = prev })
}

// A switch used to save the copy read before its checks, wiping what the admin saved meanwhile.
func TestRealityWatchSwitchKeepsAnEditSavedDuringTheCheck(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true}, map[string]bool{"b.example:443": true})
	ib := realityInbound(t, 4441, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{"b.example:443"}}); err != nil {
		t.Fatal(err)
	}
	duringRealTest(t, func() {
		if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("remark", "edited").Error; err != nil {
			t.Error(err)
		}
	})
	if _, err := s.SwitchTo(context.Background(), ib.Id, "b.example:443"); err != nil {
		t.Fatalf("SwitchTo: %v", err)
	}
	got, err := (&InboundService{}).GetInbound(ib.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Remark != "edited" {
		t.Errorf("remark = %q, the switch overwrote the admin's edit", got.Remark)
	}
	if target, _ := realityTargetOf(t, ib.Id); target != "b.example:443" {
		t.Errorf("target = %s, want b.example:443", target)
	}
}

// An admin who fixes the target by hand mid-check has decided; the watcher must not override it.
func TestRealityWatchDoesNotSwitchATargetTheAdminJustChanged(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true}, map[string]bool{"b.example:443": true})
	ib := realityInbound(t, 4442, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Auto: true, Spares: []string{"b.example:443"}}); err != nil {
		t.Fatal(err)
	}
	s.Tick(context.Background())
	s.Tick(context.Background())
	duringRealTest(t, func() {
		cur, err := (&InboundService{}).GetInbound(ib.Id)
		if err != nil {
			t.Error(err)
			return
		}
		stream := strings.ReplaceAll(cur.StreamSettings, "a.example", "admin.example")
		if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("stream_settings", stream).Error; err != nil {
			t.Error(err)
		}
	})
	s.Tick(context.Background())
	if target, _ := realityTargetOf(t, ib.Id); target != "admin.example:443" {
		t.Errorf("target = %s, the watcher replaced the admin's choice", target)
	}
	st := s.Status(ib.Id)
	if !reflect.DeepEqual(st.Spares, []string{"b.example:443"}) {
		t.Errorf("spares = %v, want them untouched", st.Spares)
	}
	if len(st.Events) != 0 {
		t.Errorf("events = %+v, want none: nothing switched and a spare did pass", st.Events)
	}
}

// Checks take up to 10 s per probe and 25 s per real test; the watch API must answer meanwhile.
func TestRealityWatchApiAnswersDuringACheck(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, nil, nil)
	ib := realityInbound(t, 4443, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{"b.example:443"}}); err != nil {
		t.Fatal(err)
	}
	prev := realityWatchProbe
	t.Cleanup(func() { realityWatchProbe = prev })
	realityWatchProbe = func(sv *ServerService, target, sni string, xver int) *RealityScanResult {
		answered := make(chan struct{})
		go func() {
			s.Status(ib.Id)
			close(answered)
		}()
		select {
		case <-answered:
		case <-time.After(3 * time.Second):
			t.Error("Status blocked behind a running check")
		}
		return prev(sv, target, sni, xver)
	}
	s.Tick(context.Background())
}

// With the lock no longer held across checks, a slow cycle must not double-count failures.
func TestRealityWatchTicksDoNotOverlap(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, nil, nil)
	ib := realityInbound(t, 4444, "a.example:443", "a.example")
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{"b.example:443"}}); err != nil {
		t.Fatal(err)
	}
	prev := realityWatchProbe
	t.Cleanup(func() { realityWatchProbe = prev })
	calls := 0
	realityWatchProbe = func(sv *ServerService, target, sni string, xver int) *RealityScanResult {
		calls++
		if calls == 1 {
			nested := make(chan struct{})
			go func() {
				s.Tick(context.Background())
				close(nested)
			}()
			select {
			case <-nested:
			case <-time.After(3 * time.Second):
				t.Error("the overlapping tick waited instead of returning at once")
			}
		}
		return prev(sv, target, sni, xver)
	}
	s.Tick(context.Background())
	if calls != 1 {
		t.Errorf("target probed %d times, want the overlapping tick to do nothing", calls)
	}
	if st := s.Status(ib.Id); st.Fails != 1 {
		t.Errorf("fails = %d, want 1", st.Fails)
	}
}

// Missing the scanner's bar (no h2, SNI not on the cert) does not mean REALITY is broken: only
// a target with no TLS 1.3, or one failing the real connection, counts as a failed check.
func TestRealityWatchRealTestJudgesATargetTheScannerDislikes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		worksNow  bool
		tls13     bool
		wantFails int
		wantOk    bool
	}{
		{"works through REALITY", true, true, 0, true},
		{"fails through REALITY", false, true, 1, false},
		{"no TLS 1.3", true, false, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			stubRealityWatch(t, nil, map[string]bool{"a.example:443": tc.worksNow})
			prev := realityWatchProbe
			t.Cleanup(func() { realityWatchProbe = prev })
			realityWatchProbe = func(_ *ServerService, target, _ string, _ int) *RealityScanResult {
				return &RealityScanResult{Target: target, TLS13: tc.tls13, Reason: "server does not negotiate HTTP/2 (h2)"}
			}
			ib := realityInbound(t, 4445, "a.example:443", "a.example")
			s := &RealityWatchService{}
			if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{"b.example:443"}}); err != nil {
				t.Fatal(err)
			}
			s.Tick(context.Background())
			st := s.Status(ib.Id)
			if st.Fails != tc.wantFails || st.Ok != tc.wantOk {
				t.Errorf("fails = %d ok = %v, want %d %v", st.Fails, st.Ok, tc.wantFails, tc.wantOk)
			}
		})
	}
}

// A Host's own SNI overrides serverNames in links, so a stale one keeps clients cut off after
// a switch. Only rows of this inbound that use REALITY and pin one of its old names may change.
func TestRealityWatchSwitchMovesHostSnis(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true}, map[string]bool{"b.example:443": true})
	ib := realityInbound(t, 4446, "a.example:443", "a.example")
	other := realityInbound(t, 4447, "a.example:443", "a.example")
	rows := []*model.Host{
		{InboundId: ib.Id, Remark: "pinned", Security: "same", Sni: "a.example"},
		{InboundId: ib.Id, Remark: "pool", Security: "reality", Sni: "a.example", Snis: []string{"a.example", "custom.example"}},
		{InboundId: ib.Id, Remark: "custom", Security: "same", Sni: "custom.example"},
		{InboundId: ib.Id, Remark: "tls", Security: "tls", Sni: "a.example"},
		{InboundId: other.Id, Remark: "other", Security: "same", Sni: "a.example"},
	}
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	s := &RealityWatchService{}
	if err := s.SaveConfig(ib.Id, RealityWatchEntry{Spares: []string{"b.example:443"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SwitchTo(context.Background(), ib.Id, "b.example:443"); err != nil {
		t.Fatalf("SwitchTo: %v", err)
	}
	want := map[string]struct {
		sni  string
		snis []string
	}{
		"pinned": {sni: "b.example"},
		"pool":   {sni: "b.example", snis: []string{"b.example", "custom.example"}},
		"custom": {sni: "custom.example"},
		"tls":    {sni: "a.example"},
		"other":  {sni: "a.example"},
	}
	var got []model.Host
	if err := database.GetDB().Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	for _, h := range got {
		w := want[h.Remark]
		if h.Sni != w.sni || !slices.Equal(h.Snis, w.snis) {
			t.Errorf("host %s: sni %q snis %v, want %q %v", h.Remark, h.Sni, h.Snis, w.sni, w.snis)
		}
	}
}

// The UI disables the button for node inbounds because this server's tests say nothing about
// the node's network; the API must hold the same line.
func TestRealityWatchManualSwitchRefusesANodeInbound(t *testing.T) {
	setupBulkDB(t)
	stubRealityWatch(t, map[string]bool{"b.example:443": true}, map[string]bool{"b.example:443": true})
	ib := realityInbound(t, 4448, "a.example:443", "a.example")
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("node_id", 7).Error; err != nil {
		t.Fatal(err)
	}
	s := &RealityWatchService{}
	if _, err := s.SwitchTo(context.Background(), ib.Id, "b.example:443"); !errors.Is(err, errRealityWatchOnNode) {
		t.Fatalf("err = %v, want errRealityWatchOnNode", err)
	}
	if target, _ := realityTargetOf(t, ib.Id); target != "a.example:443" {
		t.Errorf("target = %s, want it kept", target)
	}
}
