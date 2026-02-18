package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"
)

// ClientCenterController manages centralized client profiles and inbound assignments.
type ClientCenterController struct {
	service service.ClientCenterService
}

func NewClientCenterController(g *gin.RouterGroup) *ClientCenterController {
	a := &ClientCenterController{}
	a.initRouter(g)
	return a
}

func (a *ClientCenterController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/inbounds", a.inbounds)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
}

type clientCenterUpsertForm struct {
	Name        string `form:"name"`
	EmailPrefix string `form:"emailPrefix"`
	TotalGB     int64  `form:"totalGB"`
	ExpiryTime  int64  `form:"expiryTime"`
	LimitIP     int    `form:"limitIp"`
	Enable      bool   `form:"enable"`
	Comment     string `form:"comment"`
	InboundIds  []int  `form:"inboundIds"`
}

func (a *ClientCenterController) list(c *gin.Context) {
	user := session.GetLoginUser(c)
	items, err := a.service.ListMasterClients(user.Id)
	if err != nil {
		jsonMsg(c, "get client center list", err)
		return
	}
	jsonObj(c, items, nil)
}

func (a *ClientCenterController) inbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	items, err := a.service.ListInbounds(user.Id)
	if err != nil {
		jsonMsg(c, "get inbounds", err)
		return
	}
	jsonObj(c, items, nil)
}

func (a *ClientCenterController) add(c *gin.Context) {
	form := &clientCenterUpsertForm{}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, "invalid client payload", err)
		return
	}
	user := session.GetLoginUser(c)
	item, err := a.service.CreateMasterClient(user.Id, service.UpsertMasterClientInput{
		Name:        form.Name,
		EmailPrefix: form.EmailPrefix,
		TotalGB:     form.TotalGB,
		ExpiryTime:  form.ExpiryTime,
		LimitIP:     form.LimitIP,
		Enable:      form.Enable,
		Comment:     form.Comment,
		InboundIds:  form.InboundIds,
	})
	if err != nil {
		jsonMsg(c, "create master client", err)
		return
	}
	jsonMsgObj(c, "master client created", item, nil)
}

func (a *ClientCenterController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid client id", err)
		return
	}
	form := &clientCenterUpsertForm{}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, "invalid client payload", err)
		return
	}
	user := session.GetLoginUser(c)
	item, err := a.service.UpdateMasterClient(user.Id, id, service.UpsertMasterClientInput{
		Name:        form.Name,
		EmailPrefix: form.EmailPrefix,
		TotalGB:     form.TotalGB,
		ExpiryTime:  form.ExpiryTime,
		LimitIP:     form.LimitIP,
		Enable:      form.Enable,
		Comment:     form.Comment,
		InboundIds:  form.InboundIds,
	})
	if err != nil {
		jsonMsg(c, "update master client", err)
		return
	}
	jsonMsgObj(c, "master client updated", item, nil)
}

func (a *ClientCenterController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "invalid client id", err)
		return
	}
	user := session.GetLoginUser(c)
	err = a.service.DeleteMasterClient(user.Id, id)
	if err != nil {
		jsonMsg(c, "delete master client", err)
		return
	}
	jsonMsg(c, "master client deleted", nil)
}
