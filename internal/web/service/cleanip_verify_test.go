package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubCleanIpProbe swaps the network probe for fn and starts every test with an empty budget.
func stubCleanIpProbe(t *testing.T, fn func(ctx context.Context, host, ip string) string) {
	t.Helper()
	prevProbe, prevNow := cleanIpProbe, cleanIpNow
	cleanIpProbe = fn
	resetCleanIpVerifyBudget()
	t.Cleanup(func() {
		cleanIpProbe, cleanIpNow = prevProbe, prevNow
		resetCleanIpVerifyBudget()
	})
}

func cloudflareIps(from, n int) []string {
	ips := make([]string, n)
	for i := range ips {
		ips[i] = "104.16." + strconv.Itoa((from+i)/250) + "." + strconv.Itoa((from+i)%250+1)
	}
	return ips
}

// The server must never connect to an address the page could feed it beyond the CDN's own
// edges: private ranges (SSRF into the panel's network) or arbitrary hosts (port scanning).
func TestCleanIpVerifyOnlyProbesTheCdnsPublishedRanges(t *testing.T) {
	var probed []string
	stubCleanIpProbe(t, func(_ context.Context, _, ip string) string {
		probed = append(probed, ip)
		return cleanIpOk
	})
	var s CleanIpService
	got, err := s.VerifyIps(context.Background(), CleanIpVerifyRequest{
		Host: "cdn.example.com", Cdn: "cloudflare",
		Ips: []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "8.8.8.8", "151.101.1.1", "104.16.1.1"},
	})
	if err != nil {
		t.Fatalf("VerifyIps: %v", err)
	}
	want := map[string]string{
		"127.0.0.1": cleanIpSkipped, "10.0.0.1": cleanIpSkipped, "192.168.1.1": cleanIpSkipped,
		"169.254.169.254": cleanIpSkipped, "8.8.8.8": cleanIpSkipped, "151.101.1.1": cleanIpSkipped,
		"104.16.1.1": cleanIpOk,
	}
	for _, v := range got {
		if want[v.Ip] != v.Status {
			t.Errorf("%s: status %q, want %q", v.Ip, v.Status, want[v.Ip])
		}
	}
	if len(got) != len(want) || len(probed) != 1 || probed[0] != "104.16.1.1" {
		t.Fatalf("verdicts %+v, probed %v; want only 104.16.1.1 probed", got, probed)
	}
	if _, err := s.VerifyIps(context.Background(), CleanIpVerifyRequest{Host: "cdn.example.com", Cdn: "akamai", Ips: []string{"104.16.1.1"}}); !errors.Is(err, errCleanIpBadRequest) {
		t.Errorf("unknown CDN: err = %v, want errCleanIpBadRequest", err)
	}
}

func TestCleanIpVerifyRunsAtMostFourProbesAtOnce(t *testing.T) {
	var inFlight, peak atomic.Int32
	stubCleanIpProbe(t, func(context.Context, string, string) string {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return cleanIpOk
	})
	var s CleanIpService
	got, err := s.VerifyIps(context.Background(), CleanIpVerifyRequest{Host: "cdn.example.com", Ips: cloudflareIps(0, 80)})
	if err != nil {
		t.Fatalf("VerifyIps: %v", err)
	}
	if len(got) != 50 || peak.Load() != 4 {
		t.Fatalf("%d verdicts with %d probes at once; want %d (the per-request cap) with %d at once",
			len(got), peak.Load(), 50, 4)
	}
}

// Repeated requests must not add up to a sweep: past the hourly budget nothing is probed,
// and an IP already checked this hour is answered from memory.
func TestCleanIpVerifyHourlyBudgetSpansRequests(t *testing.T) {
	var probes atomic.Int32
	stubCleanIpProbe(t, func(context.Context, string, string) string {
		probes.Add(1)
		return cleanIpOk
	})
	start := time.Now()
	cleanIpNow = func() time.Time { return start }
	var s CleanIpService
	verify := func(ips []string) []CleanIpVerdict {
		t.Helper()
		got, err := s.VerifyIps(context.Background(), CleanIpVerifyRequest{Host: "cdn.example.com", Ips: ips})
		if err != nil {
			t.Fatalf("VerifyIps: %v", err)
		}
		return got
	}
	for i := 0; i < cleanIpVerifyHourly/cleanIpVerifyMax; i++ {
		verify(cloudflareIps(i*cleanIpVerifyMax, cleanIpVerifyMax))
	}
	if probes.Load() != cleanIpVerifyHourly {
		t.Fatalf("probed %d IPs filling the budget, want %d", probes.Load(), cleanIpVerifyHourly)
	}
	for _, v := range verify(cloudflareIps(cleanIpVerifyHourly, 3)) {
		if v.Status != cleanIpLimited {
			t.Errorf("%s past the budget: %q, want %q", v.Ip, v.Status, cleanIpLimited)
		}
	}
	if again := verify(cloudflareIps(0, 2)); again[0].Status != cleanIpOk || probes.Load() != cleanIpVerifyHourly {
		t.Errorf("re-checking a known IP: %+v after %d probes; want the remembered ok, no new probe", again, probes.Load())
	}

	cleanIpNow = func() time.Time { return start.Add(time.Hour + time.Minute) }
	if v := verify(cloudflareIps(cleanIpVerifyHourly, 1)); v[0].Status != cleanIpOk {
		t.Errorf("an hour later: %q, want the budget back", v[0].Status)
	}
}

// Closing the page (or its cancel button) aborts the request; the server stops probing too.
func TestCleanIpVerifyStopsWhenTheCallerGoesAway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var probes atomic.Int32
	var once sync.Once
	stubCleanIpProbe(t, func(ctx context.Context, _, _ string) string {
		probes.Add(1)
		once.Do(cancel)
		<-ctx.Done()
		return cleanIpUnreachable
	})
	var s CleanIpService
	got, err := s.VerifyIps(ctx, CleanIpVerifyRequest{Host: "cdn.example.com", Ips: cloudflareIps(0, cleanIpVerifyMax)})
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("VerifyIps after cancel = %d verdicts, %v; want nil, context.Canceled", len(got), err)
	}
	if n := probes.Load(); n > cleanIpVerifyWorkers {
		t.Fatalf("%d probes ran after the caller left; want at most the %d already started", n, cleanIpVerifyWorkers)
	}
}
