package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

// probe is one observatory sample: whether the outbound is alive and the
// last_try_time xray reports for it (a new probe advances lastTry).
type probe struct {
	alive   bool
	lastTry int64
}

const testSentinel eventbus.EventType = "test.sentinel"

// runObservatory feeds a probe sequence through applyObservatory with the given
// threshold and returns the outbound.* events it published, in order.
func runObservatory(t *testing.T, threshold int, seq []probe) []eventbus.EventType {
	t.Helper()

	ss := SettingService{}
	if err := ss.SetOutboundDownThreshold(threshold); err != nil {
		t.Fatalf("set threshold: %v", err)
	}

	bus := eventbus.New(256)
	events := make(chan eventbus.Event, 256)
	bus.Subscribe("test", func(e eventbus.Event) { events <- e })
	SetEventBus(bus)
	t.Cleanup(func() {
		SetEventBus(nil)
		bus.Stop()
	})

	s := &XrayMetricsService{settingService: ss}
	for _, p := range seq {
		s.applyObservatory(time.Unix(p.lastTry, 0), map[string]rawObsEntry{
			"proxy": {Alive: p.alive, Delay: 10, LastTryTime: p.lastTry, OutboundTag: "proxy"},
		})
	}

	bus.Publish(eventbus.Event{Type: testSentinel, Source: "x"})
	var got []eventbus.EventType
	for {
		select {
		case e := <-events:
			if e.Type == testSentinel {
				return got
			}
			got = append(got, e.Type)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for events to drain")
		}
	}
}

func TestApplyObservatoryDebounce(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	tests := []struct {
		name      string
		threshold int
		seq       []probe
		want      []eventbus.EventType
	}{
		{
			name:      "notifies only after threshold consecutive failed probes",
			threshold: 3,
			seq: []probe{
				{true, 1},
				{false, 2},
				{false, 3},
				{false, 4},
				{false, 5},
				{true, 6},
				{false, 7},
				{true, 8},
			},
			want: []eventbus.EventType{eventbus.EventOutboundDown, eventbus.EventOutboundUp},
		},
		{
			name:      "repeated samples of the same probe do not advance the streak",
			threshold: 3,
			seq:       []probe{{false, 2}, {false, 2}, {false, 2}, {false, 2}, {false, 2}},
			want:      nil,
		},
		{
			name:      "single-probe blip never notifies",
			threshold: 3,
			seq:       []probe{{true, 1}, {false, 2}, {true, 3}},
			want:      nil,
		},
		{
			name:      "threshold 1 keeps the legacy notify-on-first-failure behaviour",
			threshold: 1,
			seq:       []probe{{true, 1}, {false, 2}},
			want:      []eventbus.EventType{eventbus.EventOutboundDown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runObservatory(t, tt.threshold, tt.seq)
			if len(got) != len(tt.want) {
				t.Fatalf("events = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("event[%d] = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}
