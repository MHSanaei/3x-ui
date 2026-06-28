package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type LinkController struct {
	linkService service.LinkService
}

func NewLinkController(g *gin.RouterGroup) *LinkController {
	a := &LinkController{}
	a.initRouter(g)
	return a
}

func (a *LinkController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)

	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)
	g.POST("/reorder", a.reorder)
	g.POST("/assign", a.assign)
	g.POST("/bulk/setEnable", a.bulkSetEnable)
	g.POST("/bulk/del", a.bulkDel)
}

func (a *LinkController) list(c *gin.Context) {
	links, err := a.linkService.GetLinks()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.list"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *LinkController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	link, err := a.linkService.GetLink(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonObj(c, link, nil)
}

func (a *LinkController) add(c *gin.Context) {
	link, ok := middleware.BindAndValidate[model.Link](c)
	if !ok {
		return
	}
	created, err := a.linkService.AddLink(link)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.add"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.links.toasts.add"), created, nil)
}

func (a *LinkController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	link, ok := middleware.BindAndValidate[model.Link](c)
	if !ok {
		return
	}
	updated, err := a.linkService.UpdateLink(id, link)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.links.toasts.update"), updated, nil)
}

func (a *LinkController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	if err := a.linkService.DeleteLink(id); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.links.toasts.delete"), nil)
}

func (a *LinkController) setEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	if err := a.linkService.SetLinkEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), nil)
}

func (a *LinkController) reorder(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids" form:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	if err := a.linkService.ReorderLinks(req.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), nil)
}

func (a *LinkController) assign(c *gin.Context) {
	var req struct {
		LinkIds []int    `json:"linkIds" form:"linkIds"`
		Emails  []string `json:"emails" form:"emails"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.assign"), err)
		return
	}
	result, err := a.linkService.AssignLinks(req.LinkIds, req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.assign"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.links.toasts.assign"), result, nil)
	notifyClientsChanged()
}

func (a *LinkController) bulkSetEnable(c *gin.Context) {
	var req struct {
		Ids    []int `json:"ids" form:"ids"`
		Enable bool  `json:"enable" form:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	if err := a.linkService.SetLinksEnable(req.Ids, req.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.links.toasts.update"), nil)
}

func (a *LinkController) bulkDel(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids" form:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.delete"), err)
		return
	}
	if err := a.linkService.DeleteLinks(req.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.links.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.links.toasts.delete"), nil)
}
