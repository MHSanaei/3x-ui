package controller

import (
	"net/http"

	"github.com/gary/dune/internal/web/entity"
	"github.com/gary/dune/internal/web/middleware"
	"github.com/gary/dune/internal/web/session"

	"github.com/gin-gonic/gin"
)

// DuneController is the main controller for the Dune panel, serving the SPA shell.
type DuneController struct {
	BaseController
}

// NewDuneController creates a new DuneController and initializes its routes.
func NewDuneController(g *gin.RouterGroup) *DuneController {
	a := &DuneController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the main panel routes and initializes sub-controllers.
//
// The HTML routes all hand the same single-page-app shell (index.html) to the
// browser; React Router takes over and renders the correct page from the URL.
// The /panel/api, /panel/setting, /panel/xray sub-routers register POST/JSON
// endpoints on different paths and stay untouched by the shell handler.
func (a *DuneController) initRouter(g *gin.RouterGroup) {
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
}

// panelSPA serves the React SPA shell. Every GET under /panel/ that isn't an
// API endpoint returns the same index.html — React Router reads the URL and
// mounts the matching page on the client.
func (a *DuneController) panelSPA(c *gin.Context) {
	serveDistPage(c, "index.html")
}

// csrfToken returns the session CSRF token to authenticated SPA clients.
// The endpoint is GET (a safe method) so it bypasses CSRFMiddleware itself,
// but checkLogin still gates the response — anonymous callers get 401/redirect.
func (a *DuneController) csrfToken(c *gin.Context) {
	token, err := session.EnsureCSRFToken(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, entity.Msg{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, entity.Msg{Success: true, Obj: token})
}
