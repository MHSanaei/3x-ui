package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

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
	g.GET("/get/:groupId", a.get)
	g.GET("/byInbound/:inboundId", a.byInbound)
	g.GET("/tags", a.tags)

	g.POST("/add", a.add)
	g.POST("/update/:groupId", a.update)
	g.POST("/del/:groupId", a.del)
	g.POST("/setEnable/:groupId", a.setEnable)
	g.POST("/reorder", a.reorder)
	g.POST("/bulk/add", a.add)
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
	groupId := c.Param("groupId")
	h, err := a.hostService.GetHostGroup(groupId)
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
	req, ok := middleware.BindJSONAndValidate[entity.HostGroup](c)
	if !ok {
		return
	}
	created, err := a.hostService.AddHostGroup(req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.add"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.hosts.toasts.add"), created, nil)
}

func (a *HostController) update(c *gin.Context) {
	groupId := c.Param("groupId")
	req, ok := middleware.BindJSONAndValidate[entity.HostGroup](c)
	if !ok {
		return
	}
	updated, err := a.hostService.UpdateHostGroup(groupId, req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.hosts.toasts.update"), updated, nil)
}

func (a *HostController) del(c *gin.Context) {
	groupId := c.Param("groupId")
	if err := a.hostService.DeleteHostGroup(groupId); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), nil)
}

func (a *HostController) setEnable(c *gin.Context) {
	groupId := c.Param("groupId")
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	if err := a.hostService.SetHostGroupEnable(groupId, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), nil)
}

func (a *HostController) reorder(c *gin.Context) {
	var req struct {
		Ids []string `json:"ids" form:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	if err := a.hostService.ReorderHostGroups(req.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), nil)
}

func (a *HostController) bulkSetEnable(c *gin.Context) {
	var req struct {
		Ids    []string `json:"ids" form:"ids"`
		Enable bool     `json:"enable" form:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	if err := a.hostService.SetHostsGroupEnable(req.Ids, req.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.update"), nil)
}

func (a *HostController) bulkDel(c *gin.Context) {
	var req struct {
		Ids []string `json:"ids" form:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), err)
		return
	}
	if err := a.hostService.DeleteHostsGroup(req.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.delete"), nil)
}
