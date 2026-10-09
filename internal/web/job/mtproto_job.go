package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// MtprotoJob reconciles the running mtg sidecar processes against the enabled
// mtproto inbounds in the database, restarts any that crashed, and folds the
// per-client traffic scraped from each mtg /stats endpoint into the usual client
// and inbound traffic accounting.
type MtprotoJob struct {
	inboundService service.InboundService
}

// NewMtprotoJob creates a new mtproto reconcile/traffic job instance.
func NewMtprotoJob() *MtprotoJob {
	return new(MtprotoJob)
}

// Run reconciles desired mtproto inbounds with running mtg processes and records
// per-client traffic deltas and online status.
func (j *MtprotoJob) Run() {
	desired, err := j.inboundService.DesiredMtprotoInstances()
	if err != nil {
		logger.Warning("mtproto job: get desired instances failed:", err)
		return
	}

	routedTags := make(map[string]bool)
	activeTags := make([]string, 0, len(desired))
	for _, inst := range desired {
		activeTags = append(activeTags, inst.Tag)
		if inst.RouteThroughXray {
			routedTags[inst.Tag] = true
		}
	}

	mgr := mtproto.GetManager()
	mgr.Reconcile(desired)

	deltas, onlineEmails := mgr.CollectTraffic()
	j.recordTraffic(deltas, onlineEmails, routedTags, activeTags)
}

// recordTraffic folds one mtg scrape into client and inbound traffic and the
// local online set.
func (j *MtprotoJob) recordTraffic(deltas []mtproto.Traffic, onlineEmails []string, routedTags map[string]bool, activeTags []string) {
	// A routed inbound's total is already metered through the Xray bridge by
	// xray_traffic_job, so only non-routed inbounds are rolled up here; per-client
	// deltas are always kept, since the bridge cannot tell mtproto users apart.
	clientTraffics := make([]*xray.ClientTraffic, 0, len(deltas))
	inboundUp := make(map[string]int64)
	inboundDown := make(map[string]int64)
	for _, d := range deltas {
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: d.Email,
			Up:    d.Up,
			Down:  d.Down,
		})
		if !routedTags[d.Tag] {
			inboundUp[d.Tag] += d.Up
			inboundDown[d.Tag] += d.Down
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
		if _, _, err := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("mtproto job: add traffic failed:", err)
		}
	}

	// The traffic path only bumps last_online on a non-zero delta; keep it fresh
	// for clients held online by a live mtg connection alone, as the Xray poll does.
	moved := make(map[string]bool, len(deltas))
	for _, d := range deltas {
		moved[d.Email] = true
	}
	idleOnline := make([]string, 0, len(onlineEmails))
	for _, email := range onlineEmails {
		if !moved[email] {
			idleOnline = append(idleOnline, email)
		}
	}
	if err := j.inboundService.BumpClientsLastOnline(idleOnline); err != nil {
		logger.Warning("mtproto job: bump last online for connected clients failed:", err)
	}

	j.inboundService.RefreshLocalOnlineClients(onlineEmails, activeTags)
}
