package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/web/entity"
	"github.com/mhsanaei/3x-ui/v3/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// XUIController is the main controller for the X-UI panel, managing sub-controllers.
type XUIController struct {
	BaseController

	settingController     *SettingController
	xraySettingController *XraySettingController
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
	g.GET("/api-docs", a.panelSPA)

	// SPA pages built by Vite don't have a server-rendered <meta name="csrf-token">,
	// so they fetch the session token via this endpoint at startup and replay it
	// on subsequent unsafe requests through axios.
	g.GET("/csrf-token", a.csrfToken)

	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
}

// panelSPA serves the React SPA shell. Every GET under /panel/ that isn't an
// API endpoint returns the same index.html — React Router reads the URL and
// mounts the matching page on the client.
func (a *XUIController) panelSPA(c *gin.Context) {
	serveDistPage(c, "index.html")
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
