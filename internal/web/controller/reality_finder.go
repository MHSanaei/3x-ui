package controller

import (
	"io"
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type RealityFinderController struct {
	finder service.RealityFinderService
}

func NewRealityFinderController(g *gin.RouterGroup) *RealityFinderController {
	a := &RealityFinderController{}
	g.POST("/command", a.newCommand)
	g.GET("/command/:token", a.reports)
	return a
}

// requestBaseURL is the panel as the caller reached it, ending in the base path.
func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + c.GetString("base_path")
}

func (a *RealityFinderController) newCommand(c *gin.Context) {
	var req service.RealityFinderCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.form.finderCommandError"), err)
		return
	}
	cmd, err := a.finder.NewCommand(req, requestBaseURL(c))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.form.finderCommandError"), err)
		return
	}
	jsonObj(c, cmd, nil)
}

func (a *RealityFinderController) reports(c *gin.Context) {
	reports, err := a.finder.Reports(c.Param("token"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.form.finderExpired"), err)
		return
	}
	jsonObj(c, reports, nil)
}

// RealityFinderRunController serves the one-day command outside the session: the token in
// the path is its only key, and it can only add a report to its own job.
type RealityFinderRunController struct {
	finder service.RealityFinderService
}

func NewRealityFinderRunController(g *gin.RouterGroup) *RealityFinderRunController {
	a := &RealityFinderRunController{}
	g.GET("/realityfinder/run/:token/script.py", a.script)
	g.POST("/realityfinder/run/:token/report", a.report)
	return a
}

func (a *RealityFinderRunController) script(c *gin.Context) {
	token := c.Param("token")
	script, err := a.finder.Script(token, requestBaseURL(c)+"realityfinder/run/"+token+"/report")
	if err != nil {
		script = "print('This command has expired: make a new one in the REALITY target scanner.')\n"
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/x-python; charset=utf-8", []byte(script))
}

func (a *RealityFinderRunController) report(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false})
		return
	}
	report, err := a.finder.AddReport(c.Param("token"), body, getRemoteIp(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "targets": len(report.Targets)})
}
