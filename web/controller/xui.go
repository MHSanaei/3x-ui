package controller

import (
	"github.com/gin-gonic/gin"
)

// XUIController is the main controller for the X-UI panel, managing sub-controllers.
type XUIController struct {
	BaseController

	settingController     *SettingController
	xraySettingController *XraySettingController
	nodeController        *NodeController
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

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/settings", a.settings)
	g.GET("/xray", a.xraySettings)
	g.GET("/nodes", a.nodes)
	g.GET("/clients", a.clients)
	g.GET("/hosts", a.hosts)

	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
	a.nodeController = NewNodeController(g.Group("/node"))
	
	// Register client and host controllers directly under /panel (not /panel/api)
	NewClientController(g.Group("/client"))
	NewHostController(g.Group("/host"))
	NewClientHWIDController(g.Group("/client")) // Register HWID controller under /panel/client/hwid
}

// index renders the main panel index page.
func (a *XUIController) index(c *gin.Context) {
	html(c, "index.html", "pages.index.title", nil)
}

// inbounds renders the inbounds management page.
func (a *XUIController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "pages.inbounds.title", nil)
}

// settings renders the settings management page.
func (a *XUIController) settings(c *gin.Context) {
	html(c, "settings.html", "pages.settings.title", nil)
}

// xraySettings renders the Xray settings page.
func (a *XUIController) xraySettings(c *gin.Context) {
	html(c, "xray.html", "pages.xray.title", nil)
}

// nodes renders the nodes management page (multi-node mode).
func (a *XUIController) nodes(c *gin.Context) {
	html(c, "nodes.html", "pages.nodes.title", nil)
}

// clients renders the clients management page.
func (a *XUIController) clients(c *gin.Context) {
	html(c, "clients.html", "pages.clients.title", nil)
}

// hosts renders the hosts management page (multi-node mode).
func (a *XUIController) hosts(c *gin.Context) {
	html(c, "hosts.html", "pages.hosts.title", nil)
}
