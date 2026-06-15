package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type NodeController struct {
	nodeService service.NodeService
	xrayService service.XrayService
}

func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{}
	a.initRouter(g)
	return a
}

func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:id", a.get)
	g.GET("/webCert/:id", a.webCert)

	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)

	g.POST("/test", a.test)
	g.POST("/certFingerprint", a.certFingerprint)
	g.POST("/inbounds", a.inbounds)
	g.POST("/probe/:id", a.probe)
	g.POST("/updatePanel", a.updatePanel)
	g.GET("/history/:id/:metric/:bucket", a.history)
	g.POST("/mtls/ca", a.mtlsCa)
	g.POST("/mtls/trustCA", a.setMtlsTrustCA)
}

// mtlsCa returns this panel's node-auth CA certificate (public) to paste into a
// node's mTLS trust setting. It lazily mints the CA + master client cert on
// first call.
func (a *NodeController) mtlsCa(c *gin.Context) {
	caCert, err := a.nodeService.NodeMtlsCaCert()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	jsonObj(c, gin.H{"caCert": caCert}, nil)
}

// setMtlsTrustCA stores the CA this panel trusts for incoming node-API client
// certificates (this panel acting as a node). An empty value disables it.
// Applied on the next panel restart.
func (a *NodeController) setMtlsTrustCA(c *gin.Context) {
	var req struct {
		CaCert string `json:"caCert" form:"caCert"`
	}
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.saveMtls"), err)
		return
	}
	if err := a.nodeService.SetNodeMtlsTrustCA(req.CaCert); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.saveMtls"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.saveMtls"), nil)
}

func (a *NodeController) list(c *gin.Context) {
	nodes, err := a.nodeService.GetNodeTree()
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

// webCert returns the node's own web TLS certificate/key file paths so the
// inbound form's "Set Cert from Panel" can fill paths that exist on the node.
func (a *NodeController) webCert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	files, err := a.nodeService.GetWebCertFiles(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	jsonObj(c, files, nil)
}

func (a *NodeController) ensureReachable(c *gin.Context, n *model.Node) error {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 6*time.Second)
	defer cancel()
	if _, err := a.nodeService.Probe(ctx, n); err != nil {
		return errors.New(service.FriendlyProbeError(err.Error()))
	}
	return nil
}

func (a *NodeController) add(c *gin.Context) {
	n, ok := middleware.BindAndValidate[model.Node](c)
	if !ok {
		return
	}
	if n.OutboundTag == "" {
		if err := a.ensureReachable(c, n); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
			return
		}
	}
	if err := a.nodeService.Create(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
		return
	}
	if n.OutboundTag != "" {
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warning("apply node outbound bridge failed:", err)
		}
		if err := a.ensureReachable(c, n); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.add"), err)
			return
		}
	}
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.add"), n, nil)
}

func (a *NodeController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	n, ok := middleware.BindAndValidate[model.Node](c)
	if !ok {
		return
	}
	old, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	if n.OutboundTag == "" && old.OutboundTag == "" {
		if err := a.ensureReachable(c, n); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
			return
		}
	}
	if err := a.nodeService.Update(id, n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	if n.OutboundTag != old.OutboundTag {
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warning("apply node outbound bridge change failed:", err)
		}
		if err := a.ensureReachable(c, n); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
			return
		}
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
	n, err := a.nodeService.GetById(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	if err := a.nodeService.SetEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), err)
		return
	}
	if n.OutboundTag != "" {
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warning("apply node enable change failed:", err)
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.update"), nil)
}

func (a *NodeController) inbounds(c *gin.Context) {
	n := &model.Node{}
	if err := c.ShouldBind(n); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.obtain"), err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	options, err := a.nodeService.GetRemoteInboundOptions(ctx, n)
	jsonObj(c, options, err)
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
	var patch service.HeartbeatPatch
	var err error
	if n.OutboundTag != "" {
		patch, err = a.nodeService.ProbeWithOutbound(ctx, n, n.OutboundTag)
	} else {
		patch, err = a.nodeService.Probe(ctx, n)
	}
	jsonObj(c, patch.ToUI(err == nil), nil)
}

func (a *NodeController) certFingerprint(c *gin.Context) {
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
	fp, err := a.nodeService.FetchCertFingerprint(ctx, n)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.test"), err)
		return
	}
	jsonObj(c, fp, nil)
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

func (a *NodeController) updatePanel(c *gin.Context) {
	var req struct {
		Ids []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if len(req.Ids) == 0 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), fmt.Errorf("no nodes selected"))
		return
	}
	results, err := a.nodeService.UpdatePanels(req.Ids)
	jsonMsgObj(c, I18nWeb(c, "pages.nodes.toasts.updateStarted"), results, err)
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
	if err != nil || bucket <= 0 || !service.IsAllowedHistoryBucket(bucket) {
		jsonMsg(c, "invalid bucket", fmt.Errorf("unsupported bucket"))
		return
	}
	jsonObj(c, a.nodeService.AggregateNodeMetric(id, metric, bucket, 60), nil)
}
