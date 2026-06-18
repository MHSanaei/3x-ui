package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// HostController exposes CRUD + ordering for Host override endpoints under
// /panel/api/hosts. Thin HTTP layer over HostService; mirrors NodeController.
type HostController struct {
	hostService service.HostService
}

func NewHostController(g *gin.RouterGroup) *HostController {
	a := &HostController{}
	a.initRouter(g)
	return a
}

func (a *HostController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)
	g.GET("/byInbound/:inboundId", a.byInbound)
	g.GET("/tags", a.tags)

	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)
	g.POST("/reorder", a.reorder)
	g.POST("/bulk/setEnable", a.bulkSetEnable)
	g.POST("/bulk/del", a.bulkDel)
}

func (a *HostController) list(c *gin.Context) {
	hosts, err := a.hostService.GetHosts()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.list"), err)
		return
	}
	jsonObj(c, hosts, nil)
}

func (a *HostController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	h, err := a.hostService.GetHost(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.obtain"), err)
		return
	}
	jsonObj(c, h, nil)
}

func (a *HostController) byInbound(c *gin.Context) {
	inboundId, err := strconv.Atoi(c.Param("inboundId"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	hosts, err := a.hostService.GetHostsByInbound(inboundId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.list"), err)
		return
	}
	jsonObj(c, hosts, nil)
}

func (a *HostController) tags(c *gin.Context) {
	tags, err := a.hostService.GetAllTags()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.list"), err)
		return
	}
	jsonObj(c, tags, nil)
}

func (a *HostController) add(c *gin.Context) {
	h, ok := middleware.BindAndValidate[model.Host](c)
	if !ok {
		return
	}
	created, err := a.hostService.AddHost(h)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.add"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.hosts.toasts.add"), created, nil)
}

func (a *HostController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	h, ok := middleware.BindAndValidate[model.Host](c)
	if !ok {
		return
	}
	updated, err := a.hostService.UpdateHost(id, h)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.hosts.toasts.update"), updated, nil)
}

func (a *HostController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	if err := a.hostService.DeleteHost(id); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), nil)
}

func (a *HostController) setEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	if err := a.hostService.SetHostEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), nil)
}

func (a *HostController) reorder(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids" form:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	if err := a.hostService.ReorderHosts(req.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), nil)
}

func (a *HostController) bulkSetEnable(c *gin.Context) {
	var req struct {
		Ids    []int `json:"ids" form:"ids"`
		Enable bool  `json:"enable" form:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	if err := a.hostService.SetHostsEnable(req.Ids, req.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), nil)
}

func (a *HostController) bulkDel(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids" form:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), err)
		return
	}
	if err := a.hostService.DeleteHosts(req.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), nil)
}
