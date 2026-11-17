package job

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func TestCheckConntrackJobPublishesUsage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOST_PROC", root)
	dir := filepath.Join(root, "sys/net/netfilter")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nf_conntrack_max"), []byte("1000\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	j := NewCheckConntrackJob(func() bool { return true })
	events := captureConntrackEvents(t, func() {
		for _, count := range []string{"799", "800", "1001", "700", "699", "0", "800"} {
			if err := os.WriteFile(filepath.Join(dir, "nf_conntrack_count"), []byte(count+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			j.Run()
		}
	})
	if len(events) != 7 {
		t.Fatalf("events = %v, want one event for every valid sample", events)
	}
	for i, count := range []uint64{799, 800, 1001, 700, 699, 0, 800} {
		if events[i].Type != eventbus.EventConntrackHigh {
			t.Fatalf("event %d type = %s, want conntrack.high", i, events[i].Type)
		}
		data := events[i].Data.(*eventbus.ConntrackData)
		if data.Count != count || data.Max != 1000 || data.Percent != float64(count)/1000*100 {
			t.Fatalf("sample %d data = %+v", i, data)
		}
	}
}

func TestCheckConntrackJobSkipsInvalidSamples(t *testing.T) {
	for _, sample := range []struct {
		name       string
		count, max uint64
		err        error
	}{
		{name: "missing", err: os.ErrNotExist},
		{name: "permission", err: os.ErrPermission},
		{name: "invalid sample", err: errors.New("invalid conntrack counter")},
		{name: "zero maximum"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			count, max := uint64(80), uint64(100)
			var readErr error
			j := &CheckConntrackJob{enabled: func() bool { return true }, readUsage: func() (uint64, uint64, error) {
				return count, max, readErr
			}}
			events := captureConntrackEvents(t, func() {
				j.Run()
				count, max, readErr = sample.count, sample.max, sample.err
				j.Run()
				count, max, readErr = 0, 100, nil
				j.Run()
			})
			if len(events) != 2 || events[0].Type != eventbus.EventConntrackHigh || events[1].Type != eventbus.EventConntrackHigh {
				t.Fatalf("events = %v, want only the two valid samples", events)
			}
		})
	}
}

func TestCheckConntrackJobRetriesAfterReadError(t *testing.T) {
	err := errors.New("temporary read error")
	j := &CheckConntrackJob{enabled: func() bool { return true }, readUsage: func() (uint64, uint64, error) {
		return 100, 100, err
	}}
	events := captureConntrackEvents(t, func() {
		j.Run()
		err = nil
		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				j.Run()
			}()
		}
		wg.Wait()
	})
	if len(events) != 10 {
		t.Fatalf("events = %v, want one high event for each successful sample", events)
	}
	for _, e := range events {
		if e.Type != eventbus.EventConntrackHigh {
			t.Fatalf("event = %v, want high", e)
		}
	}
}

func TestCheckConntrackJobRuntimeToggle(t *testing.T) {
	enabled := false
	count := uint64(80)
	reads := 0
	j := NewCheckConntrackJob(func() bool { return enabled })
	j.readUsage = func() (uint64, uint64, error) {
		reads++
		return count, 100, nil
	}
	events := captureConntrackEvents(t, func() {
		j.Run()
		if reads != 0 {
			t.Fatal("disabled monitor read proc files")
		}
		enabled = true
		j.Run()
		j.Run()
		enabled = false
		j.Run()
		enabled = true
		j.Run()
		enabled = false
		j.Run()
		count = 0
		enabled = true
		j.Run()
	})
	if len(events) != 4 {
		t.Fatalf("events = %v, want usage for each enabled sample", events)
	}
	for _, e := range events {
		if e.Type != eventbus.EventConntrackHigh {
			t.Fatalf("event = %v, want high", e)
		}
	}
}

func captureConntrackEvents(t *testing.T, run func()) []eventbus.Event {
	t.Helper()
	bus := eventbus.New(0)
	previous := EventBus
	EventBus = bus
	t.Cleanup(func() {
		bus.Stop()
		EventBus = previous
	})
	got := make(chan eventbus.Event, 32)
	bus.Subscribe("conntrack-test", func(e eventbus.Event) { got <- e })
	run()
	const done eventbus.EventType = "test.done"
	bus.Publish(eventbus.Event{Type: done})
	var events []eventbus.Event
	for {
		select {
		case e := <-got:
			if e.Type == done {
				return events
			}
			events = append(events, e)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for conntrack events")
		}
	}
}
