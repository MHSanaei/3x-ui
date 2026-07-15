package controller

import (
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/naive"

	"github.com/gin-gonic/gin"
)

type NaiveController struct{}

func NewNaiveController(g *gin.RouterGroup) *NaiveController {
	controller := &NaiveController{}
	controller.initRouter(g)
	return controller
}

func (a *NaiveController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/naive")
	g.GET("/status", a.status)
	g.GET("/releases", a.releases)
	g.GET("/logs/:tag/:rows", a.logs)
	g.POST("/install", a.install)
	g.POST("/restart-all", a.restartAll)
	g.POST("/stop-all", a.stopAll)
	g.POST("/binary/delete", a.deleteBinary)
	g.DELETE("/binary", a.deleteBinary)
}

func (a *NaiveController) status(c *gin.Context) {
	instances, err := naive.GetManager().Statuses()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{
		"installed": naive.Installed(),
		"version":   naive.InstalledVersion(),
		"instances": instances,
	}, nil)
}

func (a *NaiveController) releases(c *gin.Context) {
	releases, err := naive.FetchReleases()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, releases, nil)
}

func (a *NaiveController) install(c *gin.Context) {
	var payload struct {
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	version, err := naive.Install(payload.Version)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	manager := naive.GetManager()
	manager.StopAll()
	if err := manager.StartAll(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonObj(c, gin.H{"version": version}, nil)
}

func (a *NaiveController) restartAll(c *gin.Context) {
	manager := naive.GetManager()
	statuses, err := manager.Statuses()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	for _, status := range statuses {
		if err := manager.Restart(status.Tag); err != nil {
			jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
			return
		}
	}
	jsonObj(c, gin.H{"success": true}, nil)
}

func (a *NaiveController) deleteBinary(c *gin.Context) {
	naive.GetManager().StopAll()
	if err := naive.UninstallBinary(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"success": true}, nil)
}

func (a *NaiveController) logs(c *gin.Context) {
	tag := c.Param("tag")
	if err := naive.ValidateTag(tag); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": "invalid tag"})
		return
	}
	rows, err := strconv.Atoi(c.Param("rows"))
	if err != nil || rows <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": "invalid rows"})
		return
	}
	lines, err := naive.ReadLogLines(tag, rows)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, lines, nil)
}

func (a *NaiveController) stopAll(c *gin.Context) {
	manager := naive.GetManager()
	statuses, err := manager.Statuses()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	for _, status := range statuses {
		if err := manager.Stop(status.Tag); err != nil {
			jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
			return
		}
	}
	jsonObj(c, gin.H{"success": true}, nil)
}
