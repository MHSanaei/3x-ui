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

	_, clientDeltas := mgr.CollectAllTraffic()
	onlineEmails, _ := mgr.GetActiveClients(30 * time.Second)

	clientTrafficMap := make(map[string]*xray.ClientTraffic, len(clientDeltas)+len(onlineEmails))
	for _, cd := range clientDeltas {
		clientTrafficMap[cd.Email] = &xray.ClientTraffic{
			Email: cd.Email,
			Up:    cd.Up,
			Down:  cd.Down,
		}
	}
	for _, email := range onlineEmails {
		if _, exists := clientTrafficMap[email]; !exists {
			clientTrafficMap[email] = &xray.ClientTraffic{
				Email: email,
				Up:    0,
				Down:  0,
			}
		}
	}

	clientTraffics := make([]*xray.ClientTraffic, 0, len(clientTrafficMap))
	for _, ct := range clientTrafficMap {
		clientTraffics = append(clientTraffics, ct)
	}

	// Inbound total traffic is already metered through the loopback SOCKS relay
	// by xray_traffic_job (matching mtproto); only per-client deltas are submitted here.
	if len(clientTraffics) > 0 {
		needRestart, _, err := j.inboundService.AddTraffic(nil, clientTraffics)
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
