package controller

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"

	"github.com/gin-gonic/gin"
)

type NodeController struct {
	nodeService service.NodeService
}

func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{}
	a.initRouter(g)
	return a
}

func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)

	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)

	g.POST("/test", a.test)
	g.POST("/probe/:id", a.probe)
	g.GET("/history/:id/:metric/:bucket", a.history)
}

func (a *NodeController) list(c *gin.Context) {
	nodes, err := a.nodeService.GetAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.list"), err)
		return
	}
	jsonObj(c, nodes, nil)
}

func (a *NodeController) get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	jsonObj(c, n, nil)
}

func (a *NodeController) add(c *gin.Context) {
	n := &model.Node{}
	if err := c.ShouldBind(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return
	}
	if err := a.nodeService.Create(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.add"), n, nil)
}

func (a *NodeController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n := &model.Node{}
	if err := c.ShouldBind(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	if err := a.nodeService.Update(id, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
}

func (a *NodeController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	if err := a.nodeService.Delete(id); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.delete"), nil)
}

func (a *NodeController) setEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	if err := a.nodeService.SetEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
}

func (a *NodeController) test(c *gin.Context) {
	n := &model.Node{}
	if err := c.ShouldBind(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}
	if n.Scheme == "" {
		n.Scheme = "https"
	}
	if n.BasePath == "" {
		n.BasePath = "/"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	patch, err := a.nodeService.Probe(ctx, n)
	jsonObj(c, patch.ToUI(err == nil), nil)
}

func (a *NodeController) probe(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	patch, probeErr := a.nodeService.Probe(ctx, n)
	if probeErr != nil {
		patch.Status = "offline"
	} else {
		patch.Status = "online"
	}
	_ = a.nodeService.UpdateHeartbeat(id, patch)
	jsonObj(c, patch.ToUI(probeErr == nil), nil)
}

func (a *NodeController) history(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	metric := c.Param("metric")
	if !slices.Contains(service.NodeMetricKeys, metric) {
		jsonMsg(c, "invalid metric", fmt.Errorf("unknown metric"))
		return
	}
	bucket, err := strconv.Atoi(c.Param("bucket"))
	if err != nil || bucket <= 0 || !allowedHistoryBuckets[bucket] {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	jsonObj(c, a.nodeService.AggregateNodeMetric(id, metric, bucket, 60), nil)
}
