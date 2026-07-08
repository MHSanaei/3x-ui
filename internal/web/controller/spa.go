package controller

import (
	"net/http"
	"path"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// XUIController is the main controller for the X-UI panel, serving the SPA shell.
type XUIController struct {
	BaseController
}

// NewXUIController creates a new XUIController and initializes its routes.
func NewXUIController(g *gin.RouterGroup) *XUIController {
	a := &XUIController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the main panel routes and initializes sub-controllers.
//
// The HTML routes all hand the same single-page-app shell (index.html) to the
// browser; React Router takes over and renders the correct page from the URL.
// The /panel/api, /panel/setting, /panel/xray sub-routers register POST/JSON
// endpoints on different paths and stay untouched by the shell handler.
func (a *XUIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/panel")
	g.Use(a.checkLogin)
	g.Use(middleware.CSRFMiddleware())

	g.GET("/", a.panelSPA)
	g.GET("/inbounds", a.panelSPA)
	g.GET("/clients", a.panelSPA)
	g.GET("/groups", a.panelSPA)
	g.GET("/nodes", a.panelSPA)
	g.GET("/settings", a.panelSPA)
	g.GET("/xray", a.panelSPA)
	g.GET("/outbound", a.panelSPA)
	g.GET("/routing", a.panelSPA)
	g.GET("/api-docs", a.panelSPA)

	// SPA pages built by Vite don't have a server-rendered <meta name="csrf-token">,
	// so they fetch the session token via this endpoint at startup and replay it
	// on subsequent unsafe requests.
	g.GET("/csrf-token", a.csrfToken)
}

// panelSPA serves the React SPA shell. Every GET under /panel/ that isn't an
// API endpoint returns the same index.html — React Router reads the URL and
// mounts the matching page on the client.
func (a *XUIController) panelSPA(c *gin.Context) {
	serveDistPage(c, "index.html")
}

// HandleNoRoutePanelSPA serves the React shell for client-side routes that were
// not explicitly registered in Gin. It intentionally runs from engine.NoRoute
// instead of a /panel/*path wildcard so explicit JSON/API routes keep their
// normal routing semantics.
func (a *XUIController) HandleNoRoutePanelSPA(c *gin.Context) bool {
	if !isPanelSPAFallbackRequest(c) {
		return false
	}

	if !session.IsLogin(c) {
		if isAjax(c) {
			pureJsonMsg(c, http.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
		} else {
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		}
		c.Abort()
		return true
	}

	a.panelSPA(c)
	return true
}

func isPanelSPAFallbackRequest(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	if !acceptsHTML(c.GetHeader("Accept")) {
		return false
	}

	basePath := c.GetString("base_path")
	if basePath == "" {
		basePath = "/"
	}
	panelPath := strings.TrimRight(basePath, "/") + "/panel"

	reqPath := c.Request.URL.Path
	if reqPath != panelPath && !strings.HasPrefix(reqPath, panelPath+"/") {
		return false
	}

	if reqPath == panelPath+"/csrf-token" || strings.HasPrefix(reqPath, panelPath+"/csrf-token/") {
		return false
	}
	if reqPath == panelPath+"/api" || strings.HasPrefix(reqPath, panelPath+"/api/") {
		return false
	}
	if isStaticAssetPath(reqPath) {
		return false
	}
	return true
}

var staticAssetExts = map[string]struct{}{
	".js": {}, ".mjs": {}, ".cjs": {}, ".css": {}, ".map": {}, ".json": {},
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".svg": {}, ".ico": {},
	".webp": {}, ".avif": {}, ".woff": {}, ".woff2": {}, ".ttf": {}, ".eot": {},
	".otf": {}, ".wasm": {}, ".txt": {}, ".xml": {}, ".webmanifest": {},
}

func isStaticAssetPath(reqPath string) bool {
	ext := strings.ToLower(path.Ext(reqPath))
	if ext == "" {
		return false
	}
	_, ok := staticAssetExts[ext]
	return ok
}

func acceptsHTML(accept string) bool {
	if accept == "" {
		return true
	}
	accept = strings.ToLower(accept)
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

// csrfToken returns the session CSRF token to authenticated SPA clients.
// The endpoint is GET (a safe method) so it bypasses CSRFMiddleware itself,
// but checkLogin still gates the response — anonymous callers get 401/redirect.
func (a *XUIController) csrfToken(c *gin.Context) {
	token, err := session.EnsureCSRFToken(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, entity.Msg{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, entity.Msg{Success: true, Obj: token})
}
