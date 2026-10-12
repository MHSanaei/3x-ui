package job

import (
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const defaultTuicSpeedSampleInterval = 10 * time.Second

type TuicJob struct {
	inboundService  service.InboundService
	runMu           sync.Mutex
	lastSpeedSample time.Time
}

func NewTuicJob() *TuicJob {
	return new(TuicJob)
}

func (j *TuicJob) Run() {
	j.runMu.Lock()
	defer j.runMu.Unlock()

	tuicJournalMu.Lock()
	journalErr := j.replayTuicJournal()
	tuicJournalMu.Unlock()
	if journalErr != nil {
		logger.Warning("tuic job: recover traffic journal failed:", journalErr)
	}

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

	clientTraffics := aggregateTuicClientTraffic(clientDeltas, onlineEmails)
	sampledAt := time.Now()
	sampleInterval := tuicSpeedSampleInterval(j.lastSpeedSample, sampledAt)

	// Inbound total traffic is already metered through the loopback SOCKS relay
	// by xray_traffic_job (matching mtproto); only per-client deltas are submitted here.
	persisted := true
	if len(clientTraffics) > 0 {
		needRestart, _, err := j.inboundService.AddTraffic(nil, clientTraffics)
		if err != nil {
			logger.Warning("tuic job: add traffic failed:", err)
			mgr.RequeueClientTraffic(clientDeltas)
			persisted = false
		} else if needRestart {
			if desired, err := j.inboundService.DesiredTuicInstances(); err == nil {
				mgr.Reconcile(desired)
			}
		}
	}
	if persisted {
		websocket.BroadcastTraffic(tuicSpeedPayload(clientTraffics, sampleInterval))
		j.lastSpeedSample = sampledAt
	}

	if len(onlineEmails) > 0 {
		if err := j.inboundService.BumpClientsLastOnline(onlineEmails); err != nil {
			logger.Warning("tuic job: bump last online for tuic clients failed:", err)
		}
	}

	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}

func tuicSpeedSampleInterval(previous, current time.Time) time.Duration {
	if previous.IsZero() || !current.After(previous) {
		return defaultTuicSpeedSampleInterval
	}
	return current.Sub(previous)
}

func tuicSpeedPayload(clientTraffics []*xray.ClientTraffic, sampleInterval time.Duration) map[string]any {
	intervalMs := sampleInterval.Milliseconds()
	if intervalMs < 1 {
		intervalMs = 1
	}
	return map[string]any{
		"clientTraffics":          clientTraffics,
		"clientTrafficSource":     "tuic",
		"clientTrafficIntervalMs": intervalMs,
	}
}

// FlushStoppedTraffic persists counters drained when the TUIC manager stops its
// listeners. Call it after scheduled jobs have stopped and before the traffic
// writer shuts down.
func (j *TuicJob) FlushStoppedTraffic() error {
	return j.flushTuicJournal()
}

func aggregateTuicClientTraffic(clientDeltas []tuic.ClientTrafficDelta, onlineEmails []string) []*xray.ClientTraffic {
	clientTrafficMap := make(map[string]*xray.ClientTraffic, len(clientDeltas)+len(onlineEmails))
	for _, cd := range clientDeltas {
		key := cd.Email
		if cd.TrafficID > 0 {
			key = fmt.Sprintf("traffic:%d", cd.TrafficID)
		}
		if cd.TrafficID == 0 && cd.InboundID > 0 && cd.UUID != "" {
			key = fmt.Sprintf("tuic:%d:%s", cd.InboundID, cd.UUID)
		}
		traffic := clientTrafficMap[key]
		if traffic == nil {
			traffic = &xray.ClientTraffic{Email: cd.Email, TuicTrafficID: cd.TrafficID, TuicUUID: cd.UUID, TuicInboundId: cd.InboundID}
			clientTrafficMap[key] = traffic
		}
		traffic.Up += cd.Up
		traffic.Down += cd.Down
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
	return clientTraffics
}
