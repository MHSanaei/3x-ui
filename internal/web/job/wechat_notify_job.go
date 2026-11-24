package job

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/wechat"
)

type WechatNotifyJob struct {
	serverService  wechat.ServerProvider
	inboundService wechat.InboundProvider
	wechatService  *wechat.Service
}

type wechatReportServer struct {
	server        wechat.ServerProvider
	mu            sync.Mutex
	nativeSampled bool
}

func (s *wechatReportServer) GetStatus(last *service.Status) *service.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.server.GetStatus(last)
	if status == nil || status.ResourceScope != "" || s.nativeSampled {
		return status
	}
	s.nativeSampled = true
	if !status.MetricAvailable("cpu") {
		return status
	}
	// A dedicated ServerService initializes its native CPU baseline with zero.
	// Preserve the snapshot and label that first value as unavailable.
	copy := *status
	copy.UnavailableMetrics = append(slices.Clone(status.UnavailableMetrics), "cpu")
	return &copy
}

func NewWechatNotifyJob(svc *wechat.Service) *WechatNotifyJob {
	return &WechatNotifyJob{serverService: &wechatReportServer{server: &service.ServerService{}}, inboundService: &service.InboundService{}, wechatService: svc}
}

// Run reports status even if Xray is down; no backup or configuration files are read.
func (job *WechatNotifyJob) Run() {
	if job.wechatService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := job.wechatService.SendReport(ctx, job.serverService, job.inboundService); err != nil {
		logger.Warning("WechatNotifyJob: ", err)
	}
}
