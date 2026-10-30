package job

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"

	"github.com/shirou/gopsutil/v4/cpu"
)

// The alarm must report usage over the minute between runs, not since boot:
// cumulative counters alone would hide a spike behind days of idle time.
func TestCheckCpuJobPublishesUsageSinceThePreviousRun(t *testing.T) {
	samples := []cpu.TimesStat{
		{User: 1000, System: 500, Idle: 98500},
		{User: 1060, System: 515, Idle: 98525}, // 60+15 busy, 25 idle: 75%
	}
	j := &CheckCpuJob{readTimes: func(bool) ([]cpu.TimesStat, error) {
		s := samples[0]
		samples = samples[1:]
		return []cpu.TimesStat{s}, nil
	}}

	bus := eventbus.New(0)
	t.Cleanup(bus.Stop)
	got := make(chan float64, 2)
	bus.Subscribe("test", func(e eventbus.Event) {
		got <- e.Data.(*eventbus.SystemMetricData).Percent
	})
	prevBus := EventBus
	EventBus = bus
	t.Cleanup(func() { EventBus = prevBus })

	j.Run()
	j.Run()

	select {
	case p := <-got:
		if p != 75 {
			t.Fatalf("published %v%%, want 75%% (busy share between the two runs)", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no cpu.high event published by the second run")
	}
	select {
	case p := <-got:
		t.Fatalf("first run has no baseline yet but published %v%%", p)
	case <-time.After(100 * time.Millisecond):
	}
}

// A Run that samples for its whole period keeps cron.Stop waiting, and with it
// every panel restart, for up to a minute behind a closed listener.
func TestCheckCpuJobRunDoesNotBlockForTheSamplePeriod(t *testing.T) {
	j := NewCheckCpuJob()
	done := make(chan struct{})
	go func() {
		j.Run()
		j.Run()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CheckCpuJob.Run still blocking after 5s; it must read CPU time since its previous run")
	}
}
