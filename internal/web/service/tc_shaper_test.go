package service

import (
	"runtime"
	"strings"
	"testing"
)

func TestUniqueValidIPs(t *testing.T) {
	t.Parallel()
	got := uniqueValidIPs([]string{" 1.2.3.4 ", "1.2.3.4", "bad", "2001:db8::1", ""})
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %#v", len(got), got)
	}
	if got[0] != "1.2.3.4" || got[1] != "2001:db8::1" {
		t.Fatalf("unexpected ips: %#v", got)
	}
}

func TestIPMatch(t *testing.T) {
	t.Parallel()
	proto, family, dir, prefix := ipMatch("10.0.0.1", "dst")
	if proto != "ip" || family != "ip" || dir != "dst" || prefix != "/32" {
		t.Fatalf("ipv4: %q %q %q %q", proto, family, dir, prefix)
	}
	proto, family, dir, prefix = ipMatch("2001:db8::1", "src")
	if proto != "ipv6" || family != "ip6" || dir != "src" || prefix != "/128" {
		t.Fatalf("ipv6: %q %q %q %q", proto, family, dir, prefix)
	}
	proto, _, _, _ = ipMatch("not-an-ip", "dst")
	if proto != "" {
		t.Fatalf("invalid ip should be empty, got %q", proto)
	}
}

func TestTcShaperSyncDiff(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tc shaper is a no-op on windows")
	}
	t.Parallel()

	var calls [][]string
	s := NewTcShaper("eth0")
	s.runner = func(args ...string) error {
		cp := append([]string(nil), args...)
		calls = append(calls, cp)
		return nil
	}
	s.ready = true
	s.ownIngress = true

	s.Sync(map[string]ClientSpeed{
		"a@test": {IPs: []string{"1.1.1.1", "2.2.2.2"}, DownMbps: 10, UpMbps: 5},
	})
	if len(s.applied) != 1 {
		t.Fatalf("applied=%d want 1", len(s.applied))
	}
	st := s.applied["a@test"]
	if st.classID == 0 || st.policeIdx == 0 {
		t.Fatalf("missing class/police: %+v", st)
	}
	if len(st.downH) != 2 || len(st.upH) != 2 {
		t.Fatalf("filters down=%d up=%d want 2/2", len(st.downH), len(st.upH))
	}

	joined := joinCalls(calls)
	if !strings.Contains(joined, "actions add action police") {
		t.Fatalf("expected shared police action create, calls:\n%s", joined)
	}
	policeAdds := 0
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "actions" && c[1] == "add" {
			policeAdds++
		}
	}
	if policeAdds != 1 {
		t.Fatalf("police action adds=%d want 1 (shared across IPs)", policeAdds)
	}

	calls = nil
	s.Sync(map[string]ClientSpeed{
		"a@test": {IPs: []string{"1.1.1.1"}, DownMbps: 10, UpMbps: 5},
	})
	if len(st.downH) != 1 || len(st.upH) != 1 {
		t.Fatalf("after IP shrink down=%d up=%d want 1/1", len(st.downH), len(st.upH))
	}
	if _, ok := st.downH["1.1.1.1"]; !ok {
		t.Fatalf("expected remaining down filter for 1.1.1.1")
	}

	calls = nil
	s.Sync(map[string]ClientSpeed{})
	if len(s.applied) != 0 {
		t.Fatalf("applied not cleared: %#v", s.applied)
	}
	joined = joinCalls(calls)
	if !strings.Contains(joined, "actions delete action police") {
		t.Fatalf("expected police delete on remove, calls:\n%s", joined)
	}
}

func TestTcShaperSharedUploadAcrossIPs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tc shaper is a no-op on windows")
	}
	t.Parallel()

	var calls [][]string
	s := NewTcShaper("eth0")
	s.runner = func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		return nil
	}
	s.ready = true
	s.ownIngress = true

	s.Sync(map[string]ClientSpeed{
		"multi@test": {IPs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, DownMbps: 0, UpMbps: 8},
	})
	st := s.applied["multi@test"]
	if st == nil || st.policeIdx == 0 {
		t.Fatalf("missing police state: %#v", st)
	}
	if len(st.upH) != 3 {
		t.Fatalf("up filters=%d want 3", len(st.upH))
	}

	idx := ""
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "actions" && c[1] == "add" {
			for i := 0; i < len(c)-1; i++ {
				if c[i] == "index" {
					idx = c[i+1]
				}
			}
		}
	}
	if idx == "" {
		t.Fatal("police index not found in actions add")
	}
	refs := 0
	for _, c := range calls {
		if len(c) < 4 || c[0] != "filter" {
			continue
		}
		for i := 0; i < len(c)-1; i++ {
			if c[i] == "index" && c[i+1] == idx {
				refs++
			}
		}
	}
	if refs != 3 {
		t.Fatalf("filters referencing shared police index %s: %d want 3", idx, refs)
	}
}

func joinCalls(calls [][]string) string {
	parts := make([]string, 0, len(calls))
	for _, c := range calls {
		parts = append(parts, strings.Join(c, " "))
	}
	return strings.Join(parts, "\n")
}
