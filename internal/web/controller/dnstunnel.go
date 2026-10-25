package controller

import (
	"io"
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type DnsTunnelController struct {
	dnsTunnelService service.DnsTunnelService
}

func NewDnsTunnelController(g *gin.RouterGroup) *DnsTunnelController {
	a := &DnsTunnelController{}
	g.GET("/profiles", a.profiles)
	g.POST("/profiles", a.saveProfiles)
	g.GET("/detect", a.detect)
	g.POST("/command", a.newCommand)
	g.GET("/command/:token", a.reports)
	return a
}

func (a *DnsTunnelController) profiles(c *gin.Context) {
	profiles, err := a.dnsTunnelService.GetProfiles()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dnsTunnel.toasts.profiles"), err)
		return
	}
	jsonObj(c, profiles, nil)
}

func (a *DnsTunnelController) saveProfiles(c *gin.Context) {
	var profiles []service.DnsTunnelProfile
	if err := c.ShouldBindJSON(&profiles); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dnsTunnel.toasts.profiles"), err)
		return
	}
	err := a.dnsTunnelService.SaveProfiles(profiles)
	jsonMsg(c, I18nWeb(c, "pages.dnsTunnel.toasts.saved"), err)
}

func (a *DnsTunnelController) detect(c *gin.Context) {
	profiles, err := a.dnsTunnelService.DetectStormDns(c.DefaultQuery("dir", "/opt/stormdns"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dnsTunnel.toasts.detect"), err)
		return
	}
	jsonObj(c, profiles, nil)
}

func (a *DnsTunnelController) newCommand(c *gin.Context) {
	var req service.DnsTunnelCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.command"), err)
		return
	}
	cmd, err := a.dnsTunnelService.NewCommand(req, panelBaseURL(c))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.command"), err)
		return
	}
	jsonObj(c, cmd, nil)
}

func (a *DnsTunnelController) reports(c *gin.Context) {
	reports, err := a.dnsTunnelService.Reports(c.Param("token"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.expired"), err)
		return
	}
	jsonObj(c, reports, nil)
}

// DnsTunnelRunController serves the resolver test outside the session: the token in the path
// is its only key, and it can only add a report to its own job.
type DnsTunnelRunController struct {
	dnsTunnelService service.DnsTunnelService
}

func NewDnsTunnelRunController(g *gin.RouterGroup) *DnsTunnelRunController {
	a := &DnsTunnelRunController{}
	g.GET("/dnstunnel/run/:token/script.py", a.script)
	g.POST("/dnstunnel/run/:token/report", a.report)
	return a
}

func (a *DnsTunnelRunController) script(c *gin.Context) {
	token := c.Param("token")
	script, err := a.dnsTunnelService.Script(token, panelBaseURL(c)+"dnstunnel/run/"+token+"/report")
	if err != nil {
		script = "print('This command has expired: make a new one on the DNS tunnel page.')\n"
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/x-python; charset=utf-8", []byte(script))
}

func (a *DnsTunnelRunController) report(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false})
		return
	}
	reporter := service.CleanIpVisitorIp(getRemoteIp(c), c.GetHeader("CF-Connecting-IP"))
	report, err := a.dnsTunnelService.AddReport(c.Request.Context(), c.Param("token"), body, reporter)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "isp": report.Network.Isp})
}
