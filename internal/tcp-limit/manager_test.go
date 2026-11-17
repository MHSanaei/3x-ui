package tcplimit

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestManagerAtomicRulesAndNoOp(t *testing.T) {
	var scripts []string
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		if stdin == "" {
			return nil, errors.New("No such file or directory")
		}
		scripts = append(scripts, stdin)
		return nil, nil
	}}
	if err := m.Apply(200, portTargets(8443, 443, 443)); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"add table inet xui_tcp_limit", "delete table inet xui_tcp_limit", "tcp dport 443", "ip saddr ct count over 200", "ip6 saddr ct count over 200", "priority -20", `iifname "lo" return`} {
		if !strings.Contains(scripts[0], part) {
			t.Errorf("rules lack %q", part)
		}
	}
	if strings.Contains(scripts[0], "flush ruleset") {
		t.Fatal("must not change other nftables tables")
	}
	if err := m.Apply(200, portTargets(443, 8443)); err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 1 {
		t.Fatal("no-op replaced connection counters")
	}
	if err := m.Apply(0, nil); err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 2 || strings.Contains(scripts[1], "ct count") {
		t.Fatal("disable must remove only the managed table")
	}
}

func TestManagerPreservesExistingCountersAfterRestart(t *testing.T) {
	calls := 0
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		calls++
		if stdin != "" {
			t.Fatal("identical existing table must not be replaced")
		}
		return []byte(`table inet xui_tcp_limit {
 comment "3x-ui TCP limit 200 targets *:443"
}`), nil
	}}
	if err := m.Apply(200, portTargets(443)); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestManagerFailurePreservesAppliedRulesAndRetries(t *testing.T) {
	fail := false
	applies := 0
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		if stdin == "" {
			return nil, errors.New("not found")
		}
		applies++
		if fail {
			return []byte("permission denied"), errors.New("nft failed")
		}
		return nil, nil
	}}
	if err := m.Apply(200, portTargets(443)); err != nil {
		t.Fatal(err)
	}
	fail = true
	if err := m.Apply(100, portTargets(443)); err == nil {
		t.Fatal("apply failure was hidden")
	}
	if !strings.Contains(m.current, "limit 200") {
		t.Fatal("failed transaction changed cached state")
	}
	fail = false
	if err := m.Apply(100, portTargets(443)); err != nil {
		t.Fatal(err)
	}
	if applies != 3 {
		t.Fatalf("applies = %d", applies)
	}
}

func TestManagerDisabledWithoutNftAndInvalidLimit(t *testing.T) {
	m := &Manager{run: func(string, ...string) ([]byte, error) { return nil, errors.New("nft missing") }}
	if err := m.Apply(0, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Apply(200, portTargets(443)); err == nil {
		t.Fatal("enabled limit requires nft")
	}
	for _, limit := range []int{-1, 1_000_001} {
		if err := m.Apply(limit, portTargets(443)); err == nil {
			t.Fatalf("accepted limit %d", limit)
		}
	}
	if err := m.Apply(200, portTargets(0)); err == nil {
		t.Fatal("accepted port zero")
	}
}

func TestManagerConcurrentNoOp(t *testing.T) {
	applies := 0
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		if stdin != "" {
			applies++
		}
		return nil, nil
	}}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if err := m.Apply(200, portTargets(443)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if applies != 1 {
		t.Fatalf("applies = %d, want 1", applies)
	}
}

func TestManagerDisablesStaleTable(t *testing.T) {
	removed := false
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		if stdin == "" {
			return []byte(`table inet xui_tcp_limit {
 comment "3x-ui TCP limit 200 targets *:443"
}`), nil
		}
		removed = strings.Contains(stdin, "delete table inet xui_tcp_limit")
		return nil, nil
	}}
	if err := m.Apply(0, nil); err != nil || !removed {
		t.Fatalf("remove stale table = %v, error %v", removed, err)
	}
}

func TestManagerStageRestoresPreviousRules(t *testing.T) {
	var scripts []string
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		if stdin == "" {
			return []byte("table inet xui_tcp_limit {\n comment \"3x-ui TCP limit 200 targets *:443, *:8443\"\n}"), nil
		}
		scripts = append(scripts, stdin)
		return nil, nil
	}}
	rollback, err := m.Stage(100, portTargets(9443))
	if err != nil {
		t.Fatal(err)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if len(scripts) != 2 || !strings.Contains(scripts[1], "limit 200 targets *:443, *:8443") {
		t.Fatalf("rollback scripts = %v", scripts)
	}
}

func TestManagerStageFailureAndInitiallyDisabledRollback(t *testing.T) {
	for _, failure := range []bool{false, true} {
		m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
			if stdin == "" || failure {
				return nil, errors.New("not found")
			}
			return nil, nil
		}}
		rollback, err := m.Stage(100, portTargets(443))
		if failure {
			if err == nil || rollback != nil {
				t.Fatal("failed stage returned a rollback or no error")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := rollback(); err != nil || m.current != "" {
			t.Fatalf("initially disabled rollback: %q %v", m.current, err)
		}
	}
}

func TestManagerStrictDisableRequiresNft(t *testing.T) {
	m := &Manager{run: func(string, ...string) ([]byte, error) {
		return nil, errors.New("permission denied")
	}}
	if err := m.Disable(); err == nil {
		t.Fatal("explicit disable concealed failure to remove existing rules")
	}
}

func TestManagerPreservesDestinationAddress(t *testing.T) {
	var script string
	m := &Manager{run: func(stdin string, args ...string) ([]byte, error) {
		if stdin != "" {
			script = stdin
		}
		return nil, nil
	}}
	targets := []Target{
		{Address: netip.MustParseAddr("192.0.2.1"), Port: 443},
		{Address: netip.MustParseAddr("2001:db8::1"), Port: 8443},
	}
	if err := m.Apply(200, targets); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ip daddr 192.0.2.1 tcp dport 443", "ip6 daddr 2001:db8::1 tcp dport 8443"} {
		if !strings.Contains(script, want) {
			t.Errorf("rules lack %q", want)
		}
	}
	if strings.Contains(script, "ip6 saddr ct count over 200 } counter reject") && strings.Contains(script, "  tcp dport") {
		t.Fatal("specific listener introduced a wildcard destination")
	}
	limit, got, err := parseSignature(m.current)
	if err != nil || limit != 200 || !slices.Equal(got, targets) {
		t.Fatalf("parsed signature = %d %v %v", limit, got, err)
	}
}

func TestManagerIPv6OnlyWildcard(t *testing.T) {
	script := rules(200, []Target{{Address: netip.IPv6Unspecified(), Port: 443}}, "test")
	if strings.Contains(script, "{ ip saddr ct count") || strings.Contains(script, "ip6 daddr :: ") {
		t.Fatal("IPv6-only wildcard must not cover IPv4 or match destination ::")
	}
	if !strings.Contains(script, "tcp dport 443") || !strings.Contains(script, "{ ip6 saddr ct count over 200 }") {
		t.Fatal("IPv6-only listener lacks protection")
	}
}

func portTargets(ports ...uint16) []Target {
	targets := make([]Target, len(ports))
	for i, port := range ports {
		targets[i] = Target{Port: port}
	}
	return targets
}
