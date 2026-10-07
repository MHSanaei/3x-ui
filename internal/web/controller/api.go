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
	setAuthenticated := func(scope string) {
		if u, err := a.userService.GetFirstUser(); err == nil {
			session.SetAPIAuthUser(c, u)
		}
		c.Set("api_authed", true)
		c.Set("api_token_scope", scope)
	}
	matchedScope := ""
	if after, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
		if row, matched := a.apiTokenService.MatchToken(after); matched {
			matchedScope = row.Scope
		}
	}
	// Verified mTLS keeps its historical node-sync identity. Only an explicit
	// node-admin bearer elevates it to software-update authority.
	if c.Request.TLS != nil && len(c.Request.TLS.VerifiedChains) > 0 {
		scope := model.ApiScopeNodeSync
		if matchedScope == model.ApiScopeNodeAdmin {
			scope = model.ApiScopeNodeAdmin
		}
		setAuthenticated(scope)
		c.Next()
		return
	}
	if matchedScope != "" {
		setAuthenticated(matchedScope)
		c.Next()
		return
	}
	if !session.IsLogin(c) {
		// A presented Bearer token is not an anonymous scan: return 401 so
		// callers can distinguish a bad/disabled token from a wrong base path
		// (NoRoute still 404s). XHR keeps 401; bare unauthenticated stays 404.
		authHdr := c.GetHeader("Authorization")
		if strings.HasPrefix(authHdr, "Bearer ") || c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			c.AbortWithStatus(http.StatusUnauthorized)
		} else {
			c.AbortWithStatus(http.StatusNotFound)
		}
		return
	}
	c.Next()
}

// monitorScopeAllow exposes only status/metrics routes without sensitive data.
// Keys are route patterns relative to /panel/api.
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

// nodeSyncScopeAllow is the node-sync route/method allowlist relative to
// /panel/api; Gin patterns prevent concrete parameters broadening authority.
var nodeSyncScopeAllow = map[string]map[string]struct{}{
	"/server/status":               {http.MethodGet: {}},
	"/inbounds/list":               {http.MethodGet: {}},
	"/inbounds/add":                {http.MethodPost: {}},
	"/inbounds/del/:id":            {http.MethodPost: {}},
	"/inbounds/update/:id":         {http.MethodPost: {}},
	"/inbounds/:id/subSortIndex":   {http.MethodPost: {}},
	"/clients/add":                 {http.MethodPost: {}},
	"/clients/del/:email":          {http.MethodPost: {}},
	"/clients/:email/detach":       {http.MethodPost: {}},
	"/clients/update/:email":       {http.MethodPost: {}},
	"/server/restartXrayService":   {http.MethodPost: {}},
	"/server/getWebCertFiles":      {http.MethodGet: {}},
	"/server/descendants":          {http.MethodGet: {}},
	"/clients/resetTraffic/:email": {http.MethodPost: {}},
	"/clients/bulkResetTraffic":    {http.MethodPost: {}},
	"/inbounds/resetAllTraffics":   {http.MethodPost: {}},
	"/inbounds/:id/resetTraffic":   {http.MethodPost: {}},
	"/clients/onlinesByGuid":       {http.MethodPost: {}},
	"/clients/onlines":             {http.MethodPost: {}},
	"/clients/lastOnline":          {http.MethodPost: {}},
	"/clients/activeInbounds":      {http.MethodPost: {}},
	"/inbounds/pushClientTraffics": {http.MethodPost: {}},
	"/server/clientIps":            {http.MethodGet: {}, http.MethodPost: {}},
	"/clients/clientIpsByGuid":     {http.MethodPost: {}},
	"/hosts/list":                  {http.MethodGet: {}},
}

var nodeAdminScopeAllow = map[string]map[string]struct{}{
	"/server/updatePanel": {http.MethodPost: {}},
}

func scopeAllows(allow map[string]map[string]struct{}, path, method string) bool {
	methods, exists := allow[path]
	if !exists {
		return false
	}
	_, exists = methods[method]
	return exists
}

// enforceTokenScope applies explicit allowlists to restricted API tokens.
// Admin tokens and session-login users retain their existing behavior.
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
		if scopeAllows(nodeSyncScopeAllow, rel, c.Request.Method) {
			c.Next()
			return
		}
	case model.ApiScopeNodeAdmin:
		if scopeAllows(nodeSyncScopeAllow, rel, c.Request.Method) || scopeAllows(nodeAdminScopeAllow, rel, c.Request.Method) {
			c.Next()
			return
		}
	default:
		deny()
		return
	}
	deny()
}

func relAPIPath(fullPath string) string {
	const marker = "/panel/api"
	_, after, ok := strings.Cut(fullPath, marker)
	if !ok {
		return ""
	}
	return after
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

	// Subscription balancers — client-side balancers for the JSON sub output
	NewSubBalancerController(api)

	// Extra routes
	api.POST("/backuptotgbot", a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
