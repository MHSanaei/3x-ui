package service

import (
	"slices"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/sys"
)

const containerHistoryPrefix = "lxc-v1/"

func scopedMetric(metric string) bool {
	return metric == "cpu" || metric == "swap"
}

func (s *ServerService) systemMetricKey(metric string) string {
	if !scopedMetric(metric) {
		return metric
	}
	if status := s.LastStatus(); status != nil {
		if status.ResourceScope != "" {
			return containerHistoryPrefix + metric
		}
		return metric
	}
	if s.lxcMetrics.UseScopedHistory() {
		return containerHistoryPrefix + metric
	}
	return metric
}

// Scope metadata applies only to CPU, swap and uptime.
func applyLXCStats(status *Status, sample *sys.LXCStats) {
	status.ResourceScope = sample.Scope
	status.UnavailableMetrics = append([]string(nil), sample.Unavailable...)
	status.Cpu = sample.CPU
	status.CpuCores = sample.CPUs
	status.LogicalPro = sample.CPUs
	status.Swap.Current = sample.SwapUsed
	status.Swap.Total = sample.SwapTotal
	status.Uptime = sample.Uptime
}

// MetricAvailable distinguishes an unavailable metric from a measured zero.
// Legacy status responses omit metadata and retain their previous semantics.
func (s *Status) MetricAvailable(metric string) bool {
	return s != nil && !slices.Contains(s.UnavailableMetrics, metric)
}

func appendScopedMetric(status *Status, metric string, t time.Time, value float64) {
	if !status.MetricAvailable(metric) {
		return
	}
	if status.ResourceScope != "" {
		metric = containerHistoryPrefix + metric
	}
	systemMetrics.append(metric, t, value)
}
