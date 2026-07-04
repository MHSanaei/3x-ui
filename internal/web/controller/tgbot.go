package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type TgBotController struct {
	BaseController
}

func NewTgBotController(g *gin.RouterGroup) *TgBotController {
	a := &TgBotController{}
	a.initRouter(g)
	return a
}

func (a *TgBotController) initRouter(g *gin.RouterGroup) {
	gg := g.Group("/tgbot")
	gg.GET("/status", a.getStatus)
}

func (a *TgBotController) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj":     "ok",
	})
}
