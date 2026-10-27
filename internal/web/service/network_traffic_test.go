package service

import (
	"errors"
	"math"
	stdnet "net"
	"reflect"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

func networkTestInterfaces() []stdnet.Interface {
	return []stdnet.Interface{
		{Index: 1, Name: "eth0", Flags: stdnet.FlagUp},
		{Index: 2, Name: "enP6129s1", Flags: stdnet.FlagUp},
		{Index: 3, Name: "ens3", Flags: stdnet.FlagUp},
		{Index: 4, Name: "wg0", Flags: stdnet.FlagUp},
	}
}

func TestNetworkLegacyRatesRebaseOnMemberResetAndMembershipChanges(t *testing.T) {
	read := func(counters []net.IOCountersStat) *networkTrafficSample {
		t.Helper()
		sample, err := selectNetworkTraffic(networkTrafficConfig{mode: "legacy"}, counters, networkTestInterfaces(), "")
		if err != nil {
			t.Fatal(err)
		}
		return &sample
	}
	before := read([]net.IOCountersStat{{Name: "eth0", BytesSent: 100}, {Name: "ens3", BytesSent: 100}})
	reset := read([]net.IOCountersStat{{Name: "ens3", BytesSent: 1000}, {Name: "eth0", BytesSent: 50}})
	if got := networkTrafficRates(before, reset, time.Second); got != [4]uint64{} {
		t.Fatalf("member reset hidden by aggregate increase: %v", got)
	}
	resumed := read([]net.IOCountersStat{{Name: "eth0", BytesSent: 150}, {Name: "ens3", BytesSent: 1100}})
	if got := networkTrafficRates(reset, resumed, 2*time.Second); got != [4]uint64{100, 0, 0, 0} {
		t.Fatalf("resumed rates=%v", got)
	}
	removed := read([]net.IOCountersStat{{Name: "eth0", BytesSent: 200}})
	if got := networkTrafficRates(resumed, removed, time.Second); got != [4]uint64{} {
		t.Fatalf("membership change rates=%v", got)
	}
	if _, err := selectNetworkTraffic(networkTrafficConfig{mode: "legacy"}, []net.IOCountersStat{{Name: "eth0", BytesSent: math.MaxUint64}, {Name: "ens3", BytesSent: 1}}, networkTestInterfaces(), ""); !errors.Is(err, errNetworkUnavailable) {
		t.Fatalf("aggregate overflow=%v", err)
	}
	zero := read([]net.IOCountersStat{{Name: "eth0"}})
	maximum := read([]net.IOCountersStat{{Name: "eth0", BytesSent: math.MaxUint64}})
	if got := networkTrafficRates(zero, maximum, time.Nanosecond); got != [4]uint64{} {
		t.Fatalf("unrepresentable rate=%v", got)
	}
}

func TestNetworkTrafficCollectionRebasesAfterFailureAndRecovery(t *testing.T) {
	setupSettingTestDB(t)
	settings := &SettingService{}
	iface := "eth0"
	bytes := uint64(30 << 30)
	failed := false
	reader := &networkTrafficReader{
		interfaces: func() ([]stdnet.Interface, error) { return networkTestInterfaces(), nil },
		counters: func() ([]net.IOCountersStat, error) {
			if failed {
				return nil, errors.New("counter read failed")
			}
			return []net.IOCountersStat{{Name: iface, BytesSent: bytes, BytesRecv: bytes, PacketsSent: bytes / 10, PacketsRecv: bytes / 10}, {Name: "enP6129s1", BytesSent: bytes, BytesRecv: bytes}}, nil
		},
		routes: func([]stdnet.Interface) ([]networkDefaultRoute, error) {
			if iface == "" {
				return nil, errNetworkUnavailable
			}
			return []networkDefaultRoute{{iface, 4, 10}}, nil
		},
	}
	svc := &ServerService{networkTrafficReader: reader, cachedCpuSpeedMhz: 1}
	legacy := svc.GetStatus(nil)
	if legacy.NetTraffic.Sent != 2*bytes || legacy.NetworkInterface.Mode != "legacy" {
		t.Fatalf("legacy default: sent=%d selection=%+v", legacy.NetTraffic.Sent, legacy.NetworkInterface)
	}
	if err := settings.saveSetting("networkTrafficMode", "auto"); err != nil {
		t.Fatal(err)
	}
	first := svc.GetStatus(legacy)
	if first.NetTraffic.Sent != bytes || first.NetworkInterface == nil || first.NetworkInterface.Name != "eth0" || first.NetIO.Up != 0 {
		t.Fatalf("central initial status: %+v", first)
	}
	previous := first
	at := first.T
	for _, tc := range []struct {
		name, iface       string
		bytes             uint64
		failed, available bool
		wantRate          uint64
	}{
		{"increasing", "eth0", bytes + 400, false, true, 200},
		{"route changes", "ens3", 100, false, true, 0},
		{"new route resumes", "ens3", 500, false, true, 200},
		{"no route", "", 0, false, false, 0},
		{"route returns", "ens3", 900, false, true, 0},
		{"counter read fails", "ens3", 900, true, false, 0},
		{"counter read recovers", "ens3", 1300, false, true, 0},
		{"resumes after recovery", "ens3", 1700, false, true, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iface, bytes, failed = tc.iface, tc.bytes, tc.failed
			at = at.Add(2 * time.Second)
			current := &Status{T: at}
			svc.collectNetworkTraffic(current, previous)
			if current.NetworkInterface.Available != tc.available || current.NetIO.Up != tc.wantRate || current.MetricAvailable("netTraffic") != tc.available {
				t.Fatalf("selection=%+v rates=%+v unavailable=%v", current.NetworkInterface, current.NetIO, current.UnavailableMetrics)
			}
			previous = current
		})
	}
	if info, err := svc.GetNetworkInterfaces(); err != nil || info.Automatic != "ens3" || len(info.Interfaces) != 4 {
		t.Fatalf("interface response=%+v, err=%v", info, err)
	}
}

func TestNetworkUnavailableHistoryIsNotRecordedAsZero(t *testing.T) {
	old := systemMetrics
	systemMetrics = newMetricHistory()
	t.Cleanup(func() { systemMetrics = old })
	s := &ServerService{}
	status := &Status{UnavailableMetrics: []string{"netUp", "netDown", "pktUp", "pktDown"}}
	s.AppendStatusSample(time.Now(), status)
	for _, metric := range []string{"netUp", "netDown", "pktUp", "pktDown"} {
		if got := s.AggregateSystemMetric(metric, 2, 60); len(got) != 0 {
			t.Fatalf("unavailable %s history=%v", metric, got)
		}
	}
	status.UnavailableMetrics = nil
	s.AppendStatusSample(time.Now(), status)
	if got := s.AggregateSystemMetric("netUp", 2, 60); len(got) != 1 || got[0]["v"] != float64(0) {
		t.Fatalf("valid zero history=%v", got)
	}
}

func TestNetworkTrafficSelectionAvoidsStackedDoubleCounting(t *testing.T) {
	const gb = uint64(1 << 30)
	counters := []net.IOCountersStat{
		{Name: "eth0", BytesSent: 30 * gb, BytesRecv: 12 * gb, PacketsSent: 100, PacketsRecv: 200},
		{Name: "enP6129s1", BytesSent: 30 * gb, BytesRecv: 12 * gb, PacketsSent: 100, PacketsRecv: 200},
		{Name: "ens3", BytesSent: gb, BytesRecv: 2 * gb},
		{Name: "wg0", BytesSent: 3 * gb, BytesRecv: 4 * gb},
		{Name: "lo", BytesSent: 9 * gb},
	}
	for _, tc := range []struct {
		name, mode, selected string
		wantSent, wantRecv   uint64
		wantPackets          uint64
	}{
		{"legacy", "legacy", "", 61 * gb, 26 * gb, 200},
		{"manual synthetic", "manual", "eth0", 30 * gb, 12 * gb, 100},
		{"manual SR-IOV", "manual", "enP6129s1", 30 * gb, 12 * gb, 100},
		{"manual unrelated", "manual", "ens3", gb, 2 * gb, 0},
		{"manual tunnel", "manual", "wg0", 3 * gb, 4 * gb, 0},
		{"automatic", "auto", "", 30 * gb, 12 * gb, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sample, err := selectNetworkTraffic(networkTrafficConfig{tc.mode, tc.selected}, counters, networkTestInterfaces(), "eth0")
			if err != nil {
				t.Fatal(err)
			}
			if sample.sent != tc.wantSent || sample.recv != tc.wantRecv || sample.pktSent != tc.wantPackets {
				t.Fatalf("sent=%d recv=%d packets=%d; want %d/%d/%d", sample.sent, sample.recv, sample.pktSent, tc.wantSent, tc.wantRecv, tc.wantPackets)
			}
		})
	}
}

func TestNetworkTrafficMissingSelection(t *testing.T) {
	for _, config := range []networkTrafficConfig{{"manual", "missing"}, {"auto", ""}, {"invalid", ""}} {
		if _, err := selectNetworkTraffic(config, nil, nil, ""); !errors.Is(err, errNetworkUnavailable) {
			t.Fatalf("config %+v: got %v, want unavailable", config, err)
		}
	}
	got, err := selectNetworkTraffic(networkTrafficConfig{mode: "legacy"}, nil, nil, "")
	if err != nil || got.sent != 0 || got.recv != 0 {
		t.Fatalf("empty legacy: %+v, %v", got, err)
	}
}

func TestNetworkDefaultRouteSelection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		routes []networkDefaultRoute
		want   string
	}{
		{"IPv4 metric", []networkDefaultRoute{{"ens3", 4, 50}, {"eth0", 4, 10}}, "eth0"},
		{"IPv6 only", []networkDefaultRoute{{"ens3", 6, 50}, {"eth0", 6, 100}}, "ens3"},
		{"IPv4 preferred", []networkDefaultRoute{{"ens3", 6, 1}, {"eth0", 4, 500}}, "eth0"},
		{"deterministic ties", []networkDefaultRoute{{"ens3", 4, 10}, {"eth0", 4, 10}}, "ens3"},
		{"missing and down interfaces", []networkDefaultRoute{{"missing", 4, 0}, {"down", 4, 1}, {"eth0", 4, 20}}, "eth0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interfaces := append(networkTestInterfaces(), stdnet.Interface{Index: 5, Name: "down"})
			got, err := chooseNetworkDefaultRoute(tc.routes, interfaces)
			if err != nil || got != tc.want {
				t.Fatalf("route interface = %q, %v; want %q", got, err, tc.want)
			}
			for i, j := 0, len(tc.routes)-1; i < j; i, j = i+1, j-1 {
				tc.routes[i], tc.routes[j] = tc.routes[j], tc.routes[i]
			}
			got, err = chooseNetworkDefaultRoute(tc.routes, interfaces)
			if err != nil || got != tc.want {
				t.Fatalf("reversed route interface = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := chooseNetworkDefaultRoute(nil, nil); !errors.Is(err, errNetworkUnavailable) {
		t.Fatalf("empty routes: %v", err)
	}
}

func TestNetworkRatesRebaseIncompatibleSamples(t *testing.T) {
	interfaces := networkTestInterfaces()
	makeSample := func(name string, sent, packets uint64) *networkTrafficSample {
		t.Helper()
		s, err := selectNetworkTraffic(networkTrafficConfig{"manual", name}, []net.IOCountersStat{{Name: name, BytesSent: sent, BytesRecv: sent, PacketsSent: packets, PacketsRecv: packets}}, interfaces, "")
		if err != nil {
			t.Fatal(err)
		}
		return &s
	}
	previous := makeSample("eth0", 1000, 100)
	for _, tc := range []struct {
		name              string
		previous, current *networkTrafficSample
		elapsed           time.Duration
		want              [4]uint64
	}{
		{"normal", previous, makeSample("eth0", 1400, 140), 2 * time.Second, [4]uint64{200, 200, 20, 20}},
		{"startup", nil, previous, time.Second, [4]uint64{}},
		{"switch to smaller", previous, makeSample("ens3", 50, 5), time.Second, [4]uint64{}},
		{"switch to larger", previous, makeSample("ens3", 9000000, 500000), time.Second, [4]uint64{}},
		{"counter reset", previous, makeSample("eth0", 50, 5), time.Second, [4]uint64{}},
		{"packet reset", previous, makeSample("eth0", 1400, 1), time.Second, [4]uint64{}},
		{"missing", previous, nil, time.Second, [4]uint64{}},
		{"reappeared", nil, makeSample("eth0", 9000000, 500000), time.Second, [4]uint64{}},
		{"zero time", previous, makeSample("eth0", 1400, 140), 0, [4]uint64{}},
		{"backwards time", previous, makeSample("eth0", 1400, 140), -time.Second, [4]uint64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := networkTrafficRates(tc.previous, tc.current, tc.elapsed); got != tc.want {
				t.Fatalf("rates = %v, want %v", got, tc.want)
			}
		})
	}
	interfaces[0].Index = 99
	recreated := makeSample("eth0", 9000000, 500000)
	if got := networkTrafficRates(previous, recreated, time.Second); got != [4]uint64{} {
		t.Fatalf("recreated device rates: %v", got)
	}
	for _, mode := range []string{"legacy", "auto"} {
		sample, err := selectNetworkTraffic(networkTrafficConfig{mode: mode}, []net.IOCountersStat{{Name: "eth0", BytesSent: 9000000}}, interfaces, "eth0")
		if err != nil {
			t.Fatal(err)
		}
		if got := networkTrafficRates(previous, &sample, time.Second); !reflect.DeepEqual(got, [4]uint64{}) {
			t.Fatalf("mode switch %s: %v", mode, got)
		}
	}
}
