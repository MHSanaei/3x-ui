package controller

import (
	"github.com/gin-gonic/gin"
)
type APIController struct {
	BaseController

	inboundController *InboundController
	settingController *SettingController
}

func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

func (a *APIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xui/API/inbounds")
	g.Use(a.checkLogin)

	g.GET("/", a.inbounds)
	g.GET("/get/:id", a.inbound)
	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)

	
	a.inboundController = NewInboundController(g)
}


func (a *APIController) inbounds(c *gin.Context) {
	a.inboundController.getInbounds(c)
}
func (a *APIController) inbound(c *gin.Context) {
	a.inboundController.getInbound(c)
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
