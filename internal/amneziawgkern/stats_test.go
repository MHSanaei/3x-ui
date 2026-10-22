package amneziawgkern

import (
	"testing"
	"time"
)

// realDump is a verbatim `awg show awg1 dump` body: interface line first, then
// one tab-separated line per peer, including a peer that has never connected.
const realDump = "cPriv=\tsMVZKrv7dcR79rp3z5rBzbiOawzyykZVr9meuqMbIDg=\t31884\toff\n" +
	"peerAPublicKeyBase64ValueForTestsPadding000=\t(none)\t92.36.61.153:51820\t10.8.1.2/32\t1788032481\t1048576\t4194304\t0\n" +
	"peerBPublicKeyBase64ValueForTestsPadding000=\t(none)\t(none)\t10.8.1.3/32\t0\t0\t0\t0\n"

func TestParseDumpSkipsInterfaceLine(t *testing.T) {
	t.Parallel()

	stats, err := ParseDump(realDump)
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d peers, want 2 (the interface line must not be counted)", len(stats))
	}
	a := stats[0]
	if a.PublicKey != "peerAPublicKeyBase64ValueForTestsPadding000=" {
		t.Errorf("public key = %q", a.PublicKey)
	}
	if a.Endpoint != "92.36.61.153:51820" {
		t.Errorf("endpoint = %q", a.Endpoint)
	}
	if a.LatestHandshake != 1788032481 {
		t.Errorf("handshake = %d", a.LatestHandshake)
	}
	if a.RxBytes != 1048576 || a.TxBytes != 4194304 {
		t.Errorf("rx/tx = %d/%d, want 1048576/4194304", a.RxBytes, a.TxBytes)
	}
	if stats[1].Endpoint != "(none)" || stats[1].LatestHandshake != 0 {
		t.Errorf("a never-connected peer must still be reported: %+v", stats[1])
	}
}

func TestParseDumpEmpty(t *testing.T) {
	t.Parallel()

	stats, err := ParseDump("")
	if err != nil {
		t.Fatalf("ParseDump(empty): %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("got %d peers from empty output", len(stats))
	}
}

// TestDeltaFirstPollCountsCurrentTotals: the interface was just created, so
// whatever the counters already hold is genuinely new traffic.
func TestDeltaFirstPollCountsCurrentTotals(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	got := c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100, TxBytes: 200}})
	if len(got) != 1 || got[0].Up != 100 || got[0].Down != 200 {
		t.Fatalf("first poll = %+v, want up=100 down=200", got)
	}
}

// TestDeltaReportsOnlyTheIncrement is the core of the accounting: reporting
// totals every poll would multiply every client's usage by the poll count.
func TestDeltaReportsOnlyTheIncrement(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100, TxBytes: 200}})
	got := c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 350, TxBytes: 700}})
	if len(got) != 1 || got[0].Up != 250 || got[0].Down != 500 {
		t.Fatalf("second poll = %+v, want up=250 down=500", got)
	}
}

// TestDeltaSkipsIdlePeers keeps the write path quiet: an idle inbound should
// not produce a database write every tick.
func TestDeltaSkipsIdlePeers(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100, TxBytes: 200}})
	if got := c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100, TxBytes: 200}}); len(got) != 0 {
		t.Fatalf("unchanged counters must produce no delta, got %+v", got)
	}
}

// TestDeltaSurvivesInterfaceRebuild is the reset case: awg-quick down/up zeroes the kernel counters,
// and a naive subtraction would go negative and either wrap or silently credit the client back their
func TestDeltaSurvivesInterfaceRebuild(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 5_000_000, TxBytes: 9_000_000}})
	got := c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 1_000, TxBytes: 2_000}})
	if len(got) != 1 {
		t.Fatalf("post-rebuild poll must report the new counters, got %+v", got)
	}
	if got[0].Up != 1_000 || got[0].Down != 2_000 {
		t.Errorf("post-rebuild delta = up %d down %d, want 1000/2000", got[0].Up, got[0].Down)
	}
}

func TestDeltaKeepsInterfacesSeparate(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100}})
	got := c.Delta("awg2", []PeerStat{{PublicKey: "p", RxBytes: 100}})
	if len(got) != 1 || got[0].Up != 100 {
		t.Errorf("the same peer key on another interface is a separate counter, got %+v", got)
	}
}

func TestForgetDropsInterfaceState(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100}})
	c.Forget("awg1")
	got := c.Delta("awg1", []PeerStat{{PublicKey: "p", RxBytes: 100}})
	if len(got) != 1 || got[0].Up != 100 {
		t.Errorf("after Forget the next poll starts fresh, got %+v", got)
	}
}

// TestEndpointRoamingIgnoresPortChanges: a peer behind NAT gets a new source port on every rekey, and
// counting those as new locations would ban every client with a limit of 1.
func TestEndpointRoamingIgnoresPortChanges(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	now := time.Unix(1_800_000_000, 0)
	window := time.Hour
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "203.0.113.7:51820", LatestHandshake: now.Unix()}}, now, window)
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "203.0.113.7:40001", LatestHandshake: now.Unix()}}, now.Add(time.Minute), window)

	if got := c.DistinctEndpoints("awg1", "p"); got != 1 {
		t.Errorf("same address on a new port = %d distinct endpoints, want 1", got)
	}
}

func TestEndpointRoamingCountsDistinctAddresses(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	now := time.Unix(1_800_000_000, 0)
	window := time.Hour
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "203.0.113.7:51820", LatestHandshake: now.Unix()}}, now, window)
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "198.51.100.4:51820", LatestHandshake: now.Add(time.Minute).Unix()}}, now.Add(time.Minute), window)

	if got := c.DistinctEndpoints("awg1", "p"); got != 2 {
		t.Errorf("two locations = %d distinct endpoints, want 2", got)
	}
}

// TestEndpointRoamingExpires: a client who moved home weeks ago must not stay
// counted forever, or the limit becomes a one-way ratchet.
func TestEndpointRoamingExpires(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	now := time.Unix(1_800_000_000, 0)
	window := 10 * time.Minute
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "203.0.113.7:51820", LatestHandshake: now.Unix()}}, now, window)
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "198.51.100.4:51820", LatestHandshake: now.Add(time.Hour).Unix()}}, now.Add(time.Hour), window)

	if got := c.DistinctEndpoints("awg1", "p"); got != 1 {
		t.Errorf("the stale sighting should have aged out, got %d", got)
	}
}

func TestEndpointRoamingExpiresIdleEndpointAndKeepsActiveInterfaces(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	start := time.Unix(1_800_000_000, 0)
	window := 30 * time.Minute
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "idle", Endpoint: "203.0.113.7:51820", LatestHandshake: start.Unix()}}, start, window)
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "roaming", Endpoint: "198.51.100.1:51820", LatestHandshake: start.Unix()}}, start, window)
	c.ObserveEndpoints("awg2", []PeerStat{{PublicKey: "other", Endpoint: "192.0.2.9:51820", LatestHandshake: start.Unix()}}, start, window)

	later := start.Add(31 * time.Minute)
	c.ObserveEndpoints("awg1", []PeerStat{
		{PublicKey: "idle", Endpoint: "203.0.113.7:51820", LatestHandshake: start.Unix()},
		{PublicKey: "roaming", Endpoint: "198.51.100.2:51820", LatestHandshake: later.Unix()},
	}, later, window)
	c.ObserveEndpoints("awg2", []PeerStat{{PublicKey: "other", Endpoint: "192.0.2.9:51820", LatestHandshake: later.Unix()}}, later, window)

	if got := c.EndpointsFor("awg1", "idle"); len(got) != 0 {
		t.Fatalf("idle peer endpoint remained visible after its handshake aged out: %v", got)
	}
	if got := c.EndpointsFor("awg1", "roaming"); len(got) != 1 || got[0] != "198.51.100.2" {
		t.Fatalf("active roaming peer endpoints = %v, want only current address", got)
	}
	if got := c.EndpointsFor("awg2", "other"); len(got) != 1 || got[0] != "192.0.2.9" {
		t.Fatalf("active peer on another interface was lost: %v", got)
	}
}

// A peer that never completed a handshake has no observed location.
func TestEndpointRoamingSkipsNeverConnected(t *testing.T) {
	t.Parallel()

	c := NewCollector()
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "p", Endpoint: "(none)"}}, time.Now(), time.Hour)
	if got := c.DistinctEndpoints("awg1", "p"); got != 0 {
		t.Errorf("a peer that never connected has %d endpoints, want 0", got)
	}
}

func TestEndpointHostHandlesIPv6(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"203.0.113.7:51820":    "203.0.113.7",
		"[2a12:bec4::1]:51820": "[2a12:bec4::1]",
		"(none)":               "",
		"":                     "",
		"  198.51.100.4:1  ":   "198.51.100.4",
	}
	for in, want := range cases {
		if got := endpointHost(in); got != want {
			t.Errorf("endpointHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestOnlineEmailsUsesHandshakeFreshness: kernel-mode peers are invisible to Xray's online-users API,
// so a stale handshake is the only thing separating a connected client from one whose laptop is shut.
func TestOnlineEmailsUsesHandshakeFreshness(t *testing.T) {
	t.Parallel()

	now := time.Now()
	c := NewCollector()
	c.SetEmails("awg1", map[string]string{"fresh": "alice", "stale": "bob"})
	c.ObserveHandshakes("awg1", []PeerStat{
		{PublicKey: "fresh", LatestHandshake: now.Add(-30 * time.Second).Unix()},
		{PublicKey: "stale", LatestHandshake: now.Add(-2 * time.Hour).Unix()},
	})

	got := c.OnlineEmails(now)
	if len(got) != 1 || got[0] != "alice" {
		t.Fatalf("OnlineEmails = %v, want [alice]", got)
	}
}

// TestOnlineEmailsIgnoresNeverConnected: handshake 0 is "never", not "now".
func TestOnlineEmailsIgnoresNeverConnected(t *testing.T) {
	t.Parallel()

	now := time.Now()
	c := NewCollector()
	c.SetEmails("awg1", map[string]string{"p": "alice"})
	c.ObserveHandshakes("awg1", []PeerStat{{PublicKey: "p", LatestHandshake: 0}})

	if got := c.OnlineEmails(now); len(got) != 0 {
		t.Errorf("OnlineEmails = %v, want empty", got)
	}
}

// TestOnlineEmailsDedupesPeersOfOneClient: a client with two devices on the
// same email must be counted once, matching how the Xray side reports.
func TestOnlineEmailsDedupesPeersOfOneClient(t *testing.T) {
	t.Parallel()

	now := time.Now()
	c := NewCollector()
	c.SetEmails("awg1", map[string]string{"a": "alice", "b": "alice"})
	c.ObserveHandshakes("awg1", []PeerStat{
		{PublicKey: "a", LatestHandshake: now.Unix()},
		{PublicKey: "b", LatestHandshake: now.Unix()},
	})

	if got := c.OnlineEmails(now); len(got) != 1 {
		t.Errorf("OnlineEmails = %v, want one entry", got)
	}
}

func TestSetEmailsPrunesRemovedPeerStateWithoutAffectingOtherInterfaces(t *testing.T) {
	c := NewCollector()
	now := time.Unix(1_800_000_000, 0)
	c.Delta("awg1", []PeerStat{{PublicKey: "removed", RxBytes: 10, TxBytes: 20}})
	c.ObserveEndpoints("awg1", []PeerStat{{PublicKey: "removed", Endpoint: "203.0.113.4:1234", LatestHandshake: now.Unix()}}, now, time.Hour)
	c.ObserveHandshakes("awg1", []PeerStat{{PublicKey: "removed", LatestHandshake: now.Unix()}})
	c.SetEmails("awg1", map[string]string{"removed": "old@example.test"})

	c.Delta("awg2", []PeerStat{{PublicKey: "kept", RxBytes: 30, TxBytes: 40}})
	c.ObserveEndpoints("awg2", []PeerStat{{PublicKey: "kept", Endpoint: "203.0.113.5:1234", LatestHandshake: now.Unix()}}, now, time.Hour)
	c.ObserveHandshakes("awg2", []PeerStat{{PublicKey: "kept", LatestHandshake: now.Unix()}})
	c.SetEmails("awg2", map[string]string{"kept": "kept@example.test"})

	c.SetEmails("awg1", map[string]string{})
	if got := c.ClientIPs(); len(got) != 1 || len(got["old@example.test"]) != 0 {
		t.Fatalf("removed peer endpoint remained visible: %+v", got)
	}
	if got := c.OnlineEmails(now); len(got) != 1 || got[0] != "kept@example.test" {
		t.Fatalf("removed peer remained online or other interface was lost: %v", got)
	}
	if got := c.DistinctEndpoints("awg1", "removed"); got != 0 {
		t.Fatalf("removed peer endpoint count = %d, want 0", got)
	}
	if got := c.Delta("awg1", []PeerStat{{PublicKey: "removed", RxBytes: 15, TxBytes: 25}}); len(got) != 1 || got[0].Up != 15 || got[0].Down != 25 {
		t.Fatalf("re-added peer did not start a fresh counter baseline: %+v", got)
	}
	if got := c.DistinctEndpoints("awg2", "kept"); got != 1 {
		t.Fatalf("other interface endpoint count = %d, want 1", got)
	}
}
