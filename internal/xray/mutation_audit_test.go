package xray

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// ---------------------------------------------------------------------------
// hot_diff.go mutation audits
// ---------------------------------------------------------------------------

// TestDiffOutbounds_EmptyOutboundsNoPanic pins hot_diff.go:154 — the
// `len(oldOut) > 0` guard that protects the oldOut[0]/newOut[0] index. With no
// outbounds on either side the first-outbound identity check must be SKIPPED
// (an empty hot diff), never executed; a mutated guard (`>= 0`) would index a
// nil slice and panic.
func TestDiffOutbounds_EmptyOutboundsNoPanic(t *testing.T) {
	oldCfg := makeHotConfig()
	oldCfg.OutboundConfigs = nil
	newCfg := makeHotConfig()
	newCfg.OutboundConfigs = nil

	diff, ok := ComputeHotDiff(oldCfg, newCfg)
	if !ok {
		t.Fatal("identical empty-outbound configs must be hot-appliable")
	}
	if len(diff.RemovedOutboundTags) != 0 || len(diff.AddedOutbounds) != 0 {
		t.Fatalf("no outbounds on either side must yield no outbound ops, got %+v", diff)
	}
}

// TestDiffOutbounds_SingleFirstOutboundChangeNeedsRestart pins the other side
// of the hot_diff.go:154 boundary. With exactly ONE outbound, changing its
// content touches the default (first) handler, which has no replace API — it
// must force a restart. A mutated guard (`> 1`) would skip the first-outbound
// check at this length and wrongly classify the change as hot-appliable.
func TestDiffOutbounds_SingleFirstOutboundChangeNeedsRestart(t *testing.T) {
	oldCfg := makeHotConfig()
	oldCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"}]`)
	newCfg := makeHotConfig()
	newCfg.OutboundConfigs = json_util.RawMessage(`[{"protocol":"freedom","settings":{"domainStrategy":"UseIP"},"tag":"direct"}]`)

	if _, ok := ComputeHotDiff(oldCfg, newCfg); ok {
		t.Fatal("changing the only (default) outbound must force a restart")
	}
}

// TestRoutingWithoutReloadable_EmptyInput pins hot_diff.go:219 — the
// `len(raw) > 0` guard that skips JSON decoding of empty input. Empty input
// must canonicalize to the empty object `{}` with ok=true (no rules/balancers
// to strip). A mutated guard (`>= 0`) would feed an empty reader to the JSON
// decoder, get io.EOF, and wrongly return ok=false.
func TestRoutingWithoutReloadable_EmptyInput(t *testing.T) {
	out, ok := routingWithoutReloadable([]byte{})
	if !ok {
		t.Fatal("empty routing input must canonicalize successfully")
	}
	if string(out) != "{}" {
		t.Fatalf("empty routing input must canonicalize to {}, got %q", out)
	}

	// nil input behaves the same as empty.
	out, ok = routingWithoutReloadable(nil)
	if !ok || string(out) != "{}" {
		t.Fatalf("nil routing input must canonicalize to {}, ok=%v out=%q", ok, out)
	}
}

// TestRoutingWithoutReloadable_StripsRulesAndBalancers complements the guard
// test: with real content the reloadable keys (rules, balancers) are removed
// and only the restart-only remainder is returned. This pins that a routing
// change limited to rules/balancers leaves an identical remainder.
func TestRoutingWithoutReloadable_StripsRulesAndBalancers(t *testing.T) {
	a, ok := routingWithoutReloadable([]byte(`{"domainStrategy":"AsIs","rules":[{"x":1}],"balancers":[{"y":2}]}`))
	if !ok {
		t.Fatal("valid routing input must parse")
	}
	b, ok := routingWithoutReloadable([]byte(`{"domainStrategy":"AsIs","rules":[],"balancers":[]}`))
	if !ok {
		t.Fatal("valid routing input must parse")
	}
	if string(a) != string(b) {
		t.Fatalf("rules/balancers must be stripped: %q != %q", a, b)
	}
	if string(a) != `{"domainStrategy":"AsIs"}` {
		t.Fatalf("remainder must keep only restart-only keys, got %q", a)
	}
}

// TestApiTagFromConfig pins hot_diff.go:357 — the three-part guard
// `len(api) > 0 && Unmarshal == nil && parsed.Tag != ""`. Each conjunct must
// hold for a custom tag to be honored; otherwise the default "api" is used.
func TestApiTagFromConfig(t *testing.T) {
	cases := []struct {
		name string
		api  string
		want string
	}{
		{"empty input falls back to api", "", "api"},
		{"explicit tag honored", `{"tag":"my-api"}`, "my-api"},
		{"empty tag falls back to api", `{"tag":""}`, "api"},
		{"missing tag falls back to api", `{"services":["StatsService"]}`, "api"},
		{"unparsable falls back to api", `{not-json`, "api"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := apiTagFromConfig(json_util.RawMessage(tc.api))
			if got != tc.want {
				t.Fatalf("apiTagFromConfig(%q) = %q, want %q", tc.api, got, tc.want)
			}
		})
	}
}

// TestApiTagDrivesInboundRestartGuard ties hot_diff.go:357 to its consumer:
// the api tag resolved from the api section is the tag whose inbound change
// forces a restart. With a custom api.tag, changing that inbound must NOT be
// hot-appliable (it carries the gRPC server the panel talks through).
func TestApiTagDrivesInboundRestartGuard(t *testing.T) {
	oldCfg := makeHotConfig()
	oldCfg.API = json_util.RawMessage(`{"services":["HandlerService"],"tag":"custom-api"}`)
	oldCfg.InboundConfigs[0].Tag = "custom-api"
	newCfg := makeHotConfig()
	newCfg.API = json_util.RawMessage(`{"services":["HandlerService"],"tag":"custom-api"}`)
	newCfg.InboundConfigs[0].Tag = "custom-api"
	newCfg.InboundConfigs[0].Port = 62790 // change the custom-api inbound

	if _, ok := ComputeHotDiff(oldCfg, newCfg); ok {
		t.Fatal("changing the inbound named by a custom api.tag must force a restart")
	}
}

// ---------------------------------------------------------------------------
// process.go mutation audits (pure-logic, cross-platform)
// ---------------------------------------------------------------------------

// TestIsRunning_ExitedProcessWithClosedDone pins process.go:240 — the
// `if p.done != nil` guard that decides whether to consult the done channel.
// When the process has exited (done closed) but ProcessState has not yet been
// observed, IsRunning must report false via the closed-channel select. A
// mutated guard (`== nil`) would skip the select and wrongly report true.
func TestIsRunning_ExitedProcessWithClosedDone(t *testing.T) {
	p := newProcess(nil)
	p.cmd = &exec.Cmd{Process: &os.Process{}}
	done := make(chan struct{})
	close(done)
	p.done = done

	if p.IsRunning() {
		t.Fatal("a process whose done channel is closed must report not running")
	}
}

// TestIsRunning_LiveProcessWithOpenDone is the complementary case: an open
// done channel and no ProcessState means the process is alive, so IsRunning
// must report true (the select's default branch is taken).
func TestIsRunning_LiveProcessWithOpenDone(t *testing.T) {
	p := newProcess(nil)
	p.cmd = &exec.Cmd{Process: &os.Process{}}
	p.done = make(chan struct{}) // open

	if !p.IsRunning() {
		t.Fatal("a process with an open done channel and live cmd must report running")
	}
}

// TestGetResult pins process.go:260 — the
// `if len(lastLine) == 0 && exitErr != nil` choice between the captured log
// line and the exit error string.
func TestGetResult(t *testing.T) {
	cases := []struct {
		name     string
		lastLine string
		exitErr  error
		want     string
	}{
		{"no line, has error -> error string", "", errProcessTest("boom"), "boom"},
		{"has line -> line wins over error", "last log", errProcessTest("boom"), "last log"},
		{"no line, no error -> empty", "", nil, ""},
		{"has line, no error -> line", "last log", nil, "last log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProcess(nil)
			p.logWriter.lastLine = tc.lastLine
			p.exitErr = tc.exitErr
			if got := p.GetResult(); got != tc.want {
				t.Fatalf("GetResult() = %q, want %q", got, tc.want)
			}
		})
	}
}

type errProcessTest string

func (e errProcessTest) Error() string { return string(e) }

// TestRefreshLocalOnline_GraceBoundaryEmails pins the exact `<` boundary at
// process.go:407: an email idle for EXACTLY graceMs must be aged out (the
// window is half-open, age < grace). A mutated comparison (`<=`) would keep it.
func TestRefreshLocalOnline_GraceBoundaryEmails(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = int64(20000)

	p.RefreshLocalOnline([]string{"edge"}, nil, 0, grace)
	// now-ts == grace exactly: age is not strictly < grace, so it must drop.
	p.RefreshLocalOnline(nil, nil, grace, grace)
	for _, e := range p.GetLocalOnlineClients() {
		if e == "edge" {
			t.Fatalf("email idle exactly graceMs must age out (half-open window), got online %v", p.GetLocalOnlineClients())
		}
	}

	// One millisecond inside the window must still be online.
	p2 := newOnlineTestProcess()
	p2.RefreshLocalOnline([]string{"edge"}, nil, 0, grace)
	p2.RefreshLocalOnline(nil, nil, grace-1, grace)
	if !containsString(p2.GetLocalOnlineClients(), "edge") {
		t.Fatalf("email idle graceMs-1 must still be online, got %v", p2.GetLocalOnlineClients())
	}
}

// TestRefreshLocalOnline_GraceBoundaryInbounds pins the same `<` boundary at
// process.go:423 for inbound tags.
func TestRefreshLocalOnline_GraceBoundaryInbounds(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = int64(20000)

	p.RefreshLocalOnline(nil, []string{"in-edge"}, 0, grace)
	p.RefreshLocalOnline(nil, nil, grace, grace)
	for _, tag := range p.GetLocalActiveInbounds() {
		if tag == "in-edge" {
			t.Fatalf("inbound idle exactly graceMs must age out, got active %v", p.GetLocalActiveInbounds())
		}
	}

	p2 := newOnlineTestProcess()
	p2.RefreshLocalOnline(nil, []string{"in-edge"}, 0, grace)
	p2.RefreshLocalOnline(nil, nil, grace-1, grace)
	if !containsString(p2.GetLocalActiveInbounds(), "in-edge") {
		t.Fatalf("inbound idle graceMs-1 must still be active, got %v", p2.GetLocalActiveInbounds())
	}
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// process.go mutation audits (require a real child process; re-invoke the
// test binary so they run cross-platform, no signals needed)
// ---------------------------------------------------------------------------

// TestWaitForCommand_CrashExitRecordsError pins process.go:554 — the
// `if err == nil || intentionalStop` guard. A process that exits with a
// NON-zero code on its own (not an intentional Stop) is a crash and its error
// MUST be recorded. A mutated guard that negates the err check (`err != nil`)
// would early-return and drop the error.
func TestWaitForCommand_CrashExitRecordsError(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())
	cmd := exec.Command(os.Args[0], "-test.run=TestMutationAuditHelper", "--", "crash-exit")
	cmd.Env = append(os.Environ(), "XRAY_MUT_HELPER=1")

	p := newProcess(nil)
	if err := p.startCommand(cmd); err != nil {
		t.Fatalf("startCommand: %v", err)
	}
	// We never call Stop -> intentionalStop stays false; the child exits 2.
	if err := p.waitForExit(5 * time.Second); err != nil {
		t.Fatalf("child did not exit: %v", err)
	}
	if p.GetErr() == nil {
		t.Fatal("a non-intentional non-zero exit must record an error")
	}
}

// TestStop_RemovesTempConfigFile pins process.go:579 — the
// `if p.configPath != ""` guard that removes the per-run temp config file on
// Stop (so test runs never disturb the main config.json). A mutated guard
// (`== ""`) would skip the removal and leak the temp file.
func TestStop_RemovesTempConfigFile(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())

	tmpCfg := filepath.Join(t.TempDir(), "test-config.json")
	if err := os.WriteFile(tmpCfg, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMutationAuditHelper", "--", "block")
	cmd.Env = append(os.Environ(), "XRAY_MUT_HELPER=1")

	p := newProcess(nil)
	p.configPath = tmpCfg
	if err := p.startCommand(cmd); err != nil {
		t.Fatalf("startCommand: %v", err)
	}
	t.Cleanup(func() {
		if p.IsRunning() {
			p.intentionalStop.Store(true)
			_ = p.cmd.Process.Kill()
			_ = p.waitForExit(2 * time.Second)
		}
	})

	if !p.IsRunning() {
		t.Fatal("helper process must be running before Stop")
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := os.Stat(tmpCfg); !os.IsNotExist(err) {
		t.Fatalf("temp config file must be removed on Stop, stat err=%v", err)
	}
}

// TestMutationAuditHelper is the re-invoked child for the process tests above.
// It is inert unless XRAY_MUT_HELPER=1 is set.
func TestMutationAuditHelper(t *testing.T) {
	if os.Getenv("XRAY_MUT_HELPER") != "1" {
		return
	}
	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}
	switch mode {
	case "crash-exit":
		os.Exit(2)
	case "block":
		select {}
	}
}
