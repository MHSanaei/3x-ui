package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/psiphon"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/integration"
)

// PsiphonController handles REST endpoints for managing Psiphon outbounds.
type PsiphonController struct {
	BaseController
	psiphonService integration.PsiphonService
	xrayService    service.XrayService
}

func NewPsiphonController(g *gin.RouterGroup) *PsiphonController {
	a := &PsiphonController{}
	a.initRouter(g)
	return a
}

func (a *PsiphonController) initRouter(g *gin.RouterGroup) {
	g.GET("/countries", a.getCountries)
	g.GET("/instances", a.getInstances)
	g.POST("/instances", a.addInstances)
	g.DELETE("/instances/:id", a.deleteInstance)
	g.POST("/instances/:id/toggle", a.toggleInstance)
	g.POST("/instances/:id/ping", a.pingInstance)
	g.POST("/ping-all", a.pingAll)
	g.GET("/config", a.getConfig)
	g.POST("/config", a.saveConfig)
}

func (a *PsiphonController) getCountries(c *gin.Context) {
	countries, err := a.psiphonService.GetCountries()
	jsonObj(c, countries, err)
}

func (a *PsiphonController) getInstances(c *gin.Context) {
	instances, err := a.psiphonService.GetInstances()
	jsonObj(c, instances, err)
}

func (a *PsiphonController) addInstances(c *gin.Context) {
	var req psiphon.BulkAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		countries := c.PostFormArray("countries")
		if len(countries) == 0 {
			if single := c.PostForm("country"); single != "" {
				countries = []string{single}
			}
		}
		req.Countries = countries
	}
	created, err := a.psiphonService.AddCountries(req.Countries)
	if err == nil && a.xrayService.IsXrayRunning() {
		_ = a.xrayService.RestartXray(false)
	}
	jsonObj(c, created, err)
}

func (a *PsiphonController) deleteInstance(c *gin.Context) {
	id := c.Param("id")
	err := a.psiphonService.DeleteInstance(id)
	if err == nil && a.xrayService.IsXrayRunning() {
		_ = a.xrayService.RestartXray(false)
	}
	jsonObj(c, nil, err)
}

func (a *PsiphonController) toggleInstance(c *gin.Context) {
	id := c.Param("id")
	inst, err := a.psiphonService.ToggleInstance(id)
	if err == nil && a.xrayService.IsXrayRunning() {
		_ = a.xrayService.RestartXray(false)
	}
	jsonObj(c, inst, err)
}

func (a *PsiphonController) pingInstance(c *gin.Context) {
	id := c.Param("id")
	res, err := a.psiphonService.TestLatency(id)
	jsonObj(c, res, err)
}

func (a *PsiphonController) pingAll(c *gin.Context) {
	results, err := a.psiphonService.TestAll()
	jsonObj(c, results, err)
}

func (a *PsiphonController) getConfig(c *gin.Context) {
	cfg, err := a.psiphonService.GetConfig()
	jsonObj(c, cfg, err)
}

func (a *PsiphonController) saveConfig(c *gin.Context) {
	var req psiphon.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.ConfigJSON = c.PostForm("configJson")
	}
	err := a.psiphonService.SaveConfig(req.ConfigJSON)
	jsonObj(c, nil, err)
}
