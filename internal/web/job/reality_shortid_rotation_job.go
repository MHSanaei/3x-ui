package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
)

// RealityShortIDRotationJob rotates due IDs and retires grace-expired ones.
// The service owns atomic writes and local/remote runtime updates.
type RealityShortIDRotationJob struct {
	inboundService service.InboundService
	xrayService    service.XrayService
	now            func() time.Time
}

func NewRealityShortIDRotationJob() *RealityShortIDRotationJob {
	return &RealityShortIDRotationJob{now: time.Now}
}

func (j *RealityShortIDRotationJob) Run() {
	now := time.Now
	if j.now != nil {
		now = j.now
	}
	result, err := j.inboundService.ProcessRealityShortIDRotations(now())
	if err != nil {
		logger.Warning("REALITY short ID rotation completed with errors:", err)
	}
	if result.NeedRestart {
		j.xrayService.SetToNeedRestart()
	}
	if result.Rotated > 0 || result.Retired > 0 {
		logger.Infof(
			"REALITY short ID maintenance completed: %d rotated, %d retired",
			result.Rotated,
			result.Retired,
		)
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
	}
}
