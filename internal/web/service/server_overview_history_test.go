package service

import (
	"testing"
	"time"
)

// The overview draws eight charts from this one response; a series missing from it
// or filled from another metric's samples leaves a chart blank or wrong.
func TestAggregateOverviewHistoryCarriesEveryOverviewSeries(t *testing.T) {
	old := systemMetrics
	systemMetrics = newMetricHistory()
	t.Cleanup(func() { systemMetrics = old })
	want := map[string]float64{
		"cpu": 11, "mem": 22, "swap": 33, "diskUsage": 44,
		"netUp": 55, "netDown": 66, "tcpCount": 77, "udpCount": 88,
	}
	now := time.Now()
	for key, value := range want {
		systemMetrics.append(key, now, value)
	}
	systemMetrics.append("load1", now, 99)

	got := (&ServerService{lastStatus: &Status{}}).AggregateOverviewHistory(2, 60)
	if len(got) != len(want) {
		t.Fatalf("series = %d, want the %d the overview draws: %v", len(got), len(want), got)
	}
	for key, value := range want {
		if points := got[key]; len(points) != 1 || points[0]["v"] != value {
			t.Errorf("%s = %v, want one point of %v", key, points, value)
		}
	}
}
