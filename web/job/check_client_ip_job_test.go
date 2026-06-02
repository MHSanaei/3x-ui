package job

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestMergeClientIps_EvictsStaleOldEntries(t *testing.T) {
	// #4077: after a ban expires, a single IP that reconnects used to get
	// banned again immediately because a long-disconnected IP stayed in the
	// DB with an ancient timestamp and kept "protecting" itself against
	// eviction. Guard against that regression here.
	old := []IPWithTimestamp{
		{IP: "1.1.1.1", Timestamp: 100},  // stale — client disconnected long ago
		{IP: "2.2.2.2", Timestamp: 1900}, // fresh — still connecting
	}
	new := []IPWithTimestamp{
		{IP: "2.2.2.2", Timestamp: 2000}, // same IP, newer log line
	}

	got := mergeClientIps(old, new, 1000)

	want := map[string]int64{"2.2.2.2": 2000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stale 1.1.1.1 should have been dropped\ngot:  %v\nwant: %v", got, want)
	}
}

func TestMergeClientIps_KeepsFreshOldEntriesUnchanged(t *testing.T) {
	// Backwards-compat: entries that aren't stale are still carried forward,
	// so enforcement survives access-log rotation.
	old := []IPWithTimestamp{
		{IP: "1.1.1.1", Timestamp: 1500},
	}
	got := mergeClientIps(old, nil, 1000)

	want := map[string]int64{"1.1.1.1": 1500}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fresh old IP should have been retained\ngot:  %v\nwant: %v", got, want)
	}
}

func TestMergeClientIps_PrefersLaterTimestampForSameIp(t *testing.T) {
	old := []IPWithTimestamp{{IP: "1.1.1.1", Timestamp: 1500}}
	new := []IPWithTimestamp{{IP: "1.1.1.1", Timestamp: 1700}}

	got := mergeClientIps(old, new, 1000)

	if got["1.1.1.1"] != 1700 {
		t.Fatalf("expected latest timestamp 1700, got %d", got["1.1.1.1"])
	}
}

func TestMergeClientIps_DropsStaleNewEntries(t *testing.T) {
	// A log line with a clock-skewed old timestamp must not resurrect a
	// stale IP past the cutoff.
	new := []IPWithTimestamp{{IP: "1.1.1.1", Timestamp: 500}}
	got := mergeClientIps(nil, new, 1000)

	if len(got) != 0 {
		t.Fatalf("stale new IP should have been dropped, got %v", got)
	}
}

func TestMergeClientIps_NoStaleCutoffStillWorks(t *testing.T) {
	// Defensive: a zero cutoff (e.g. during very first run on a fresh
	// install) must not over-evict.
	old := []IPWithTimestamp{{IP: "1.1.1.1", Timestamp: 100}}
	new := []IPWithTimestamp{{IP: "2.2.2.2", Timestamp: 200}}

	got := mergeClientIps(old, new, 0)

	want := map[string]int64{"1.1.1.1": 100, "2.2.2.2": 200}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("zero cutoff should keep everything\ngot:  %v\nwant: %v", got, want)
	}
}

func collectIps(entries []IPWithTimestamp) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.IP)
	}
	return out
}

func TestPartitionLiveIps_SingleLiveNotStarvedByStillFreshHistoricals(t *testing.T) {
	// #4091: db holds A, B, C from minutes ago (still in the 30min
	// window) but they're not connecting anymore. only D is. old code
	// merged all four, sorted ascending, kept [A,B,C] and banned D
	// every tick. pin the new rule: only live ips count toward the limit.
	ipMap := map[string]int64{
		"A": 1000,
		"B": 1100,
		"C": 1200,
		"D": 2000,
	}
	observed := map[string]bool{"D": true}

	live, historical := partitionLiveIps(ipMap, observed)

	if got := collectIps(live); !reflect.DeepEqual(got, []string{"D"}) {
		t.Fatalf("live set should only contain the ip observed this scan\ngot:  %v\nwant: [D]", got)
	}
	if got := collectIps(historical); !reflect.DeepEqual(got, []string{"A", "B", "C"}) {
		t.Fatalf("historical set should contain db-only ips in ascending order\ngot:  %v\nwant: [A B C]", got)
	}
}

func TestPartitionLiveIps_ConcurrentLiveIpsSortedAscending(t *testing.T) {
	// when several ips are really live, partition returns them all in the
	// live set sorted ascending by timestamp. updateInboundClientIps then
	// keeps the newest and bans the oldest (last-IP-wins, #4699).
	ipMap := map[string]int64{
		"A": 5000,
		"B": 5500,
	}
	observed := map[string]bool{"A": true, "B": true}

	live, historical := partitionLiveIps(ipMap, observed)

	if got := collectIps(live); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("both live ips should be in the live set, ascending\ngot:  %v\nwant: [A B]", got)
	}
	if len(historical) != 0 {
		t.Fatalf("no historical ips expected, got %v", historical)
	}
}

func TestPartitionLiveIps_EmptyScanLeavesDbIntact(t *testing.T) {
	// quiet tick: nothing observed => nothing live. everything merged
	// is historical. keeps the panel from wiping recent-but-idle ips.
	ipMap := map[string]int64{
		"A": 1000,
		"B": 1100,
	}
	observed := map[string]bool{}

	live, historical := partitionLiveIps(ipMap, observed)

	if len(live) != 0 {
		t.Fatalf("no live ips expected, got %v", live)
	}
	if got := collectIps(historical); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("all merged entries should flow to historical\ngot:  %v\nwant: [A B]", got)
	}
}

func TestCheckFail2BanInstalled_DisabledEnvSkipsClientProbe(t *testing.T) {
	t.Setenv("XUI_ENABLE_FAIL2BAN", "false")
	marker := fakeFail2BanClient(t)

	if (&CheckClientIpJob{}).checkFail2BanInstalled() {
		t.Fatal("fail2ban should be unavailable when XUI_ENABLE_FAIL2BAN=false")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("fail2ban-client should not have been executed, stat error: %v", err)
	}
}

func TestCheckFail2BanInstalled_EmptyEnvSkipsClientProbe(t *testing.T) {
	t.Setenv("XUI_ENABLE_FAIL2BAN", "")
	marker := fakeFail2BanClient(t)

	if (&CheckClientIpJob{}).checkFail2BanInstalled() {
		t.Fatal("fail2ban should be unavailable when XUI_ENABLE_FAIL2BAN is empty")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("fail2ban-client should not have been executed, stat error: %v", err)
	}
}

func TestIsFail2BanEnabled_DefaultsToEnabledWhenUnset(t *testing.T) {
	value, ok := os.LookupEnv("XUI_ENABLE_FAIL2BAN")
	os.Unsetenv("XUI_ENABLE_FAIL2BAN")
	t.Cleanup(func() {
		if ok {
			os.Setenv("XUI_ENABLE_FAIL2BAN", value)
		} else {
			os.Unsetenv("XUI_ENABLE_FAIL2BAN")
		}
	})

	if !isFail2BanEnabled() {
		t.Fatal("fail2ban should default to enabled when XUI_ENABLE_FAIL2BAN is unset")
	}
}

func TestCheckFail2BanInstalled_EnabledEnvProbesClient(t *testing.T) {
	t.Setenv("XUI_ENABLE_FAIL2BAN", "true")
	marker := fakeFail2BanClient(t)

	if !(&CheckClientIpJob{}).checkFail2BanInstalled() {
		t.Fatal("fail2ban should be available when the client probe succeeds")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("fail2ban-client should have been executed: %v", err)
	}
}

func fakeFail2BanClient(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	marker := filepath.Join(dir, "probe-called")
	fakeClient := filepath.Join(dir, "fail2ban-client")
	script := "#!/bin/sh\n: > \"$FAIL2BAN_PROBE_MARKER\"\nexit 0\n"
	if runtime.GOOS == "windows" {
		fakeClient += ".bat"
		script = "@echo off\ntype nul > \"%FAIL2BAN_PROBE_MARKER%\"\nexit /b 0\n"
	}
	if err := os.WriteFile(fakeClient, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake fail2ban-client: %v", err)
	}

	t.Setenv("FAIL2BAN_PROBE_MARKER", marker)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}
