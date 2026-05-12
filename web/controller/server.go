package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/entity"
	"github.com/mhsanaei/3x-ui/v3/web/global"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"

	"github.com/gin-gonic/gin"
)

var filenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)

// ServerController handles server management and status-related operations.
type ServerController struct {
	BaseController

	serverService      service.ServerService
	settingService     service.SettingService
	panelService       service.PanelService
	xrayMetricsService service.XrayMetricsService

	lastStatus *service.Status

	lastVersions        []string
	lastGetVersionsTime int64 // unix seconds
}

// NewServerController creates a new ServerController, initializes routes, and starts background tasks.
func NewServerController(g *gin.RouterGroup) *ServerController {
	a := &ServerController{}
	a.initRouter(g)
	a.startTask()
	return a
}

// initRouter sets up the routes for server status, Xray management, and utility endpoints.
func (a *ServerController) initRouter(g *gin.RouterGroup) {

	g.GET("/status", a.status)
	g.GET("/cpuHistory/:bucket", a.getCpuHistoryBucket)
	g.GET("/history/:metric/:bucket", a.getMetricHistoryBucket)
	g.GET("/xrayMetricsState", a.getXrayMetricsState)
	g.GET("/xrayMetricsHistory/:metric/:bucket", a.getXrayMetricsHistoryBucket)
	g.GET("/xrayObservatory", a.getXrayObservatory)
	g.GET("/xrayObservatoryHistory/:tag/:bucket", a.getXrayObservatoryHistoryBucket)
	g.GET("/getXrayVersion", a.getXrayVersion)
	g.GET("/getPanelUpdateInfo", a.getPanelUpdateInfo)
	g.GET("/getConfigJson", a.getConfigJson)
	g.GET("/getDb", a.getDb)
	g.GET("/getNewUUID", a.getNewUUID)
	g.GET("/getNewX25519Cert", a.getNewX25519Cert)
	g.GET("/getNewmldsa65", a.getNewmldsa65)
	g.GET("/getNewmlkem768", a.getNewmlkem768)
	g.GET("/getNewVlessEnc", a.getNewVlessEnc)

	g.POST("/stopXrayService", a.stopXrayService)
	g.POST("/restartXrayService", a.restartXrayService)
	g.POST("/installXray/:version", a.installXray)
	g.POST("/updatePanel", a.updatePanel)
	g.POST("/updateGeofile", a.updateGeofile)
	g.POST("/updateGeofile/:fileName", a.updateGeofile)
	g.POST("/logs/:count", a.getLogs)
	g.POST("/xraylogs/:count", a.getXrayLogs)
	g.POST("/importDB", a.importDB)
	g.POST("/getNewEchCert", a.getNewEchCert)
}

// refreshStatus updates the cached server status and collects time-series
// metrics. CPU/Mem/Net/Online/Load are all written in one call so the
// SystemHistoryModal's tabs share an identical x-axis.
func (a *ServerController) refreshStatus() {
	a.lastStatus = a.serverService.GetStatus(a.lastStatus)
	if a.lastStatus != nil {
		now := time.Now()
		a.serverService.AppendStatusSample(now, a.lastStatus)
		a.xrayMetricsService.Sample(now)
		// Broadcast status update via WebSocket
		websocket.BroadcastStatus(a.lastStatus)
	}
}

// startTask initiates background tasks for continuous status monitoring.
func (a *ServerController) startTask() {
	webServer := global.GetWebServer()
	c := webServer.GetCron()
	c.AddFunc("@every 2s", func() {
		// Always refresh to keep CPU history collected continuously.
		// Sampling is lightweight and capped to ~6 hours in memory.
		a.refreshStatus()
	})
}

// status returns the current server status information.
func (a *ServerController) status(c *gin.Context) { jsonObj(c, a.lastStatus, nil) }

// allowedHistoryBuckets is the bucket-second whitelist shared by both
// /cpuHistory/:bucket and /history/:metric/:bucket. Restricting it
// prevents callers from triggering arbitrary aggregation work and keeps
// the front-end's bucket selector self-documenting.
var allowedHistoryBuckets = map[int]bool{
	2:   true, // Real-time view
	30:  true, // 30s intervals
	60:  true, // 1m intervals
	120: true, // 2m intervals
	180: true, // 3m intervals
	300: true, // 5m intervals
}

// getCpuHistoryBucket retrieves aggregated CPU usage history based on the specified time bucket.
// Kept for back-compat; new callers should use /history/cpu/:bucket which
// returns {"t","v"} (uniform across all metrics) instead of {"t","cpu"}.
func (a *ServerController) getCpuHistoryBucket(c *gin.Context) {
	bucketStr := c.Param("bucket")
	bucket, err := strconv.Atoi(bucketStr)
	if err != nil || bucket <= 0 {
		jsonMsg(c, "invalid bucket", fmt.Errorf("bad bucket"))
		return
	}
	if !allowedHistoryBuckets[bucket] {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	points := a.serverService.AggregateCpuHistory(bucket, 60)
	jsonObj(c, points, nil)
}

// getMetricHistoryBucket returns up to 60 buckets of history for a single
// system metric (cpu, mem, netUp, netDown, online, load1/5/15). The
// SystemHistoryModal calls one endpoint per active tab.
func (a *ServerController) getMetricHistoryBucket(c *gin.Context) {
	metric := c.Param("metric")
	if !slices.Contains(service.SystemMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return
	}
	bucket, err := strconv.Atoi(c.Param("bucket"))
	if err != nil || bucket <= 0 || !allowedHistoryBuckets[bucket] {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	jsonObj(c, a.serverService.AggregateSystemMetric(metric, bucket, 60), nil)
}

func (a *ServerController) getXrayMetricsState(c *gin.Context) {
	jsonObj(c, a.xrayMetricsService.State(), nil)
}

func (a *ServerController) getXrayMetricsHistoryBucket(c *gin.Context) {
	metric := c.Param("metric")
	if !slices.Contains(service.XrayMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return
	}
	bucket, err := strconv.Atoi(c.Param("bucket"))
	if err != nil || bucket <= 0 || !allowedHistoryBuckets[bucket] {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	jsonObj(c, a.xrayMetricsService.AggregateMetric(metric, bucket, 60), nil)
}

func (a *ServerController) getXrayObservatory(c *gin.Context) {
	jsonObj(c, a.xrayMetricsService.ObservatorySnapshot(), nil)
}

func (a *ServerController) getXrayObservatoryHistoryBucket(c *gin.Context) {
	tag := c.Param("tag")
	if !a.xrayMetricsService.HasObservatoryTag(tag) {
		jsonMsg(c, "invalid tag", fmt.Errorf("unknown observatory tag"))
		return
	}
	bucket, err := strconv.Atoi(c.Param("bucket"))
	if err != nil || bucket <= 0 || !allowedHistoryBuckets[bucket] {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	jsonObj(c, a.xrayMetricsService.AggregateObservatory(tag, bucket, 60), nil)
}

func (a *ServerController) getXrayVersion(c *gin.Context) {
	const cacheTTLSeconds = 15 * 60

	now := time.Now().Unix()
	if a.lastVersions != nil && now-a.lastGetVersionsTime <= cacheTTLSeconds {
		jsonObj(c, a.lastVersions, nil)
		return
	}

	versions, err := a.serverService.GetXrayVersions()
	if err != nil {
		if a.lastVersions != nil {
			logger.Warning("getXrayVersion failed; serving cached list:", err)
			jsonObj(c, a.lastVersions, nil)
			return
		}
		jsonMsg(c, I18nWeb(c, "getVersion"), err)
		return
	}

	a.lastVersions = versions
	a.lastGetVersionsTime = now

	jsonObj(c, versions, nil)
}

// getPanelUpdateInfo retrieves the current and latest panel version.
func (a *ServerController) getPanelUpdateInfo(c *gin.Context) {
	info, err := a.panelService.GetUpdateInfo()
	if err != nil {
		logger.Debug("panel update check failed:", err)
		c.JSON(http.StatusOK, entity.Msg{Success: false})
		return
	}
	jsonObj(c, info, nil)
}

// installXray installs or updates Xray to the specified version.
func (a *ServerController) installXray(c *gin.Context) {
	version := c.Param("version")
	err := a.serverService.UpdateXray(version)
	jsonMsg(c, I18nWeb(c, "pages.index.xraySwitchVersionPopover"), err)
}

// updatePanel starts a panel self-update to the latest release.
func (a *ServerController) updatePanel(c *gin.Context) {
	err := a.panelService.StartUpdate()
	jsonMsg(c, I18nWeb(c, "pages.index.panelUpdateStartedPopover"), err)
}

// updateGeofile updates the specified geo file for Xray.
func (a *ServerController) updateGeofile(c *gin.Context) {
	fileName := c.Param("fileName")

	// Validate the filename for security (prevent path traversal attacks)
	if fileName != "" && !a.serverService.IsValidGeofileName(fileName) {
		jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"),
			fmt.Errorf("invalid filename: contains unsafe characters or path traversal patterns"))
		return
	}

	err := a.serverService.UpdateGeofile(fileName)
	jsonMsg(c, I18nWeb(c, "pages.index.geofileUpdatePopover"), err)
}

// stopXrayService stops the Xray service.
func (a *ServerController) stopXrayService(c *gin.Context) {
	err := a.serverService.StopXrayService()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.xray.stopError"), err)
		websocket.BroadcastXrayState("error", err.Error())
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.stopSuccess"), err)
	websocket.BroadcastXrayState("stop", "")
	websocket.BroadcastNotification(
		I18nWeb(c, "pages.xray.stopSuccess"),
		"Xray service has been stopped",
		"warning",
	)
}

// restartXrayService restarts the Xray service.
func (a *ServerController) restartXrayService(c *gin.Context) {
	err := a.serverService.RestartXrayService()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.xray.restartError"), err)
		websocket.BroadcastXrayState("error", err.Error())
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.xray.restartSuccess"), err)
	websocket.BroadcastXrayState("running", "")
	websocket.BroadcastNotification(
		I18nWeb(c, "pages.xray.restartSuccess"),
		"Xray service has been restarted successfully",
		"success",
	)
}

// getLogs retrieves the application logs based on count, level, and syslog filters.
func (a *ServerController) getLogs(c *gin.Context) {
	count := c.Param("count")
	level := c.PostForm("level")
	syslog := c.PostForm("syslog")
	logs := a.serverService.GetLogs(count, level, syslog)
	jsonObj(c, logs, nil)
}

// getXrayLogs retrieves Xray logs with filtering options for direct, blocked, and proxy traffic.
func (a *ServerController) getXrayLogs(c *gin.Context) {
	count := c.Param("count")
	filter := c.PostForm("filter")
	showDirect := c.PostForm("showDirect")
	showBlocked := c.PostForm("showBlocked")
	showProxy := c.PostForm("showProxy")

	var freedoms []string
	var blackholes []string

	//getting tags for freedom and blackhole outbounds
	config, err := a.settingService.GetDefaultXrayConfig()
	if err == nil && config != nil {
		if cfgMap, ok := config.(map[string]any); ok {
			if outbounds, ok := cfgMap["outbounds"].([]any); ok {
				for _, outbound := range outbounds {
					if obMap, ok := outbound.(map[string]any); ok {
						switch obMap["protocol"] {
						case "freedom":
							if tag, ok := obMap["tag"].(string); ok {
								freedoms = append(freedoms, tag)
							}
						case "blackhole":
							if tag, ok := obMap["tag"].(string); ok {
								blackholes = append(blackholes, tag)
							}
						}
					}
				}
			}
		}
	}

	if len(freedoms) == 0 {
		freedoms = []string{"direct"}
	}
	if len(blackholes) == 0 {
		blackholes = []string{"blocked"}
	}

	logs := a.serverService.GetXrayLogs(count, filter, showDirect, showBlocked, showProxy, freedoms, blackholes)
	jsonObj(c, logs, nil)
}

// getConfigJson retrieves the Xray configuration as JSON.
func (a *ServerController) getConfigJson(c *gin.Context) {
	configJson, err := a.serverService.GetConfigJson()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.getConfigError"), err)
		return
	}
	jsonObj(c, configJson, nil)
}

// getDb downloads the database file.
func (a *ServerController) getDb(c *gin.Context) {
	db, err := a.serverService.GetDb()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.getDatabaseError"), err)
		return
	}

	filename := "x-ui.db"

	if !isValidFilename(filename) {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid filename"))
		return
	}

	// Set the headers for the response
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	// Write the file contents to the response
	c.Writer.Write(db)
}

func isValidFilename(filename string) bool {
	// Validate that the filename only contains allowed characters
	return filenameRegex.MatchString(filename)
}

// importDB imports a database file and restarts the Xray service.
func (a *ServerController) importDB(c *gin.Context) {
	// Get the file from the request body
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.readDatabaseError"), err)
		return
	}
	defer file.Close()
	err = a.serverService.ImportDB(file)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.importDatabaseError"), err)
		return
	}
	jsonObj(c, I18nWeb(c, "pages.index.importDatabaseSuccess"), nil)
}

// getNewX25519Cert generates a new X25519 certificate.
func (a *ServerController) getNewX25519Cert(c *gin.Context) {
	cert, err := a.serverService.GetNewX25519Cert()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewX25519CertError"), err)
		return
	}
	jsonObj(c, cert, nil)
}

// getNewmldsa65 generates a new ML-DSA-65 key.
func (a *ServerController) getNewmldsa65(c *gin.Context) {
	cert, err := a.serverService.GetNewmldsa65()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewmldsa65Error"), err)
		return
	}
	jsonObj(c, cert, nil)
}

// getNewEchCert generates a new ECH certificate for the given SNI.
func (a *ServerController) getNewEchCert(c *gin.Context) {
	sni := c.PostForm("sni")
	cert, err := a.serverService.GetNewEchCert(sni)
	if err != nil {
		jsonMsg(c, "get ech certificate", err)
		return
	}
	jsonObj(c, cert, nil)
}

// getNewVlessEnc generates a new VLESS encryption key.
func (a *ServerController) getNewVlessEnc(c *gin.Context) {
	out, err := a.serverService.GetNewVlessEnc()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.getNewVlessEncError"), err)
		return
	}
	jsonObj(c, out, nil)
}

// getNewUUID generates a new UUID.
func (a *ServerController) getNewUUID(c *gin.Context) {
	uuidResp, err := a.serverService.GetNewUUID()
	if err != nil {
		jsonMsg(c, "Failed to generate UUID", err)
		return
	}

	jsonObj(c, uuidResp, nil)
}

// getNewmlkem768 generates a new ML-KEM-768 key.
func (a *ServerController) getNewmlkem768(c *gin.Context) {
	out, err := a.serverService.GetNewmlkem768()
	if err != nil {
		jsonMsg(c, "Failed to generate mlkem768 keys", err)
		return
	}
	jsonObj(c, out, nil)
}
