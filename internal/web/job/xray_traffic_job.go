package job

import (
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/outbound"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/valyala/fasthttp"
)

// XrayTrafficJob collects and processes traffic statistics from Xray, updating the database and optionally informing external APIs.
type XrayTrafficJob struct {
	settingService  service.SettingService
	xrayService     service.XrayService
	inboundService  service.InboundService
	outboundService outbound.OutboundService
}

// clientStatsSnapshotMaxClients caps how many client_traffics rows the job
// ships as a full websocket snapshot per poll (same spirit as the
// controller's broadcastInboundsUpdateClientLimit). Above it, a snapshot
// would blow past the hub's payload cap and be dropped wholesale, so the job
// broadcasts only this poll's active rows and the UI leans on its 5s REST
// refetch for the rest.
const clientStatsSnapshotMaxClients = 5000

// splitMovedClientTraffics keeps the rows that actually moved bytes this poll,
// alongside the active-email list and set derived from the same pass.
//
// Xray reports a row for every known email whether or not it transferred
// anything, so on a large panel nearly every delta is zero. The database writes
// and the external-API inform consume the full slice before this point; the
// WebSocket frame only feeds the dashboard's live speed column, where an absent
// row and a zero row render identically. Broadcasting just the movers keeps that
// frame from growing with the client count — at 5k clients it was carrying about
// a megabyte of zeros every five seconds.
func splitMovedClientTraffics(clientTraffics []*xray.ClientTraffic) ([]*xray.ClientTraffic, []string, map[string]bool) {
	moved := make([]*xray.ClientTraffic, 0, len(clientTraffics))
	emails := make([]string, 0, len(clientTraffics))
	active := make(map[string]bool, len(clientTraffics))
	for _, ct := range clientTraffics {
		if ct == nil || ct.Up+ct.Down <= 0 {
			continue
		}
		moved = append(moved, ct)
		emails = append(emails, ct.Email)
		active[ct.Email] = true
	}
	return moved, emails, active
}

const externalInformTimeout = 3 * time.Second

var externalInformClient = &fasthttp.Client{
	ReadTimeout:         externalInformTimeout,
	WriteTimeout:        externalInformTimeout,
	MaxIdleConnDuration: time.Minute,
	MaxConnsPerHost:     4,
}

// NewXrayTrafficJob creates a new traffic collection job instance.
func NewXrayTrafficJob() *XrayTrafficJob {
	return new(XrayTrafficJob)
}

// Run collects traffic statistics from Xray, updates the database, and pushes
// real-time updates over WebSocket using compact delta payloads — no REST
// fallback, scales to 10k–20k+ clients per inbound.
func (j *XrayTrafficJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}
	traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		return
	}
	for _, td := range tuic.GetManager().CollectTraffic() {
		traffics = append(traffics, &xray.Traffic{
			Tag:       td.Tag,
			Up:        td.Up,
			Down:      td.Down,
			IsInbound: true,
		})
		for email, stats := range td.Clients {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email: email,
				Up:    stats.Up,
				Down:  stats.Down,
			})
		}
	}
	needRestart0, clientsDisabled, err := j.inboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add inbound traffic failed:", err)
	}
	err, needRestart1 := j.outboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add outbound traffic failed:", err)
	}
	if clientsDisabled {
		restartOnDisable, settingErr := j.settingService.GetRestartXrayOnClientDisable()
		if settingErr != nil {
			logger.Warning("get RestartXrayOnClientDisable failed:", settingErr)
		}
		if restartOnDisable {
			if err := j.xrayService.RestartXray(false); err != nil {
				logger.Warning("reconcile xray after disabling clients failed:", err)
				j.xrayService.SetToNeedRestart()
			}
		}
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
	}
	if ExternalTrafficInformEnable, err := j.settingService.GetExternalTrafficInformEnable(); ExternalTrafficInformEnable {
		j.informTrafficToExternalAPI(traffics, clientTraffics)
	} else if err != nil {
		logger.Warning("get ExternalTrafficInformEnable failed:", err)
	}
	if needRestart0 || needRestart1 {
		j.xrayService.SetToNeedRestart()
	}

	// Derive the local online set from this poll's per-email deltas rather
	// than the shared last_online column, which remote-node syncs also bump
	// and would otherwise make a client active only on a remote node appear
	// online on local inbounds.
	movedTraffics, activeEmails, deltaActive := splitMovedClientTraffics(clientTraffics)
	// When the core supports the online-stats API, union in connection-based
	// onlines. Neither signal alone covers everything: an idle-but-connected
	// client moves no bytes between polls (the delta heuristic's blind spot),
	// while a short-lived connection can close before this poll yet still show
	// in the delta. Older cores fall back to deltas alone.
	if onlineUsers, apiMode, ouErr := j.xrayService.GetOnlineUsers(); ouErr != nil {
		logger.Debug("get online users from xray api failed:", ouErr)
	} else if apiMode {
		idleOnline := make([]string, 0, len(onlineUsers))
		for _, u := range onlineUsers {
			if !deltaActive[u.Email] {
				activeEmails = append(activeEmails, u.Email)
				idleOnline = append(idleOnline, u.Email)
			}
		}
		// The traffic path only bumps last_online on a non-zero delta; keep the
		// column fresh for clients kept online purely by a live connection.
		if err := j.inboundService.BumpClientsLastOnline(idleOnline); err != nil {
			logger.Warning("bump last online for connected clients failed:", err)
		}
	}
	tuicEmails, tuicTags := tuic.GetManager().GetActiveClients(30 * time.Second)
	if len(tuicEmails) > 0 {
		for _, em := range tuicEmails {
			if !deltaActive[em] {
				activeEmails = append(activeEmails, em)
			}
		}
		if err := j.inboundService.BumpClientsLastOnline(tuicEmails); err != nil {
			logger.Warning("bump last online for tuic clients failed:", err)
		}
	}
	activeInboundTags := make([]string, 0, len(traffics)+len(tuicTags))
	for _, tr := range traffics {
		if tr != nil && tr.IsInbound && tr.Up+tr.Down > 0 {
			activeInboundTags = append(activeInboundTags, tr.Tag)
		}
	}
	activeInboundTags = append(activeInboundTags, tuicTags...)
	j.inboundService.RefreshLocalOnlineClients(activeEmails, activeInboundTags)

	if !websocket.HasClients() {
		return
	}

	// Small installs broadcast the full snapshot (see GetAllClientTraffics for
	// why deltas alone left UI rows stale). Above the threshold the snapshot
	// would be dropped by the hub's payload cap anyway, so ship this poll's
	// active rows instead and scope last-online to them; the initial full map
	// still arrives over REST.
	snapshot := true
	if total, countErr := j.inboundService.CountClientTraffics(); countErr != nil {
		logger.Warning("count client traffics for websocket failed:", countErr)
	} else if total > clientStatsSnapshotMaxClients {
		snapshot = false
	}

	var stats []*xray.ClientTraffic
	var statsErr error
	if snapshot {
		stats, statsErr = j.inboundService.GetAllClientTraffics()
	} else {
		stats, statsErr = j.inboundService.GetActiveClientTraffics(activeEmails)
	}
	if statsErr != nil {
		logger.Warning("get client traffics for websocket failed:", statsErr)
	}

	var lastOnlineMap map[string]int64
	if snapshot {
		if lastOnlineMap, err = j.inboundService.GetClientsLastOnline(); err != nil {
			logger.Warning("get clients last online failed:", err)
		}
	} else {
		lastOnlineMap = make(map[string]int64, len(stats))
		for _, ct := range stats {
			if ct != nil {
				lastOnlineMap[ct.Email] = ct.LastOnline
			}
		}
	}
	if lastOnlineMap == nil {
		lastOnlineMap = make(map[string]int64)
	}
	onlineClients := j.inboundService.GetOnlineClients()
	if onlineClients == nil {
		onlineClients = []string{}
	}
	websocket.BroadcastTraffic(map[string]any{
		"traffics":       traffics,
		"clientTraffics": movedTraffics,
		"onlineClients":  onlineClients,
		"onlineByGuid":   j.inboundService.GetOnlineClientsByGuid(),
		"activeInbounds": j.inboundService.GetActiveInboundsByGuid(),
		"lastOnlineMap":  lastOnlineMap,
	})

	clientStatsPayload := map[string]any{"snapshot": snapshot}
	if len(stats) > 0 {
		clientStatsPayload["clients"] = stats
	}
	if inboundSummary, err := j.inboundService.GetInboundsTrafficSummary(); err != nil {
		logger.Warning("get inbounds traffic summary for websocket failed:", err)
	} else if len(inboundSummary) > 0 {
		clientStatsPayload["inbounds"] = inboundSummary
	}
	if len(clientStatsPayload) > 1 {
		websocket.BroadcastClientStats(clientStatsPayload)
	}

	if updatedOutbounds, err := j.outboundService.GetOutboundsTraffic(); err == nil && updatedOutbounds != nil {
		websocket.BroadcastOutbounds(updatedOutbounds)
	} else if err != nil {
		logger.Warning("get all outbounds for websocket failed:", err)
	}
}

func (j *XrayTrafficJob) informTrafficToExternalAPI(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) {
	if len(inboundTraffics) == 0 && len(clientTraffics) == 0 {
		return
	}
	informURL, err := j.settingService.GetExternalTrafficInformURI()
	if err != nil {
		logger.Warning("get ExternalTrafficInformURI failed:", err)
		return
	}
	informURL, err = service.SanitizePublicHTTPURL(informURL, false)
	if err != nil {
		logger.Warning("ExternalTrafficInformURI blocked:", err)
		return
	}
	requestBody, err := json.Marshal(map[string]any{"clientTraffics": clientTraffics, "inboundTraffics": inboundTraffics})
	if err != nil {
		logger.Warning("parse client/inbound traffic failed:", err)
		return
	}
	request := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(request)
	request.Header.SetMethod("POST")
	request.Header.SetContentType("application/json; charset=UTF-8")
	request.SetBody(requestBody)
	request.SetRequestURI(informURL)
	request.Header.SetConnectionClose()
	response := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(response)
	if err := externalInformClient.DoTimeout(request, response, externalInformTimeout); err != nil {
		logger.Warning("POST ExternalTrafficInformURI failed:", err)
	}
}
