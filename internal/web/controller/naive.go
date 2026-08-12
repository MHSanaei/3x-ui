package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/naive"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

const maxNaiveLogRows = 10000

type NaiveController struct {
	settingService service.SettingService
}

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
		"installed":  naive.Installed(),
		"version":    naive.InstalledVersion(),
		"releaseTag": naive.InstalledReleaseTag(),
		"instances":  instances,
	}, nil)
}

func (a *NaiveController) releases(c *gin.Context) {
	releases, err := naive.FetchReleases(a.settingService.NewProxiedHTTPClient(15 * time.Second))
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
	if err := naive.ValidateVersion(payload.Version); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	manager := naive.GetManager()
	if err := manager.StopAll(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	version, err := naive.Install(a.settingService.NewProxiedHTTPClient(2*time.Minute), payload.Version)
	if err != nil {
		_ = manager.StartAll()
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := manager.StartAll(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"version": version}, nil)
}

func (a *NaiveController) restartAll(c *gin.Context) {
	if err := naive.GetManager().RestartAll(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"success": true}, nil)
}

func (a *NaiveController) deleteBinary(c *gin.Context) {
	if err := naive.GetManager().StopAll(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := naive.UninstallBinary(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := naive.RemoveAllLogs(); err != nil {
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
	if err != nil || rows <= 0 || rows > maxNaiveLogRows {
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
	if err := naive.GetManager().StopAll(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"success": true}, nil)
}
