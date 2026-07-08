package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/integration"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/outbound"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/gin-gonic/gin"
)

// XraySettingController handles Xray configuration and settings operations.
type XraySettingController struct {
	XraySettingService          service.XraySettingService
	SettingService              service.SettingService
	InboundService              service.InboundService
	OutboundService             outbound.OutboundService
	XrayService                 service.XrayService
	WarpService                 integration.WarpService
	NordService                 integration.NordService
	OutboundSubscriptionService service.OutboundSubscriptionService
}

// NewXraySettingController creates a new XraySettingController and initializes its routes.
func NewXraySettingController(g *gin.RouterGroup) *XraySettingController {
	a := &XraySettingController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for Xray settings management.
func (a *XraySettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xray")
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	g.GET("/getOutboundsTraffic", a.getOutboundsTraffic)
	g.GET("/getXrayResult", a.getXrayResult)

	g.POST("/", a.getXraySetting)
	g.POST("/warp/:action", a.warp)
	g.POST("/nord/:action", a.nord)
	g.POST("/update", a.updateSetting)
	g.POST("/resetOutboundsTraffic", a.resetOutboundsTraffic)
	g.POST("/testOutbound", a.testOutbound)
	g.POST("/testOutbounds", a.testOutbounds)
	g.POST("/balancerStatus", a.balancerStatus)
	g.POST("/balancerOverride", a.balancerOverride)
	g.POST("/routeTest", a.routeTest)

	// Outbound subscription (remote outbound lists)
	g.GET("/outbound-subs", a.listOutboundSubs)
	g.POST("/outbound-subs", a.createOutboundSub)
	g.POST("/outbound-subs/:id/refresh", a.refreshOutboundSub)
	g.POST("/outbound-subs/:id/move", a.moveOutboundSub)
	g.POST("/outbound-subs/:id", a.updateOutboundSub)
	g.DELETE("/outbound-subs/:id", a.deleteOutboundSub)
	g.POST("/outbound-subs/:id/del", a.deleteOutboundSub) // POST alias for clients that can't send DELETE
	g.POST("/outbound-subs/parse", a.parseOutboundSubURL) // preview without saving
}

// getXraySetting retrieves the Xray configuration template, inbound tags, and outbound test URL.
func (a *XraySettingController) getXraySetting(c *gin.Context) {
	xraySetting, err := a.SettingService.GetXrayConfigTemplate()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	// Older versions of this handler embedded the raw DB value as
	// `xraySetting` in the response without checking if the value
	// already had that wrapper shape. When the frontend saved it
	// back through the textarea verbatim, the wrapper got persisted
	// and every subsequent save nested another layer, which is what
	// eventually produced the blank Xray Settings page in #4059.
	// Strip any such wrapper here, and heal the DB if we found one so
	// the next read is O(1) instead of climbing the same pile again.
	if unwrapped := service.UnwrapXrayTemplateConfig(xraySetting); unwrapped != xraySetting {
		if saveErr := a.XraySettingService.SaveXraySetting(unwrapped); saveErr == nil {
			xraySetting = unwrapped
		} else {
			// Don't fail the read — just serve the unwrapped value
			// and leave the DB healing for a later save.
			xraySetting = unwrapped
		}
	}
	inboundTags, err := a.InboundService.GetInboundTags()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	clientReverseTags, err := a.InboundService.GetClientReverseTags()
	if err != nil {
		clientReverseTags = "[]"
	}
	outboundTestUrl, _ := a.SettingService.GetXrayOutboundTestUrl()
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	xrayResponse := map[string]any{
		"xraySetting":       json.RawMessage(xraySetting),
		"inboundTags":       json.RawMessage(inboundTags),
		"clientReverseTags": json.RawMessage(clientReverseTags),
		"outboundTestUrl":   outboundTestUrl,
	}

	// Surface subscription outbounds (and their tags) so the frontend can:
	// - show them as read-only items in the Outbounds tab
	// - let users pick them in balancers and routing rules
	// These are not part of the editable template; they are injected at runtime.
	if subObs, err := a.OutboundSubscriptionService.AllActiveOutbounds(); err == nil && len(subObs) > 0 {
		xrayResponse["subscriptionOutbounds"] = subObs
	}
	if subTags, err := a.OutboundSubscriptionService.AllActiveOutboundTags(); err == nil && len(subTags) > 0 {
		xrayResponse["subscriptionOutboundTags"] = subTags
	}
	result, err := json.Marshal(xrayResponse)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, string(result), nil)
}

// updateSetting updates the Xray configuration settings and applies them to
// the running core right away — through the gRPC API when only inbounds,
// outbounds or routing rules changed, with a process restart otherwise.
func (a *XraySettingController) updateSetting(c *gin.Context) {
	xraySetting := c.PostForm("xraySetting")
	if err := a.XraySettingService.SaveXraySetting(xraySetting); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	outboundTestUrl := c.PostForm("outboundTestUrl")
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	if err := a.SettingService.SetXrayOutboundTestUrl(outboundTestUrl); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	// Only reconcile a running core; a manually stopped xray stays stopped.
	if a.XrayService.IsXrayRunning() {
		if err := a.XrayService.RestartXray(false); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
			return
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), nil)
}

// getDefaultXrayConfig retrieves the default Xray configuration.
func (a *XraySettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.SettingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}

// getXrayResult retrieves the current Xray service result.
func (a *XraySettingController) getXrayResult(c *gin.Context) {
	jsonObj(c, a.XrayService.GetXrayResult(), nil)
}

// warp handles Warp-related operations based on the action parameter.
func (a *XraySettingController) warp(c *gin.Context) {
	action := c.Param("action")
	var resp string
	var err error
	switch action {
	case "data":
		resp, err = a.WarpService.GetWarpData()
	case "del":
		err = a.WarpService.DelWarpData()
	case "config":
		resp, err = a.WarpService.GetWarpConfig()
	case "reg":
		skey := c.PostForm("privateKey")
		pkey := c.PostForm("publicKey")
		resp, err = a.WarpService.RegWarp(skey, pkey)
	case "changeIp":
		resp, err = a.WarpService.ChangeWarpIP()
		if err == nil {
			a.XrayService.SetToNeedRestart()
			// Restart the auto-update clock so a scheduled rotation
			// doesn't fire right after this manual one.
			_ = a.SettingService.SetWarpLastUpdate(time.Now().Unix())
		}
	case "license":
		license := c.PostForm("license")
		resp, err = a.WarpService.SetWarpLicense(license)
	case "interval":
		interval, convErr := strconv.Atoi(c.PostForm("interval"))
		if convErr != nil || interval < 0 {
			err = common.NewError("invalid warp update interval")
		} else if err = a.SettingService.SetWarpUpdateInterval(interval); err == nil && interval > 0 {
			// Count the interval from now rather than from epoch 0,
			// otherwise the job would rotate on its next tick.
			_ = a.SettingService.SetWarpLastUpdate(time.Now().Unix())
		}
	}

	jsonObj(c, resp, err)
}

// nord handles NordVPN-related operations based on the action parameter.
func (a *XraySettingController) nord(c *gin.Context) {
	action := c.Param("action")
	var resp string
	var err error
	switch action {
	case "countries":
		resp, err = a.NordService.GetCountries()
	case "servers":
		countryId := c.PostForm("countryId")
		resp, err = a.NordService.GetServers(countryId)
	case "reg":
		token := c.PostForm("token")
		resp, err = a.NordService.GetCredentials(token)
	case "setKey":
		key := c.PostForm("key")
		resp, err = a.NordService.SetKey(key)
	case "data":
		resp, err = a.NordService.GetNordData()
	case "del":
		err = a.NordService.DelNordData()
	}

	jsonObj(c, resp, err)
}

// getOutboundsTraffic retrieves the traffic statistics for outbounds.
func (a *XraySettingController) getOutboundsTraffic(c *gin.Context) {
	outboundsTraffic, err := a.OutboundService.GetOutboundsTraffic()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getOutboundTrafficError"), err)
		return
	}
	jsonObj(c, outboundsTraffic, nil)
}

// resetOutboundsTraffic resets the traffic statistics for the specified outbound tag.
func (a *XraySettingController) resetOutboundsTraffic(c *gin.Context) {
	tag := c.PostForm("tag")
	err := a.OutboundService.ResetOutboundTraffic(tag)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.resetOutboundTrafficError"), err)
		return
	}
	jsonObj(c, "", nil)
}

// testOutbound tests an outbound configuration and returns the delay/response time.
// Optional form "allOutbounds": JSON array of all outbounds; used to resolve sockopt.dialerProxy dependencies.
// Optional form "mode": "tcp" for a fast dial-only probe, "real" for the cold
// full-request delay, anything else (default) for a full HTTP probe through a temp xray instance.
func (a *XraySettingController) testOutbound(c *gin.Context) {
	outboundJSON := c.PostForm("outbound")
	allOutboundsJSON := c.PostForm("allOutbounds")
	mode := c.PostForm("mode")

	if outboundJSON == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("outbound parameter is required"))
		return
	}

	// Load the test URL from server settings to prevent SSRF via user-controlled URLs
	testURL, _ := a.SettingService.GetXrayOutboundTestUrl()
	testURL, err := service.SanitizePublicHTTPURL(testURL, false)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	result, err := a.OutboundService.TestOutbound(outboundJSON, testURL, allOutboundsJSON, mode)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonObj(c, result, nil)
}

// testOutbounds tests a batch of outbound configurations through one shared
// temp xray instance and returns an array of results in input order.
// Form "outbounds": JSON array of outbound configs (required).
// Optional form "allOutbounds": JSON array of all outbounds; used to resolve sockopt.dialerProxy dependencies.
// Optional form "mode": "tcp" for fast dial-only probes, "real" for the cold
// full-request delay, anything else (default) for real HTTP requests routed through each outbound.
func (a *XraySettingController) testOutbounds(c *gin.Context) {
	outboundsJSON := c.PostForm("outbounds")
	allOutboundsJSON := c.PostForm("allOutbounds")
	mode := c.PostForm("mode")

	if outboundsJSON == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("outbounds parameter is required"))
		return
	}

	// Load the test URL from server settings to prevent SSRF via user-controlled URLs
	testURL, _ := a.SettingService.GetXrayOutboundTestUrl()
	testURL, err := service.SanitizePublicHTTPURL(testURL, false)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	results, err := a.OutboundService.TestOutbounds(outboundsJSON, testURL, allOutboundsJSON, mode)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonObj(c, results, nil)
}

// balancerStatus reports the live state (override + strategy picks) of the
// balancer tags given as a comma-separated "tags" form field.
func (a *XraySettingController) balancerStatus(c *gin.Context) {
	raw := c.PostForm("tags")
	var tags []string
	for tag := range strings.SplitSeq(raw, ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	statuses, err := a.XrayService.GetBalancersStatus(tags)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	byTag := make(map[string]service.BalancerStatus, len(statuses))
	for _, status := range statuses {
		byTag[status.Tag] = status
	}
	jsonObj(c, byTag, nil)
}

// balancerOverride forces a balancer to a specific outbound tag; an empty
// "target" clears the override.
func (a *XraySettingController) balancerOverride(c *gin.Context) {
	tag := c.PostForm("tag")
	if tag == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("tag is required"))
		return
	}
	target := c.PostForm("target")
	if err := a.XrayService.OverrideBalancer(tag, target); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, "", nil)
}

// routeTest asks the running core which outbound it would route a synthetic
// connection to.
func (a *XraySettingController) routeTest(c *gin.Context) {
	port := 0
	if portStr := c.PostForm("port"); portStr != "" {
		parsed, err := strconv.Atoi(portStr)
		if err != nil || parsed < 0 || parsed > 65535 {
			jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("invalid port"))
			return
		}
		port = parsed
	}
	req := xray.RouteTestRequest{
		InboundTag: c.PostForm("inboundTag"),
		Domain:     c.PostForm("domain"),
		IP:         c.PostForm("ip"),
		Port:       port,
		Network:    c.PostForm("network"),
		Protocol:   c.PostForm("protocol"),
		Email:      c.PostForm("email"),
	}
	if req.Domain == "" && req.IP == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("domain or ip is required"))
		return
	}
	result, err := a.XrayService.TestRoute(req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
}

// --- Outbound Subscription handlers ---

func (a *XraySettingController) listOutboundSubs(c *gin.Context) {
	list, err := a.OutboundSubscriptionService.List()
	if err != nil {
		jsonMsg(c, "Failed to list outbound subscriptions", err)
		return
	}
	jsonObj(c, list, nil)
}

func (a *XraySettingController) createOutboundSub(c *gin.Context) {
	remark := c.PostForm("remark")
	rawURL := c.PostForm("url")
	prefix := c.PostForm("tagPrefix")
	enabled := c.PostForm("enabled") != "false"
	allowPrivate := c.PostForm("allowPrivate") == "true"
	prepend := c.PostForm("prepend") == "true"
	intervalStr := c.PostForm("updateInterval")
	interval := 600
	if intervalStr != "" {
		if v, err := parseIntSafe(intervalStr); err == nil && v > 0 {
			interval = v
		}
	}
	sub, err := a.OutboundSubscriptionService.Create(remark, rawURL, prefix, enabled, interval, allowPrivate, prepend)
	if err != nil {
		jsonMsg(c, "Failed to create outbound subscription", err)
		return
	}
	jsonObj(c, sub, nil)
}

func (a *XraySettingController) updateOutboundSub(c *gin.Context) {
	id := c.Param("id")
	var subID int
	if _, err := fmt.Sscanf(id, "%d", &subID); err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	remark := c.PostForm("remark")
	rawURL := c.PostForm("url")
	prefix := c.PostForm("tagPrefix")
	enabled := c.PostForm("enabled") != "false"
	allowPrivate := c.PostForm("allowPrivate") == "true"
	prepend := c.PostForm("prepend") == "true"
	intervalStr := c.PostForm("updateInterval")
	interval := 600
	if intervalStr != "" {
		if v, err := parseIntSafe(intervalStr); err == nil && v > 0 {
			interval = v
		}
	}
	if err := a.OutboundSubscriptionService.Update(subID, remark, rawURL, prefix, enabled, interval, allowPrivate, prepend); err != nil {
		jsonMsg(c, "Failed to update outbound subscription", err)
		return
	}
	jsonObj(c, "", nil)
}

func (a *XraySettingController) deleteOutboundSub(c *gin.Context) {
	id := c.Param("id")
	var subID int
	if _, err := fmt.Sscanf(id, "%d", &subID); err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	if err := a.OutboundSubscriptionService.Delete(subID); err != nil {
		jsonMsg(c, "Failed to delete outbound subscription", err)
		return
	}
	// Signal that xray should drop this subscription's outbounds on next reload.
	a.XrayService.SetToNeedRestart()
	jsonObj(c, "", nil)
}

func (a *XraySettingController) refreshOutboundSub(c *gin.Context) {
	id := c.Param("id")
	var subID int
	if _, err := fmt.Sscanf(id, "%d", &subID); err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	obs, err := a.OutboundSubscriptionService.Refresh(subID)
	if err != nil {
		jsonMsg(c, "Refresh failed", err)
		return
	}
	// Signal that xray should pick up the new outbounds on next restart/reload
	a.XrayService.SetToNeedRestart()
	jsonObj(c, obs, nil)
}

func (a *XraySettingController) moveOutboundSub(c *gin.Context) {
	id := c.Param("id")
	var subID int
	if _, err := fmt.Sscanf(id, "%d", &subID); err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	up := c.PostForm("dir") == "up"
	if err := a.OutboundSubscriptionService.Move(subID, up); err != nil {
		jsonMsg(c, "Failed to reorder outbound subscription", err)
		return
	}
	// Order affects the merged outbounds, so xray needs a reload.
	a.XrayService.SetToNeedRestart()
	jsonObj(c, "", nil)
}

// parseOutboundSubURL is a preview endpoint: it fetches + parses the provided
// URL but does not persist anything. Useful for the "add subscription" flow
// so the user can see the resulting outbounds (and assigned tags) before saving.
func (a *XraySettingController) parseOutboundSubURL(c *gin.Context) {
	rawURL := c.PostForm("url")
	if rawURL == "" {
		jsonMsg(c, "url is required", common.NewError("missing url"))
		return
	}
	allowPrivate := c.PostForm("allowPrivate") == "true"
	// Use a throw-away service instance; it only needs the settingService for proxy.
	svc := service.OutboundSubscriptionService{}
	// We don't have a direct "fetch once" that returns without storing, so we
	// temporarily create a disabled row, refresh it, then delete. Cleaner would
	// be to expose a pure ParseURL on the service, but this keeps the surface small.
	tmp, err := svc.Create("preview", rawURL, "", false, 600, allowPrivate, false)
	if err != nil {
		jsonMsg(c, "Failed to preview subscription", err)
		return
	}
	obs, err := svc.Refresh(tmp.Id)
	// best-effort cleanup
	_ = svc.Delete(tmp.Id)
	if err != nil {
		jsonMsg(c, "Failed to fetch/parse subscription", err)
		return
	}
	jsonObj(c, obs, nil)
}

func parseIntSafe(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
