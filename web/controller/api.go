package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type APIController struct {
	BaseController
	inboundController *InboundController
	Tgbot             service.Tgbot
}

func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

func (a *APIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/panel/api/inbounds")
	g.Use(a.checkLogin)

	g.GET("/list", a.getAllInbounds)
	g.GET("/get/:id", a.getSingleInbound)
	g.GET("/getClientTraffics/:email", a.getClientTraffics)
	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)
	g.POST("/clientIps/:email", a.getClientIps)
	g.POST("/clearClientIps/:email", a.clearClientIps)
	g.POST("/addClient", a.addInboundClient)
	g.POST("/:id/delClient/:clientId", a.delInboundClient)
	g.POST("/updateClient/:clientId", a.updateInboundClient)
	g.POST("/:id/resetClientTraffic/:email", a.resetClientTraffic)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/resetAllClientTraffics/:id", a.resetAllClientTraffics)
	g.POST("/delDepletedClients/:id", a.delDepletedClients)
	g.GET("/createbackup", a.createBackup)
	g.POST("/onlines", a.onlines)

	a.inboundController = NewInboundController(g)
}

func (a *APIController) getAllInbounds(c *gin.Context) {
	a.inboundController.getInbounds(c)
}

func (a *APIController) getSingleInbound(c *gin.Context) {
	a.inboundController.getInbound(c)
}

func (a *APIController) getClientTraffics(c *gin.Context) {
	a.inboundController.getClientTraffics(c)
}

func (a *APIController) addInbound(c *gin.Context) {
	a.inboundController.addInbound(c)
}

func (a *APIController) delInbound(c *gin.Context) {
	a.inboundController.delInbound(c)
}

func (a *APIController) updateInbound(c *gin.Context) {
	a.inboundController.updateInbound(c)
}

func (a *APIController) getClientIps(c *gin.Context) {
	a.inboundController.getClientIps(c)
}

func (a *APIController) clearClientIps(c *gin.Context) {
	a.inboundController.clearClientIps(c)
}

func (a *APIController) addInboundClient(c *gin.Context) {
	a.inboundController.addInboundClient(c)
}

func (a *APIController) delInboundClient(c *gin.Context) {
	a.inboundController.delInboundClient(c)
}

func (a *APIController) updateInboundClient(c *gin.Context) {
	a.inboundController.updateInboundClient(c)
}

func (a *APIController) resetClientTraffic(c *gin.Context) {
	a.inboundController.resetClientTraffic(c)
}

func (a *APIController) resetAllTraffics(c *gin.Context) {
	a.inboundController.resetAllTraffics(c)
}

func (a *APIController) resetAllClientTraffics(c *gin.Context) {
	a.inboundController.resetAllClientTraffics(c)
}

func (a *APIController) delDepletedClients(c *gin.Context) {
	a.inboundController.delDepletedClients(c)
}

func (a *APIController) createBackup(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}

func (a *APIController) onlines(c *gin.Context) {
	a.inboundController.onlines(c)
}
