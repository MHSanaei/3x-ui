package controller

import (
	"time"
	"x-ui/web/global"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type ServerController struct {
	BaseController

	serverService service.ServerService

	lastStatus        *service.Status
	lastGetStatusTime time.Time

	lastVersions        []string
	lastGetVersionsTime time.Time
}

func NewServerController(g *gin.RouterGroup) *ServerController {
	a := &ServerController{
		lastGetStatusTime: time.Now(),
	}
	a.initRouter(g)
	a.startTask()
	return a
}

func (a *ServerController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/server")

	g.Use(a.checkLogin)
	g.POST("/status", a.status)
	g.POST("/getXrayVersion", a.getXrayVersion)
	g.POST("/stopXrayService", a.stopXrayService)
	g.POST("/restartXrayService", a.restartXrayService)
	g.POST("/installXray/:version", a.installXray)
	g.POST("/logs/:count", a.getLogs)
	g.POST("/getConfigJson", a.getConfigJson)
	g.GET("/getDb", a.getDb)
}

func (a *ServerController) refreshStatus() {
	a.lastStatus = a.serverService.GetStatus(a.lastStatus)
}

func (a *ServerController) startTask() {
	webServer := global.GetWebServer()
	c := webServer.GetCron()
	c.AddFunc("@every 2s", func() {
		now := time.Now()
		if now.Sub(a.lastGetStatusTime) > time.Minute*3 {
			return
		}
		a.refreshStatus()
	})
}

func (a *ServerController) status(c *gin.Context) {
	a.lastGetStatusTime = time.Now()

	jsonObj(c, a.lastStatus, nil)
}

func (a *ServerController) getXrayVersion(c *gin.Context) {
	now := time.Now()
	if now.Sub(a.lastGetVersionsTime) <= time.Minute {
		jsonObj(c, a.lastVersions, nil)
		return
	}

	versions, err := a.serverService.GetXrayVersions()
	if err != nil {
		jsonMsg(c, I18n(c, "getVersion"), err)
		return
	}

	a.lastVersions = versions
	a.lastGetVersionsTime = time.Now()

	jsonObj(c, versions, nil)
}

func (a *ServerController) installXray(c *gin.Context) {
	version := c.Param("version")
	err := a.serverService.UpdateXray(version)
	jsonMsg(c, I18n(c, "install")+" xray", err)
}

func (a *ServerController) stopXrayService(c *gin.Context) {
	a.lastGetStatusTime = time.Now()
	err := a.serverService.StopXrayService()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonMsg(c, "Xray stoped", err)

}
func (a *ServerController) restartXrayService(c *gin.Context) {
	err := a.serverService.RestartXrayService()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonMsg(c, "Xray restarted", err)

}

func (a *ServerController) getLogs(c *gin.Context) {
	count := c.Param("count")
	logs, err := a.serverService.GetLogs(count)
	if err != nil {
		jsonMsg(c, I18n(c, "getLogs"), err)
		return
	}
	jsonObj(c, logs, nil)
}

func (a *ServerController) getConfigJson(c *gin.Context) {
	configJson, err := a.serverService.GetConfigJson()
	if err != nil {
		jsonMsg(c, I18n(c, "getLogs"), err)
		return
	}
	jsonObj(c, configJson, nil)
}

func (a *ServerController) getDb(c *gin.Context) {
	db, err := a.serverService.GetDb()
	if err != nil {
		jsonMsg(c, I18n(c, "getLogs"), err)
		return
	}
	// Set the headers for the response
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=x-ui.db")

	// Write the file contents to the response
	c.Writer.Write(db)
}
