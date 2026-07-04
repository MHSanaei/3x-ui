package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// PluginController exposes the initial plugin contract under /panel/api/plugins.
// The current implementation is intentionally non-executing: it gives the UI
// and future plugin authors a stable manifest shape before runtime loading lands.
type PluginController struct {
	pluginService service.PluginService
}

func NewPluginController(g *gin.RouterGroup) *PluginController {
	a := &PluginController{}
	a.initRouter(g)
	return a
}

func (a *PluginController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/template", a.template)
	g.POST("/install", a.install)
}

func (a *PluginController) list(c *gin.Context) {
	catalog := a.pluginService.GetCatalog()
	jsonObj(c, catalog, nil)
}

func (a *PluginController) template(c *gin.Context) {
	jsonObj(c, a.pluginService.GetTemplate(), nil)
}

func (a *PluginController) install(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11<<20)
	file, header, err := c.Request.FormFile("plugin")
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.plugins.toasts.readZip"), err)
		return
	}
	defer file.Close()

	record, err := a.pluginService.InstallZip(file, header)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.plugins.toasts.install"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.plugins.toasts.installed"), record, nil)
}
