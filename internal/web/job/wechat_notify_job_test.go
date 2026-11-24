package job

import (
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type wechatStatusSequence struct {
	statuses []*service.Status
}

func (s *wechatStatusSequence) GetStatus(*service.Status) *service.Status {
	status := s.statuses[0]
	s.statuses = s.statuses[1:]
	return status
}

func TestWechatReportDoesNotPresentCPUWarmupAsAnIdleHost(t *testing.T) {
	warmup := &service.Status{UnavailableMetrics: []string{"uptime"}}
	server := &wechatReportServer{server: &wechatStatusSequence{statuses: []*service.Status{
		warmup, {}, {Cpu: 25},
	}}}
	first := server.GetStatus(nil)
	if first.Cpu != 0 || first.MetricAvailable("cpu") || !slices.Equal(first.UnavailableMetrics, []string{"uptime", "cpu"}) {
		t.Fatalf("CPU warmup must be unavailable without losing other metadata: %+v", first)
	}
	if !warmup.MetricAvailable("cpu") || !slices.Equal(warmup.UnavailableMetrics, []string{"uptime"}) {
		t.Fatal("WeChat report changed its source status snapshot")
	}
	idle := server.GetStatus(nil)
	if idle.Cpu != 0 || !idle.MetricAvailable("cpu") {
		t.Fatalf("a measured idle sample must remain 0%% and available: %+v", idle)
	}
	busy := server.GetStatus(nil)
	if busy.Cpu != 25 || !busy.MetricAvailable("cpu") {
		t.Fatalf("a measured busy sample must remain 25%% and available: %+v", busy)
	}
}

func TestWechatReportWaitsForNativeCPUBaselineAfterMissingOrScopedStatus(t *testing.T) {
	scoped := &service.Status{ResourceScope: "lxc", UnavailableMetrics: []string{"cpu", "swap"}}
	server := &wechatReportServer{server: &wechatStatusSequence{statuses: []*service.Status{
		nil, scoped, {}, {},
	}}}
	if got := server.GetStatus(nil); got != nil {
		t.Fatalf("missing server status became a reportable status: %+v", got)
	}
	if got := server.GetStatus(nil); got != scoped || !slices.Equal(got.UnavailableMetrics, []string{"cpu", "swap"}) {
		t.Fatalf("scoped metric availability was changed: %+v", got)
	}
	if got := server.GetStatus(nil); got.MetricAvailable("cpu") {
		t.Fatal("missing or scoped status consumed the native CPU baseline")
	}
	if got := server.GetStatus(nil); got.Cpu != 0 || !got.MetricAvailable("cpu") {
		t.Fatalf("native CPU remained unavailable after its baseline: %+v", got)
	}
}
