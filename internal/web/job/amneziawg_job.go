package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AmneziaWGJob reconciles the running AmneziaWG interfaces against the
// enabled AmneziaWG inbounds in the database, restarts/reloads any that
// drifted, and folds the per-peer traffic scraped from `awg show dump` into
// the usual client and inbound traffic accounting. Mirrors MtprotoJob.
type AmneziaWGJob struct {
	inboundService service.InboundService
	// warnedMissing tracks whether the "awg/awg-quick not found" warning has
	// already been logged, so a host without the AmneziaWG kernel module
	// (the Docker image, RHEL, Arch, or a failed install.sh PPA step) logs it
	// once instead of every @every-10s tick forever.
	warnedMissing bool
}

// NewAmneziaWGJob creates a new AmneziaWG reconcile/traffic job instance.
func NewAmneziaWGJob() *AmneziaWGJob {
	return new(AmneziaWGJob)
}

// Run reconciles desired AmneziaWG inbounds with running interfaces and
// records per-peer traffic deltas and online status.
func (j *AmneziaWGJob) Run() {
	desired, err := j.inboundService.DesiredAmneziaWGInstances()
	if err != nil {
		logger.Warning("amneziawg job: get desired instances failed:", err)
		return
	}

	// Only relevant once an admin actually has an AmneziaWG inbound: no
	// point warning about a missing binary the panel never needed to touch.
	if len(desired) > 0 && !amneziawg.IsAwgInstalled() {
		if !j.warnedMissing {
			j.warnedMissing = true
			logger.Warningf("amneziawg job: %d AmneziaWG inbound(s) configured but awg/awg-quick not found on PATH; skipping reconcile until installed", len(desired))
		}
		return
	}
	j.warnedMissing = false

	activeTags := make([]string, 0, len(desired))
	for _, inst := range desired {
		activeTags = append(activeTags, inst.Tag)
	}

	mgr := amneziawg.GetManager()
	mgr.Reconcile(desired)

	deltas, onlineEmails := mgr.CollectTraffic()

	clientTraffics := make([]*xray.ClientTraffic, 0, len(deltas))
	inboundUp := make(map[string]int64)
	inboundDown := make(map[string]int64)
	for _, d := range deltas {
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: d.Email,
			Up:    d.Up,
			Down:  d.Down,
		})
		inboundUp[d.Tag] += d.Up
		inboundDown[d.Tag] += d.Down
	}

	traffics := make([]*xray.Traffic, 0, len(inboundUp))
	for tag, up := range inboundUp {
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       tag,
			Up:        up,
			Down:      inboundDown[tag],
		})
	}

	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("amneziawg job: add traffic failed:", err)
		}
	}

	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}
