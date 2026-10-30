package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// RealityWatchController sets the spare targets of REALITY inbounds and switches them.
type RealityWatchController struct {
	realityWatchService service.RealityWatchService
}

func NewRealityWatchController(g *gin.RouterGroup) *RealityWatchController {
	a := &RealityWatchController{}
	g.GET("/list", a.list)
	g.GET("/status/:id", a.status)
	g.POST("/save/:id", a.save)
	g.POST("/switch/:id", a.switchTarget)
	g.POST("/test", a.test)
	return a
}

type realityWatchTargetRequest struct {
	Target string `json:"target"`
}

func (a *RealityWatchController) list(c *gin.Context) {
	jsonObj(c, a.realityWatchService.List(), nil)
}

func (a *RealityWatchController) status(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.failed"), err)
		return
	}
	jsonObj(c, a.realityWatchService.Status(id), nil)
}

func (a *RealityWatchController) save(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.failed"), err)
		return
	}
	var entry service.RealityWatchEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.failed"), err)
		return
	}
	if err := a.realityWatchService.SaveConfig(id, entry); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.failed"), err)
		return
	}
	jsonObj(c, a.realityWatchService.Status(id), nil)
}

// switchTarget moves the inbound to the given target after the server check and a real
// REALITY connection through it; a target failing either is refused.
func (a *RealityWatchController) switchTarget(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.switchFailed"), err)
		return
	}
	var req realityWatchTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.switchFailed"), err)
		return
	}
	ev, err := a.realityWatchService.SwitchTo(c.Request.Context(), id, req.Target)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.switchFailed"), err)
		return
	}
	jsonObj(c, ev, nil)
}

func (a *RealityWatchController) test(c *gin.Context) {
	var req realityWatchTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.failed"), err)
		return
	}
	res, err := a.realityWatchService.TestTarget(c.Request.Context(), req.Target)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.realityWatch.toasts.failed"), err)
		return
	}
	jsonObj(c, res, nil)
}
