package service

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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

// snapshot returns a deep copy of every series, safe to serialize without
// holding the lock during disk I/O.
func (h *metricHistory) snapshot() map[string][]MetricSample {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string][]MetricSample, len(h.metrics))
	for k, v := range h.metrics {
		cp := make([]MetricSample, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// restore replaces the in-memory series with a previously persisted set,
// re-applying the per-series capacity cap so a tampered or oversized file
// can't grow the working set unbounded.
func (h *metricHistory) restore(data map[string][]MetricSample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, v := range data {
		if len(v) > metricCapacityDefault {
			v = v[len(v)-metricCapacityDefault:]
		}
		h.metrics[k] = v
	}
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
	for i, h := range slices.Backward(hist) {
		if h.T < cutoff {
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
// fed by ServerService.RefreshStatus every 2s. nodeMetrics holds
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
	"cpu", "mem", "swap", "netUp", "netDown", "pktUp", "pktDown", "diskRead", "diskWrite", "diskUsage", "tcpCount", "udpCount", "online", "load1", "load5", "load15",
}

// NodeMetricKeys lists the per-node metric names NodeHeartbeatJob writes.
var NodeMetricKeys = []string{"cpu", "mem", "netUp", "netDown"}

// XrayMetricKeys lists series sourced from xray's /debug/vars expvar
// endpoint. Populated by XrayMetricsService.Sample on the same 2s cadence
// as the system metrics, but only when the xray config has a `metrics`
// block configured.
var XrayMetricKeys = []string{
	"xrAlloc", "xrSys", "xrHeapObjects", "xrNumGC", "xrPauseNs",
}

// systemMetricsStorePath is where the host time-series is persisted between
// restarts. It lives next to the database so a single volume mount carries
// both. Only systemMetrics is persisted — node and xray series are cheap to
// rebuild and tied to live connections.
func systemMetricsStorePath() string {
	return filepath.Join(config.GetDBFolderPath(), "system_metrics.gob")
}

// PersistSystemMetrics writes the host time-series to disk via a temp file +
// rename so a crash mid-write can't corrupt the previous snapshot. Called on a
// timer and at shutdown.
func PersistSystemMetrics() error {
	path := systemMetricsStorePath()
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(systemMetrics.snapshot()); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// RestoreSystemMetrics loads a previously persisted host time-series on startup.
// A missing file is not an error (first boot). Aggregation already windows by
// time, so any gap from downtime is handled by the readers.
func RestoreSystemMetrics() {
	path := systemMetricsStorePath()
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning("restore system metrics failed:", err)
		}
		return
	}
	defer f.Close()
	var data map[string][]MetricSample
	if err := gob.NewDecoder(f).Decode(&data); err != nil {
		logger.Warning("decode system metrics failed:", err)
		return
	}
	systemMetrics.restore(data)
}
