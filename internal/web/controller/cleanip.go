package controller

import (
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
}

// network reports the ISP of whoever opened the page: the scan runs in their
// browser, so that is the ISP its IPs are good for.
func (a *CleanIpController) network(c *gin.Context) {
	network, err := a.cleanIpService.LookupNetwork(c.Request.Context(), getRemoteIp(c))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.cleanIp.toasts.network"), err)
		return
	}
	jsonObj(c, network, nil)
}
