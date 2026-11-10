package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

type ExitPoolController struct {
	service service.ExitPoolService
}

func NewExitPoolController(g *gin.RouterGroup) *ExitPoolController {
	a := &ExitPoolController{}
	g = g.Group("/exit-pool")
	g.GET("", a.state)
	g.POST("/sync", a.sync)
	g.POST("/delete/:subId", a.delete)
	// The embedded console currently needs only reads and form/API POSTs.
	// Do not use gin.Any here: that would unnecessarily expose CONNECT, TRACE,
	// PUT, PATCH, DELETE and OPTIONS through the authenticated reverse proxy.
	g.GET("/console/*path", a.console)
	g.HEAD("/console/*path", a.console)
	g.POST("/console/*path", a.console)
	return a
}

func (a *ExitPoolController) state(c *gin.Context) {
	state, err := a.service.State(session.GetLoginUser(c).Id, c.Request.Host)
	jsonObj(c, state, err)
}

func (a *ExitPoolController) console(c *gin.Context) {
	token, err := session.EnsureCSRFToken(c)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	// xnvgatebox ships a self-contained console with inline JS/CSS. It is only
	// served here after 3x-ui authentication and may only be framed by 3x-ui.
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'")
	if err := a.service.ProxyConsole(c.Writer, c.Request, c.Param("path"), token); err != nil {
		c.String(http.StatusBadGateway, "%s", err.Error())
	}
}

func (a *ExitPoolController) sync(c *gin.Context) {
	var request service.ExitPoolSyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonObj(c, nil, err)
		return
	}
	group, err := a.service.Sync(session.GetLoginUser(c).Id, c.Request.Host, request)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	websocketNotifyXrayAndClients()
	jsonObj(c, group, nil)
}

func (a *ExitPoolController) delete(c *gin.Context) {
	if err := a.service.Delete(c.Param("subId")); err != nil {
		jsonObj(c, nil, err)
		return
	}
	websocketNotifyXrayAndClients()
	jsonMsg(c, "Exit-pool subscription group deleted", nil)
}

func websocketNotifyXrayAndClients() {
	notifyClientsChanged()
}
