package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type TuicJob struct {
	inboundService service.InboundService
}

func NewTuicJob() *TuicJob {
	return new(TuicJob)
}

func (j *TuicJob) Run() {
	desired, err := j.inboundService.DesiredTuicInstances()
	if err != nil {
		logger.Warning("tuic job: get desired instances failed:", err)
		return
	}

	activeTags := make([]string, 0, len(desired))
	for _, inst := range desired {
		activeTags = append(activeTags, inst.Tag)
	}

	mgr := tuic.GetManager()
	mgr.Reconcile(desired)

	deltas := mgr.CollectTraffic()
	onlineEmails, _ := mgr.GetActiveClients(30 * time.Second)

	clientTraffics := make([]*xray.ClientTraffic, 0)
	inboundUp := make(map[string]int64)
	inboundDown := make(map[string]int64)
	seenClients := make(map[string]bool)
	for _, d := range deltas {
		for email, stats := range d.Clients {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email: email,
				Up:    stats.Up,
				Down:  stats.Down,
			})
			seenClients[email] = true
		}
		inboundUp[d.Tag] += d.Up
		inboundDown[d.Tag] += d.Down
	}

	for _, email := range onlineEmails {
		if !seenClients[email] {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email: email,
				Up:    0,
				Down:  0,
			})
			seenClients[email] = true
		}
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
		needRestart, _, err := j.inboundService.AddTraffic(traffics, clientTraffics)
		if err != nil {
			logger.Warning("tuic job: add traffic failed:", err)
		} else if needRestart {
			if desired, err := j.inboundService.DesiredTuicInstances(); err == nil {
				mgr.Reconcile(desired)
			}
		}
	}

	if len(onlineEmails) > 0 {
		if err := j.inboundService.BumpClientsLastOnline(onlineEmails); err != nil {
			logger.Warning("tuic job: bump last online for tuic clients failed:", err)
		}
	}

	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}
