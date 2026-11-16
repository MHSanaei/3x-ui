package service

import (
	"testing"
	"time"
)

func TestConntrackStatusHistory(t *testing.T) {
	previous := systemMetrics
	systemMetrics = newMetricHistory()
	t.Cleanup(func() { systemMetrics = previous })
	s := &ServerService{}
	s.AppendStatusSample(time.Now(), &Status{})
	if got := systemMetrics.aggregate("conntrackCount", 2, 60); len(got) != 0 {
		t.Fatalf("unavailable conntrack recorded samples: %v", got)
	}
	s.AppendStatusSample(time.Now(), &Status{ConntrackCount: 6144, ConntrackMax: 7680})
	got := systemMetrics.aggregate("conntrackCount", 2, 60)
	if len(got) != 1 || got[0]["v"] != float64(6144) {
		t.Fatalf("conntrack samples = %v, want one value 6144", got)
	}
}
