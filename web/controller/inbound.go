package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/session"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"

	"github.com/gin-gonic/gin"
)

// InboundController handles HTTP requests related to Xray inbounds management.
type InboundController struct {
	inboundService  service.InboundService
	xrayService     service.XrayService
	fallbackService service.FallbackService
}

// NewInboundController creates a new InboundController and sets up its routes.
func NewInboundController(g *gin.RouterGroup) *InboundController {
	a := &InboundController{}
	a.initRouter(g)
	return a
}

// broadcastInboundsUpdateClientLimit is the threshold past which we skip the
// full-list push over WebSocket and signal the frontend to re-fetch via REST.
// Mirrors the same heuristic used by the periodic traffic job.
const broadcastInboundsUpdateClientLimit = 5000

// broadcastInboundsUpdate fetches and broadcasts the inbound list for userId.
// At scale (10k+ clients) the marshaled JSON exceeds the WS payload ceiling,
// so we send an invalidate signal instead — frontend re-fetches via REST.
// Skipped entirely when no WebSocket clients are connected.
func (a *InboundController) broadcastInboundsUpdate(userId int) {
	if !websocket.HasClients() {
		return
	}
	inbounds, err := a.inboundService.GetInbounds(userId)
	if err != nil {
		return
	}
	totalClients := 0
	for _, ib := range inbounds {
		totalClients += len(ib.ClientStats)
	}
	if totalClients > broadcastInboundsUpdateClientLimit {
		websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
		return
	}
	websocket.BroadcastInbounds(inbounds)
}

// initRouter initializes the routes for inbound-related operations.
func (a *InboundController) initRouter(g *gin.RouterGroup) {

	g.GET("/list", a.getInbounds)
	g.GET("/options", a.getInboundOptions)
	g.GET("/get/:id", a.getInbound)
	g.GET("/:id/fallbacks", a.getFallbacks)

	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)
	g.POST("/setEnable/:id", a.setInboundEnable)
	g.POST("/:id/resetTraffic", a.resetInboundTraffic)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/import", a.importInbound)
	g.POST("/:id/fallbacks", a.setFallbacks)
}

// getInbounds retrieves the list of inbounds for the logged-in user.
func (a *InboundController) getInbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	inbounds, err := a.inboundService.GetInbounds(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbounds, nil)
}

// getInboundOptions returns a lightweight projection of the user's inbounds
// (id, remark, protocol, port, tlsFlowCapable) for pickers in the clients UI.
// Avoids shipping per-client settings and traffic stats just to fill a dropdown.
func (a *InboundController) getInboundOptions(c *gin.Context) {
	user := session.GetLoginUser(c)
	options, err := a.inboundService.GetInboundOptions(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, options, nil)
}

// getInbound retrieves a specific inbound by its ID.
func (a *InboundController) getInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbound, nil)
}

// addInbound creates a new inbound configuration.
func (a *InboundController) addInbound(c *gin.Context) {
	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), err)
		return
	}
	user := session.GetLoginUser(c)
	inbound.UserId = user.Id
	// Treat NodeID=0 as "no node" — gin's *int form binding can land on
	// 0 when the field is absent or empty, and 0 is never a valid Node
	// row id. Without this normalization the runtime layer would try to
	// load Node id=0 and surface "record not found".
	if inbound.NodeID != nil && *inbound.NodeID == 0 {
		inbound.NodeID = nil
	}
	// When the central panel deploys an inbound to a remote node, it sends
	// the Tag pre-computed (so both DBs agree on the identifier). Local
	// UI submits don't include a Tag — we compute one from listen+port
	// using the original collision-avoiding scheme.
	if inbound.Tag == "" {
		if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
			inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
		} else {
			inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
		}
	}

	inbound, needRestart, err := a.inboundService.AddInbound(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), inbound, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	a.broadcastInboundsUpdate(user.Id)
}

// delInbound deletes an inbound configuration by its ID.
func (a *InboundController) delInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundDeleteSuccess"), err)
		return
	}
	needRestart, err := a.inboundService.DelInbound(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundDeleteSuccess"), id, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	user := session.GetLoginUser(c)
	a.broadcastInboundsUpdate(user.Id)
}

// updateInbound updates an existing inbound configuration.
func (a *InboundController) updateInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	inbound := &model.Inbound{
		Id: id,
	}
	err = c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	// Same NodeID=0 → nil normalisation as addInbound. UpdateInbound
	// loads the existing row's NodeID from DB anyway (Phase 1 doesn't
	// support migrating an inbound between nodes), but normalising here
	// keeps the wire shape consistent.
	if inbound.NodeID != nil && *inbound.NodeID == 0 {
		inbound.NodeID = nil
	}
	inbound, needRestart, err := a.inboundService.UpdateInbound(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), inbound, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	user := session.GetLoginUser(c)
	a.broadcastInboundsUpdate(user.Id)
}

// setInboundEnable flips only the enable flag of an inbound. This is a
// dedicated endpoint because the regular update path serialises the entire
// settings JSON (every client) — far too heavy for an interactive switch
// on inbounds with thousands of clients. Frontend optimistically updates
// the UI; we just persist + sync xray + nudge other open admin sessions.
func (a *InboundController) setInboundEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	type form struct {
		Enable bool `json:"enable" form:"enable"`
	}
	var f form
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.inboundService.SetInboundEnable(id, f.Enable)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// Cross-admin sync: lightweight invalidate signal (a few hundred bytes)
	// instead of fetching + serialising the whole inbound list. Other open
	// sessions re-fetch via REST. The toggling admin's own UI already
	// updated optimistically.
	websocket.BroadcastInvalidate(websocket.MessageTypeInbounds)
}

// resetInboundTraffic resets traffic counters for a specific inbound.
func (a *InboundController) resetInboundTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}

	err = a.inboundService.ResetInboundTraffic(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundTrafficSuccess"), nil)
}

// resetAllTraffics resets all traffic counters across all inbounds.
func (a *InboundController) resetAllTraffics(c *gin.Context) {
	err := a.inboundService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllTrafficSuccess"), nil)
}

// importInbound imports an inbound configuration from provided data.
func (a *InboundController) importInbound(c *gin.Context) {
	inbound := &model.Inbound{}
	err := json.Unmarshal([]byte(c.PostForm("data")), inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user := session.GetLoginUser(c)
	inbound.Id = 0
	inbound.UserId = user.Id
	if inbound.Tag == "" {
		if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
			inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
		} else {
			inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
		}
	}

	for index := range inbound.ClientStats {
		inbound.ClientStats[index].Id = 0
		inbound.ClientStats[index].Enable = true
	}

	needRestart := false
	inbound, needRestart, err = a.inboundService.AddInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), inbound, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

// resolveHost mirrors what sub.SubService.ResolveRequest does for the host
// field: prefers X-Forwarded-Host (first entry of any list, port stripped),
// then X-Real-IP, then the host portion of c.Request.Host. Keeping it in the
// controller layer means the service interface stays HTTP-agnostic — service
// methods receive a plain host string instead of a *gin.Context.
func resolveHost(c *gin.Context) string {
	if isTrustedForwardedRequest(c) {
		if h := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); h != "" {
			if i := strings.Index(h, ","); i >= 0 {
				h = strings.TrimSpace(h[:i])
			}
			if hp, _, err := net.SplitHostPort(h); err == nil {
				return hp
			}
			return h
		}
		if h := c.GetHeader("X-Real-IP"); h != "" {
			return h
		}
	}
	if h, _, err := net.SplitHostPort(c.Request.Host); err == nil {
		return h
	}
	return c.Request.Host
}

// getFallbacks returns the fallback rules attached to the master inbound.
func (a *InboundController) getFallbacks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	rows, err := a.fallbackService.GetByMaster(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	jsonObj(c, rows, nil)
}

// setFallbacks atomically replaces the master inbound's fallback list
// and triggers an Xray restart so the new settings.fallbacks take effect.
func (a *InboundController) setFallbacks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	type body struct {
		Fallbacks []service.FallbackInput `json:"fallbacks"`
	}
	var b body
	if err := c.ShouldBindJSON(&b); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.fallbackService.SetByMaster(id, b.Fallbacks); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	a.xrayService.SetToNeedRestart()
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), nil)
}

