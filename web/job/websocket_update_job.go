package job

import (
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// WebSocketUpdateJob sends periodic updates via WebSocket
type WebSocketUpdateJob struct {
	wsService   *service.WebSocketService
	xrayService service.XrayService
}

// NewWebSocketUpdateJob creates a new WebSocket update job
func NewWebSocketUpdateJob(wsService *service.WebSocketService, xrayService service.XrayService) *WebSocketUpdateJob {
	return &WebSocketUpdateJob{
		wsService:   wsService,
		xrayService: xrayService,
	}
}

// Run sends system metrics update
func (j *WebSocketUpdateJob) Run() {
	if j.wsService == nil {
		return
	}

	// Get system metrics
	cpuPercents, _ := cpu.Percent(0, false)
	var cpuPercent float64
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	memInfo, err := mem.VirtualMemory()
	var memoryPercent float64
	if err == nil && memInfo != nil && memInfo.Total > 0 {
		memoryPercent = memInfo.UsedPercent
	}

	// Send system update
	j.wsService.SendSystemUpdate(cpuPercent, memoryPercent)

	// Send traffic update if Xray is running
	if j.xrayService.IsXrayRunning() {
		traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
		if err == nil {
			j.wsService.SendTrafficUpdate(traffics, clientTraffics)
		}
	}
}
