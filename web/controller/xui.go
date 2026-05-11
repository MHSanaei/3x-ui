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
func (a *XUIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/panel")
	g.Use(a.checkLogin)
	g.Use(middleware.CSRFMiddleware())

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/nodes", a.nodes)
	g.GET("/settings", a.settings)
	g.GET("/xray", a.xraySettings)

	// SPA pages built by Vite don't have a server-rendered <meta name="csrf-token">,
	// so they fetch the session token via this endpoint at startup and replay it
	// on subsequent unsafe requests through axios.
	g.GET("/csrf-token", a.csrfToken)

	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
}

// All four panel pages now serve the Vue 3 builds from web/dist/
// instead of rendering the legacy Go templates. Each handler is a
// thin wrapper around serveDistPage so the basePath injection +
// no-cache headers stay centralised.

// index renders the main panel index page.
func (a *XUIController) index(c *gin.Context) {
	serveDistPage(c, "index.html")
}

// inbounds renders the inbounds management page.
func (a *XUIController) inbounds(c *gin.Context) {
	serveDistPage(c, "inbounds.html")
}

// nodes renders the multi-panel nodes management page.
func (a *XUIController) nodes(c *gin.Context) {
	serveDistPage(c, "nodes.html")
}

// settings renders the settings management page.
func (a *XUIController) settings(c *gin.Context) {
	serveDistPage(c, "settings.html")
}

// xraySettings renders the Xray settings page.
func (a *XUIController) xraySettings(c *gin.Context) {
	serveDistPage(c, "xray.html")
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
