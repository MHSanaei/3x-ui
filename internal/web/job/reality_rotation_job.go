package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// RealityRotationJob periodically rotates the shortIds (and optionally the
// x25519 keypair) of locally-hosted Reality inbounds that opted in via the
// per-inbound rotation interval. Rotating the shortId on a cadence keeps the
// Reality handshake pattern moving so DPI cannot fingerprint a long-lived
// endpoint (see https://github.com/ghostmcf/RealityGhost). Intervals are stored
// per inbound under streamSettings.realitySettings.rotation; 0 means disabled.
type RealityRotationJob struct {
	inboundService service.InboundService
	xrayService    service.XrayService
	serverService  service.ServerService
}

// NewRealityRotationJob creates a new Reality rotation job instance.
func NewRealityRotationJob() *RealityRotationJob {
	return new(RealityRotationJob)
}

// Run rotates every enabled, locally-hosted Reality inbound whose configured
// interval has elapsed, then asks xray to reload once if anything changed.
func (j *RealityRotationJob) Run() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("reality rotation job: get inbounds failed:", err)
		return
	}

	now := time.Now().Unix()
	genKeypair := func() (string, string, error) {
		obj, err := j.serverService.GetNewX25519Cert()
		if err != nil {
			return "", "", err
		}
		m, _ := obj.(map[string]any)
		priv, _ := m["privateKey"].(string)
		pub, _ := m["publicKey"].(string)
		return priv, pub, nil
	}

	restartNeeded := false
	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		newStream, changed, needRestart, err := service.RotateRealityStreamSettings(inbound.StreamSettings, now, genKeypair)
		if err != nil {
			logger.Warning("reality rotation job: rotate inbound", inbound.Id, "failed:", err)
			continue
		}
		if !changed {
			continue
		}
		if err := j.inboundService.UpdateInboundStreamSettings(inbound.Id, newStream); err != nil {
			logger.Warning("reality rotation job: persist inbound", inbound.Id, "failed:", err)
			continue
		}
		if needRestart {
			restartNeeded = true
			logger.Info("reality rotation job: rotated inbound", inbound.Id)
		}
	}

	if restartNeeded {
		j.xrayService.SetToNeedRestart()
	}
}
