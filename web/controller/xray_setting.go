package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type XraySettingController struct {
	XraySettingService service.XraySettingService
	SettingService     service.SettingService
	InboundService     service.InboundService
	OutboundService    service.OutboundService
	XrayService        service.XrayService
}

func NewXraySettingController(g *gin.RouterGroup) *XraySettingController {
	a := &XraySettingController{}
	a.initRouter(g)
	return a
}

func (a *XraySettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xray")

	g.POST("/", a.getXraySetting)
	g.POST("/update", a.updateSetting)
	g.GET("/getXrayResult", a.getXrayResult)
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	g.POST("/warp/:action", a.warp)
	g.GET("/getOutboundsTraffic", a.getOutboundsTraffic)
	g.POST("/resetOutboundsTraffic", a.resetOutboundsTraffic)
}

func (a *XraySettingController) getXraySetting(c *gin.Context) {
	xraySetting, err := a.SettingService.GetXrayConfigTemplate()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	inboundTags, err := a.InboundService.GetInboundTags()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	xrayResponse := "{ \"xraySetting\": " + xraySetting + ", \"inboundTags\": " + inboundTags + " }"
	jsonObj(c, xrayResponse, nil)
}

func (a *XraySettingController) updateSetting(c *gin.Context) {
	xraySetting := c.PostForm("xraySetting")
	err := a.XraySettingService.SaveXraySetting(xraySetting)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
}

func (a *XraySettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.SettingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}

func (a *XraySettingController) getXrayResult(c *gin.Context) {
	jsonObj(c, a.XrayService.GetXrayResult(), nil)
}

func (a *XraySettingController) warp(c *gin.Context) {
	action := c.Param("action")
	var resp string
	var err error
	switch action {
	case "data":
		resp, err = a.XraySettingService.GetWarp()
	case "config":
		resp, err = a.XraySettingService.GetWarpConfig()
	case "reg":
		skey := c.PostForm("privateKey")
		pkey := c.PostForm("publicKey")
		resp, err = a.XraySettingService.RegWarp(skey, pkey)
	case "license":
		license := c.PostForm("license")
		resp, err = a.XraySettingService.SetWarpLicence(license)
	}

	jsonObj(c, resp, err)
}

func (a *XraySettingController) getOutboundsTraffic(c *gin.Context) {
	outboundsTraffic, err := a.OutboundService.GetOutboundsTraffic()
	if err != nil {
		jsonMsg(c, "Error getting traffics", err)
		return
	}
	jsonObj(c, outboundsTraffic, nil)
}

func (a *XraySettingController) resetOutboundsTraffic(c *gin.Context) {
	tag := c.PostForm("tag")
	err := a.OutboundService.ResetOutboundTraffic(tag)
	if err != nil {
		jsonMsg(c, "Error in reset outbound traffics", err)
		return
	}
	jsonObj(c, "", nil)
}
