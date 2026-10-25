package controller

import (
	"io"
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type CleanIpController struct {
	cleanIpService service.CleanIpService
}

func NewCleanIpController(g *gin.RouterGroup) *CleanIpController {
	a := &CleanIpController{}
	a.initRouter(g)
	return a
}

func (a *CleanIpController) initRouter(g *gin.RouterGroup) {
	g.GET("/network", a.network)
	g.GET("/cdns", a.cdns)
	g.POST("/command", a.newCommand)
	g.GET("/command/:token", a.reports)
	g.POST("/verify", a.verify)
}

// network reports the ISP of whoever opened the page: the scan runs in their
// browser, so that is the ISP its IPs are good for.
func (a *CleanIpController) cdns(c *gin.Context) {
	jsonObj(c, a.cleanIpService.Cdns(), nil)
}

func (a *CleanIpController) network(c *gin.Context) {
	visitor := service.CleanIpVisitorIp(getRemoteIp(c), c.GetHeader("CF-Connecting-IP"))
	network, err := a.cleanIpService.LookupNetwork(c.Request.Context(), visitor)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.network"), err)
		return
	}
	jsonObj(c, network, nil)
}

// panelBaseURL is the panel as the caller reached it, ending in the base path.
func panelBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + c.GetString("base_path")
}

func (a *CleanIpController) newCommand(c *gin.Context) {
	var req service.CleanIpCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.command"), err)
		return
	}
	cmd, err := a.cleanIpService.NewCommand(req, panelBaseURL(c))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.command"), err)
		return
	}
	jsonObj(c, cmd, nil)
}

// verify checks from this server that the IPs the browser found really serve the domain.
func (a *CleanIpController) verify(c *gin.Context) {
	var req service.CleanIpVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.verify"), err)
		return
	}
	verdicts, err := a.cleanIpService.VerifyIps(c.Request.Context(), req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.verify"), err)
		return
	}
	jsonObj(c, verdicts, nil)
}

func (a *CleanIpController) reports(c *gin.Context) {
	reports, err := a.cleanIpService.Reports(c.Param("token"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.expired"), err)
		return
	}
	jsonObj(c, reports, nil)
}

// CleanIpRunController serves the one-day command outside the session: the token in
// the path is its only key, and it can only add a report to its own job.
type CleanIpRunController struct {
	cleanIpService service.CleanIpService
}

func NewCleanIpRunController(g *gin.RouterGroup) *CleanIpRunController {
	a := &CleanIpRunController{}
	g.GET("/cleanip/run/:token/script.py", a.script)
	g.POST("/cleanip/run/:token/report", a.report)
	return a
}

func (a *CleanIpRunController) script(c *gin.Context) {
	token := c.Param("token")
	script, err := a.cleanIpService.Script(token, panelBaseURL(c)+"cleanip/run/"+token+"/report")
	if err != nil {
		script = "print('This command has expired: make a new one on the Clean IP scanner page.')\n"
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/x-python; charset=utf-8", []byte(script))
}

func (a *CleanIpRunController) report(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false})
		return
	}
	reporter := service.CleanIpVisitorIp(getRemoteIp(c), c.GetHeader("CF-Connecting-IP"))
	report, err := a.cleanIpService.AddReport(c.Request.Context(), c.Param("token"), body, reporter)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "isp": report.Network.Isp})
}
