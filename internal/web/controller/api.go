package controller

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// APIController handles the main API routes for the 3x-ui panel, including inbounds and server management.
type APIController struct {
	BaseController
	inboundController     *InboundController
	serverController      *ServerController
	nodeController        *NodeController
	hostController        *HostController
	settingController     *SettingController
	xraySettingController *XraySettingController
	userService           panel.UserService
	apiTokenService       panel.ApiTokenService
	Tgbot                 tgbot.Tgbot
}

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

func (a *APIController) checkAPIAuth(c *gin.Context) {
	// A verified client certificate (a completed mTLS handshake) authenticates
	// the caller, equivalent to a valid bearer token. api_authed must be set so
	// the CSRF middleware lets cert-authed mutations through.
	if c.Request.TLS != nil && len(c.Request.TLS.VerifiedChains) > 0 {
		if u, err := a.userService.GetFirstUser(); err == nil {
			session.SetAPIAuthUser(c, u)
		}
		c.Set("api_authed", true)
		c.Set("api_token_scope", model.ApiScopeNodeSync)
		c.Next()
		return
	}
	auth := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		tok := after
		if row, ok := a.apiTokenService.MatchToken(tok); ok {
			if u, err := a.userService.GetFirstUser(); err == nil {
				session.SetAPIAuthUser(c, u)
			}
			c.Set("api_authed", true)
			c.Set("api_token_scope", row.Scope)
			c.Next()
			return
		}
	}
	if !session.IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			c.AbortWithStatus(http.StatusUnauthorized)
		} else {
			c.AbortWithStatus(http.StatusNotFound)
		}
		return
	}
	c.Next()
}

// monitorScopeAllow is the explicit capability allowlist for the `monitor`
// token scope: pure status/metrics endpoints that expose no DB, config, certs,
// keys, client identities/links, or IPs. Keys are route patterns RELATIVE to
// /panel/api (matched via c.FullPath so the dynamic panel base path does not
// matter). This is an allowlist by design.
var monitorScopeAllow = map[string]struct{}{
	"/server/status":                              {},
	"/server/cpuHistory/:bucket":                  {},
	"/server/history/:metric/:bucket":             {},
	"/server/xrayMetricsState":                    {},
	"/server/xrayMetricsHistory/:metric/:bucket":  {},
	"/server/xrayObservatory":                     {},
	"/server/xrayObservatoryHistory/:tag/:bucket": {},
	"/server/getXrayVersion":                      {},
	"/server/getPanelUpdateInfo":                  {},
	"/nodes/history/:id/:metric/:bucket":          {},
}

// nodeSyncScopeAllow is the complete master-to-worker capability matrix. Keys
// are registered Gin route patterns relative to /panel/api, so concrete path
// parameters cannot broaden a token's authority. Methods are explicit.
var nodeSyncScopeAllow = map[string]map[string]struct{}{
	"/server/status":               {http.MethodGet: {}},
	"/inbounds/list":               {http.MethodGet: {}},
	"/inbounds/add":                {http.MethodPost: {}},
	"/inbounds/del/:id":            {http.MethodPost: {}},
	"/inbounds/update/:id":         {http.MethodPost: {}},
	"/clients/add":                 {http.MethodPost: {}},
	"/clients/del/:email":          {http.MethodPost: {}},
	"/clients/:email/detach":       {http.MethodPost: {}},
	"/clients/update/:email":       {http.MethodPost: {}},
	"/server/restartXrayService":   {http.MethodPost: {}},
	"/server/getWebCertFiles":      {http.MethodGet: {}},
	"/server/descendants":          {http.MethodGet: {}},
	"/clients/resetTraffic/:email": {http.MethodPost: {}},
	"/inbounds/resetAllTraffics":   {http.MethodPost: {}},
	"/inbounds/:id/resetTraffic":   {http.MethodPost: {}},
	"/clients/onlinesByGuid":       {http.MethodPost: {}},
	"/clients/onlines":             {http.MethodPost: {}},
	"/clients/lastOnline":          {http.MethodPost: {}},
	"/inbounds/pushClientTraffics": {http.MethodPost: {}},
	"/server/clientIps":            {http.MethodGet: {}, http.MethodPost: {}},
	"/clients/clientIpsByGuid":     {http.MethodPost: {}},
	"/hosts/list":                  {http.MethodGet: {}},
}

// enforceTokenScope authorizes the request for the token's scope. admin is
// unrestricted; monitor and node-sync are explicit allowlists. Session-login UI
// users carry no api_token_scope and are unaffected.
func (a *APIController) enforceTokenScope(c *gin.Context) {
	scopeVal, ok := c.Get("api_token_scope")
	if !ok {
		c.Next()
		return
	}
	scope, _ := scopeVal.(string)
	if scope == model.ApiScopeAdmin {
		c.Next()
		return
	}
	deny := func() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"msg":     "this API token is not permitted to access this endpoint",
		})
	}
	rel := relAPIPath(c.FullPath())
	switch scope {
	case model.ApiScopeMonitor:
		if _, allowed := monitorScopeAllow[rel]; allowed && (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) {
			c.Next()
			return
		}
	case model.ApiScopeNodeSync:
		if methods, allowed := nodeSyncScopeAllow[rel]; allowed {
			if _, allowedMethod := methods[c.Request.Method]; allowedMethod {
				c.Next()
				return
			}
		}
	default:
		deny()
		return
	}
	deny()
}

func relAPIPath(fullPath string) string {
	const marker = "/panel/api"
	i := strings.Index(fullPath, marker)
	if i < 0 {
		return ""
	}
	return fullPath[i+len(marker):]
}

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup) {
	// Main API group
	api := g.Group("/panel/api")
	api.Use(a.checkAPIAuth)
	api.Use(a.enforceTokenScope)
	// Decode + verify the node config envelope (zstd + X-Config-Sha256) and
	// advertise support, before CSRF/handlers read the body.
	api.Use(middleware.ConfigEnvelopeMiddleware())
	api.Use(middleware.CSRFMiddleware())

	api.GET("/openapi.json", ServeOpenAPISpec)

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)

	clients := api.Group("/clients")
	NewClientController(clients)
	NewGroupController(clients)

	// Server API
	server := api.Group("/server")
	a.serverController = NewServerController(server)

	// Nodes API — multi-panel management
	nodes := api.Group("/nodes")
	a.nodeController = NewNodeController(nodes)

	// Hosts API — per-inbound override endpoints for subscription links
	hosts := api.Group("/hosts")
	a.hostController = NewHostController(hosts)

	// Settings + Xray config management live under the API surface too, so the
	// same API token drives them. Paths are /panel/api/setting/* and
	// /panel/api/xray/*.
	a.settingController = NewSettingController(api)
	a.xraySettingController = NewXraySettingController(api)

	// Extra routes
	api.POST("/backuptotgbot", a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
