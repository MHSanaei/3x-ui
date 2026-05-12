package service

import (
	"sync"
	"time"
)

// MetricSample is one point of any time-series we keep in memory.
// The frontend deserializes both keys, so they must stay short.
type MetricSample struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// metricCapacityDefault caps each ring buffer at ~5h worth of @2s samples
// or ~25h worth of @10s samples. Plenty for the bucketed aggregation
// view and small enough that the working set per metric stays under
// ~150 KiB.
const metricCapacityDefault = 9000

// metricHistory is a thread-safe, in-memory ring buffer keyed by
// arbitrary strings. Two singletons live below: one for system-wide
// host metrics, one for per-node metrics. Keeping them in this file
// (rather than scattered across services) makes the storage model
// easy to reason about and avoids double-locking.
type metricHistory struct {
	mu      sync.Mutex
	metrics map[string][]MetricSample
}

func newMetricHistory() *metricHistory {
	return &metricHistory{metrics: map[string][]MetricSample{}}
}

// append stores a single sample for the given metric, deduping when
// two appends happen within the same wall-clock second (which can
// happen if the cron tick is faster than the metric's natural rate).
func (h *metricHistory) append(metric string, t time.Time, v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	buf := h.metrics[metric]
	p := MetricSample{T: t.Unix(), V: v}
	if n := len(buf); n > 0 && buf[n-1].T == p.T {
		buf[n-1] = p
	} else {
		buf = append(buf, p)
	}
	if len(buf) > metricCapacityDefault {
		buf = buf[len(buf)-metricCapacityDefault:]
	}
	h.metrics[metric] = buf
}

// drop removes the entire history for one metric. Used when a node is
// deleted so its old samples don't linger forever in the singleton.
func (h *metricHistory) drop(metric string) {
	h.mu.Lock()
	delete(h.metrics, metric)
	h.mu.Unlock()
}

// aggregate returns up to maxPoints buckets of size bucketSeconds,
// each bucket carrying the arithmetic mean of the underlying samples.
// Bucket alignment is to absolute Unix-second boundaries so two
// concurrent calls (e.g. two browser tabs) see identical x-axes.
func (h *metricHistory) aggregate(metric string, bucketSeconds int, maxPoints int) []map[string]any {
	if bucketSeconds <= 0 || maxPoints <= 0 {
		return []map[string]any{}
	}
	cutoff := time.Now().Add(-time.Duration(bucketSeconds*maxPoints) * time.Second).Unix()

	h.mu.Lock()
	hist := h.metrics[metric]
	startIdx := 0
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].T < cutoff {
			startIdx = i + 1
			break
		}
	}
	if startIdx >= len(hist) {
		h.mu.Unlock()
		return []map[string]any{}
	}
	tmp := make([]MetricSample, len(hist)-startIdx)
	copy(tmp, hist[startIdx:])
	h.mu.Unlock()

	if len(tmp) == 0 {
		return []map[string]any{}
	}

	bSize := int64(bucketSeconds)
	curBucket := (tmp[0].T / bSize) * bSize
	var out []map[string]any
	var acc []float64
	flush := func(ts int64) {
		if len(acc) == 0 {
			return
		}
		sum := 0.0
		for _, v := range acc {
			sum += v
		}
		out = append(out, map[string]any{"t": ts, "v": sum / float64(len(acc))})
		acc = acc[:0]
	}
	for _, p := range tmp {
		b := (p.T / bSize) * bSize
		if b != curBucket {
			flush(curBucket)
			curBucket = b
		}
		acc = append(acc, p.V)
	}
	flush(curBucket)
	if len(out) > maxPoints {
		out = out[len(out)-maxPoints:]
	}
	if out == nil {
		return []map[string]any{}
	}
	return out
}

// systemMetrics holds whole-host time series (cpu, mem, netUp, etc.)
// fed by ServerController.refreshStatus every 2s. nodeMetrics holds
// per-node CPU/Mem fed by NodeHeartbeatJob every 10s. Both are
// process-local — survival across panel restart is not required.
var (
	systemMetrics = newMetricHistory()
	nodeMetrics   = newMetricHistory()
	xrayMetrics   = newMetricHistory()
)

// SystemMetricKeys lists the metric names ServerService writes on every
// status sample. Exposed for documentation/test purposes; the
// controller validates incoming names against an allow-list.
var SystemMetricKeys = []string{
	"cpu", "mem", "netUp", "netDown", "online", "load1", "load5", "load15",
}

// NodeMetricKeys lists the per-node metric names NodeHeartbeatJob writes.
var NodeMetricKeys = []string{"cpu", "mem"}

// XrayMetricKeys lists series sourced from xray's /debug/vars expvar
// endpoint. Populated by XrayMetricsService.Sample on the same 2s cadence
// as the system metrics, but only when the xray config has a `metrics`
// block configured.
var XrayMetricKeys = []string{
	"xrAlloc", "xrSys", "xrHeapObjects", "xrNumGC", "xrPauseNs",
}
