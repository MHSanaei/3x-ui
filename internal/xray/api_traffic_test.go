package xray

import (
	"context"
	"net"
	"testing"

	statsService "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
)

// fakeStatsServer serves a scripted sequence of QueryStats responses, one per
// call, so the delta bookkeeping in GetTraffic can be driven exactly.
type fakeStatsServer struct {
	statsService.UnimplementedStatsServiceServer
	rounds [][]*statsService.Stat
	calls  int
}

func (f *fakeStatsServer) QueryStats(context.Context, *statsService.QueryStatsRequest) (*statsService.QueryStatsResponse, error) {
	round := f.calls
	f.calls++
	if round >= len(f.rounds) {
		round = len(f.rounds) - 1
	}
	return &statsService.QueryStatsResponse{Stat: f.rounds[round]}, nil
}

func stat(name string, value int64) *statsService.Stat {
	return &statsService.Stat{Name: name, Value: value}
}

// startFakeStats runs the scripted stats service and returns an XrayAPI wired
// to it.
func startFakeStats(t *testing.T, rounds [][]*statsService.Stat) *XrayAPI {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	statsService.RegisterStatsServiceServer(srv, &fakeStatsServer{rounds: rounds})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	api := &XrayAPI{}
	if err := api.Init(lis.Addr().(*net.TCPAddr).Port); err != nil {
		t.Fatalf("api init: %v", err)
	}
	t.Cleanup(api.Close)
	return api
}

func clientTrafficByEmail(t *testing.T, traffics []*ClientTraffic) map[string]*ClientTraffic {
	t.Helper()
	byEmail := make(map[string]*ClientTraffic, len(traffics))
	for _, ct := range traffics {
		byEmail[ct.Email] = ct
	}
	return byEmail
}

// TestGetTrafficFirstPollIsBaselineOnly pins the one case the skip protects:
// the panel may attach to counters that already hold traffic it cannot
// attribute, so the first poll of a client only records baselines.
func TestGetTrafficFirstPollIsBaselineOnly(t *testing.T) {
	api := startFakeStats(t, [][]*statsService.Stat{
		{stat("user>>>alice>>>traffic>>>uplink", 5000)},
	})

	_, clients, err := api.GetTraffic()
	if err != nil {
		t.Fatalf("GetTraffic: %v", err)
	}
	if len(clients) != 0 {
		t.Fatalf("first poll reported %+v, want no traffic (baseline only)", clients[0])
	}
	if got := api.StatsLastValues["user>>>alice>>>traffic>>>uplink"]; got != 5000 {
		t.Fatalf("baseline = %d, want 5000", got)
	}
}

// TestGetTrafficCountsNewStatFromZero is the regression for a new client's
// traffic being dropped: xray creates a counter on first use, so a name that
// appears after the baseline poll starts at zero and every byte in it is new.
func TestGetTrafficCountsNewStatFromZero(t *testing.T) {
	api := startFakeStats(t, [][]*statsService.Stat{
		{stat("user>>>alice>>>traffic>>>uplink", 100)},
		{
			stat("user>>>alice>>>traffic>>>uplink", 180),
			stat("user>>>bob>>>traffic>>>uplink", 4096),
			stat("user>>>bob>>>traffic>>>downlink", 8192),
		},
	})

	if _, _, err := api.GetTraffic(); err != nil {
		t.Fatalf("GetTraffic (baseline): %v", err)
	}
	_, clients, err := api.GetTraffic()
	if err != nil {
		t.Fatalf("GetTraffic: %v", err)
	}

	byEmail := clientTrafficByEmail(t, clients)
	bob, ok := byEmail["bob"]
	if !ok {
		t.Fatal("a client whose counter appeared after the baseline poll reported no traffic")
	}
	if bob.Up != 4096 || bob.Down != 8192 {
		t.Fatalf("bob = up %d / down %d, want 4096 / 8192", bob.Up, bob.Down)
	}
	alice, ok := byEmail["alice"]
	if !ok {
		t.Fatal("alice reported no traffic")
	}
	if alice.Up != 80 {
		t.Fatalf("alice up = %d, want the delta 80", alice.Up)
	}
}

// TestGetTrafficCountsAfterCounterReset covers a core restart: the counters
// start over at zero, so a value below the recorded baseline is all new
// traffic rather than something to drop.
func TestGetTrafficCountsAfterCounterReset(t *testing.T) {
	api := startFakeStats(t, [][]*statsService.Stat{
		{stat("user>>>alice>>>traffic>>>uplink", 100)},
		{stat("user>>>alice>>>traffic>>>uplink", 900)},
		{stat("user>>>alice>>>traffic>>>uplink", 250)},
	})

	if _, _, err := api.GetTraffic(); err != nil {
		t.Fatalf("GetTraffic (baseline): %v", err)
	}
	if _, _, err := api.GetTraffic(); err != nil {
		t.Fatalf("GetTraffic (delta): %v", err)
	}
	_, clients, err := api.GetTraffic()
	if err != nil {
		t.Fatalf("GetTraffic (after reset): %v", err)
	}

	alice, ok := clientTrafficByEmail(t, clients)["alice"]
	if !ok {
		t.Fatal("traffic after a counter reset was dropped entirely")
	}
	if alice.Up != 250 {
		t.Fatalf("alice up = %d, want 250 counted from zero", alice.Up)
	}
}

// TestGetTrafficSkipsAPIInboundAndPrunes checks the tag-level parsing: the api
// inbound is the panel's own gRPC channel and is never a user-facing inbound,
// and baselines for vanished stats are dropped once they outgrow the live set.
func TestGetTrafficSkipsAPIInboundAndPrunes(t *testing.T) {
	api := startFakeStats(t, [][]*statsService.Stat{
		{
			stat("inbound>>>api>>>traffic>>>uplink", 10),
			stat("inbound>>>in-443>>>traffic>>>uplink", 10),
			stat("inbound>>>gone-1>>>traffic>>>uplink", 10),
			stat("inbound>>>gone-2>>>traffic>>>uplink", 10),
			stat("inbound>>>gone-3>>>traffic>>>uplink", 10),
			stat("inbound>>>gone-4>>>traffic>>>uplink", 10),
			stat("inbound>>>gone-5>>>traffic>>>uplink", 10),
		},
		{
			stat("inbound>>>api>>>traffic>>>uplink", 99),
			stat("inbound>>>in-443>>>traffic>>>uplink", 60),
			stat("inbound>>>in-443>>>traffic>>>downlink", 70),
		},
	})

	if _, _, err := api.GetTraffic(); err != nil {
		t.Fatalf("GetTraffic (baseline): %v", err)
	}
	tags, _, err := api.GetTraffic()
	if err != nil {
		t.Fatalf("GetTraffic: %v", err)
	}

	if len(tags) != 1 {
		t.Fatalf("got %d tag traffics, want only the non-api inbound: %+v", len(tags), tags)
	}
	if tags[0].Tag != "in-443" || !tags[0].IsInbound || tags[0].IsOutbound {
		t.Fatalf("tag traffic = %+v, want inbound in-443", tags[0])
	}
	if tags[0].Up != 50 || tags[0].Down != 70 {
		t.Fatalf("in-443 = up %d / down %d, want 50 / 70", tags[0].Up, tags[0].Down)
	}
	if _, stale := api.StatsLastValues["inbound>>>gone-1>>>traffic>>>uplink"]; stale {
		t.Fatal("baselines for stats that no longer exist were not pruned")
	}
}
