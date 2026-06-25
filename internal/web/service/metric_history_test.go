package service

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestMetricMemoryFootprint(t *testing.T) {
	const metrics = 16

	retained := func(build func() any) uint64 {
		runtime.GC()
		runtime.GC()
		var m0 runtime.MemStats
		runtime.ReadMemStats(&m0)
		obj := build()
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)
		runtime.KeepAlive(obj)
		if m1.HeapAlloc < m0.HeapAlloc {
			return 0
		}
		return m1.HeapAlloc - m0.HeapAlloc
	}

	fill := func(buf []MetricSample) {
		for j := range buf {
			buf[j] = MetricSample{T: int64(j), V: float64(j)}
		}
	}

	oldFlat := retained(func() any {
		m := make(map[string][]MetricSample, metrics)
		for i := range metrics {
			buf := make([]MetricSample, 86400)
			fill(buf)
			m[fmt.Sprintf("m%d", i)] = buf
		}
		return m
	})

	newTiered := retained(func() any {
		h := newMetricHistory()
		for i := range metrics {
			s := newSeries()
			for _, tb := range s.tiers {
				buf := make([]MetricSample, tb.capacity)
				fill(buf)
				tb.samples = buf
			}
			h.series[fmt.Sprintf("m%d", i)] = s
		}
		return h
	})

	t.Logf("metric history footprint (16 system metrics, full):")
	t.Logf("  before (flat 48h@2s): %d KiB", oldFlat/1024)
	t.Logf("  after  (tiered 7d):   %d KiB", newTiered/1024)
	if newTiered >= oldFlat {
		t.Fatalf("expected tiered footprint smaller: old=%d new=%d", oldFlat, newTiered)
	}
}

func TestTierBufRollupAveragesClosedBuckets(t *testing.T) {
	tb := &tierBuf{resolution: 10, capacity: 100}
	tb.add(0, 2)
	tb.add(2, 4)
	tb.add(5, 6)
	tb.add(10, 10)

	if len(tb.samples) != 1 || tb.samples[0].T != 0 || tb.samples[0].V != 4 {
		t.Fatalf("expected one closed bucket {0,4}, got %+v", tb.samples)
	}

	got := tb.readSamples()
	if len(got) != 2 || got[1].T != 10 || got[1].V != 10 {
		t.Fatalf("expected open bucket {10,10} appended on read, got %+v", got)
	}
}

func TestTierBufRespectsCapacity(t *testing.T) {
	tb := &tierBuf{resolution: 1, capacity: 5}
	for i := range int64(20) {
		tb.add(i, float64(i))
	}
	if len(tb.samples) != 5 {
		t.Fatalf("expected closed buckets capped at 5, got %d", len(tb.samples))
	}
	if last := tb.samples[len(tb.samples)-1]; last.T != 18 {
		t.Fatalf("expected last closed bucket T=18 (19 still open), got %d", last.T)
	}
}

func TestSeriesPickTierBySpan(t *testing.T) {
	s := newSeries()
	cases := []struct {
		span int64
		res  int
	}{
		{120, 2},
		{3600, 2},
		{7200, 60},
		{172800, 60},
		{604800, 600},
		{9999999, 600},
	}
	for _, c := range cases {
		if got := s.pickTier(c.span); got.resolution != c.res {
			t.Errorf("span %d: expected resolution %d, got %d", c.span, c.res, got.resolution)
		}
	}
}

func TestAggregateFineRealtime(t *testing.T) {
	h := newMetricHistory()
	now := time.Now().Unix()
	for i := int64(59); i >= 0; i-- {
		h.append("cpu", time.Unix(now-i*2, 0), float64(100-i))
	}

	out := h.aggregate("cpu", 2, 60)
	if len(out) == 0 {
		t.Fatalf("expected non-empty realtime aggregate")
	}
	if _, ok := out[len(out)-1]["v"].(float64); !ok {
		t.Fatalf("expected float64 value, got %T", out[len(out)-1]["v"])
	}
}

func TestAggregateLongSpanUsesCoarseTier(t *testing.T) {
	h := newMetricHistory()
	now := time.Now().Unix()
	for i := range int64(200) {
		ts := now - (200-i)*600
		h.append("cpu", time.Unix(ts, 0), float64(i))
	}

	out := h.aggregate("cpu", 10080, 60)
	if len(out) == 0 {
		t.Fatalf("expected non-empty 7d aggregate from the archive tier")
	}
}

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	h := newMetricHistory()
	now := time.Now().Unix()
	for i := range int64(10) {
		h.append("cpu", time.Unix(now-(9-i)*2, 0), float64(i))
	}

	h2 := newMetricHistory()
	h2.restore(h.snapshot())

	if out := h2.aggregate("cpu", 2, 60); len(out) == 0 {
		t.Fatalf("expected restored series to aggregate")
	}
}

func TestAggregateMissingMetricIsEmpty(t *testing.T) {
	h := newMetricHistory()
	if out := h.aggregate("nope", 2, 60); len(out) != 0 {
		t.Fatalf("expected empty result for unknown metric, got %d points", len(out))
	}
}
