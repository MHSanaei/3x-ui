package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/util/random"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"

	"github.com/gin-gonic/gin"
)

func notifyClientsChanged() {
	websocket.BroadcastInvalidate(websocket.MessageTypeClients)
}

func parseInboundIdsQuery(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

type ClientController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
	settingService service.SettingService
}

func NewClientController(g *gin.RouterGroup) *ClientController {
	a := &ClientController{}
	a.initRouter(g)
	return a
}

func (a *ClientController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/list/paged", a.listPaged)
	g.GET("/get/:email", a.get)
	g.GET("/traffic/:email", a.getTrafficByEmail)
	g.GET("/subLinks/:subId", a.getSubLinks)
	g.GET("/links/:email", a.getClientLinks)

	g.POST("/add", a.create)
	g.POST("/update/:email", a.update)
	g.POST("/del/:email", a.delete)
	g.POST("/:email/attach", a.attach)
	g.POST("/:email/detach", a.detach)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/delDepleted", a.delDepleted)
	g.POST("/bulkAdjust", a.bulkAdjust)
	g.POST("/bulkDel", a.bulkDelete)
	g.POST("/bulkCreate", a.bulkCreate)
	g.POST("/bulkAttach", a.bulkAttach)
	g.POST("/bulkDetach", a.bulkDetach)
	g.POST("/bulkResetTraffic", a.bulkResetTraffic)
	g.POST("/resetTraffic/:email", a.resetTrafficByEmail)
	g.POST("/updateTraffic/:email", a.updateTrafficByEmail)
	g.POST("/ips/:email", a.getIps)
	g.POST("/clearIps/:email", a.clearIps)
	g.POST("/onlines", a.onlines)
	g.POST("/onlinesByGuid", a.onlinesByGuid)
	g.POST("/activeInbounds", a.activeInbounds)
	g.POST("/lastOnline", a.lastOnline)
}

func (a *ClientController) list(c *gin.Context) {
	rows, err := a.clientService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *ClientController) listPaged(c *gin.Context) {
	var params service.ClientPageParams
	if err := c.ShouldBindQuery(&params); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	resp, err := a.clientService.ListPaged(&a.inboundService, &a.settingService, params)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, resp, nil)
}

func (a *ClientController) get(c *gin.Context) {
	email := c.Param("email")
	rec, err := a.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	inboundIds, err := a.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	flow, err := a.clientService.EffectiveFlow(nil, rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	rec.Flow = flow
	jsonObj(c, gin.H{"client": rec, "inboundIds": inboundIds}, nil)
}

func (a *ClientController) create(c *gin.Context) {
	var payload service.ClientCreatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.clientService.Create(&a.inboundService, &payload)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	obj := pendingNodeObj(a.inboundService.AnyNodePending(payload.InboundIds))

	if payload.GetClientJson && len(payload.InboundIds) > 0 {
		inbound, ibErr := a.inboundService.GetInbound(payload.InboundIds[0])
		if ibErr == nil {
			clientRec, crErr := a.clientService.GetRecordByEmail(nil, payload.Client.Email)
			if crErr == nil {
				cfg, cfgErr := buildClientConfig(inbound, clientRec, resolveHost(c))
				if cfgErr == nil {
					var data gin.H
					if obj != nil {
						data = obj.(gin.H)
					} else {
						data = gin.H{}
					}
					data["clientConfig"] = cfg
					obj = data
				}
			}
		}
	}

	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), obj, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) update(c *gin.Context) {
	email := c.Param("email")
	var updated model.Client
	if err := c.ShouldBindJSON(&updated); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	inboundFilter := parseInboundIdsQuery(c.Query("inboundIds"))
	needRestart, err := a.clientService.UpdateByEmail(&a.inboundService, email, updated, inboundFilter...)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), pendingNodeObj(a.clientService.HasPendingNode(&a.inboundService, email)), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) delete(c *gin.Context) {
	email := c.Param("email")
	keepTraffic := c.Query("keepTraffic") == "1"
	needRestart, err := a.clientService.DeleteByEmail(&a.inboundService, email, keepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type attachDetachBody struct {
	InboundIds []int `json:"inboundIds"`
}

func (a *ClientController) attach(c *gin.Context) {
	email := c.Param("email")
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.clientService.AttachByEmail(&a.inboundService, email, body.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(body.InboundIds)), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) resetAllTraffics(c *gin.Context) {
	needRestart, err := a.clientService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkAdjustRequest struct {
	Emails   []string `json:"emails"`
	AddDays  int      `json:"addDays"`
	AddBytes int64    `json:"addBytes"`
}

func (a *ClientController) bulkAdjust(c *gin.Context) {
	var req bulkAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkAdjust(&a.inboundService, req.Emails, req.AddDays, req.AddBytes)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkDeleteRequest struct {
	Emails      []string `json:"emails"`
	KeepTraffic bool     `json:"keepTraffic"`
}

type bulkAttachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkAttach(c *gin.Context) {
	var req bulkAttachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkAttach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkDetachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkDetach(c *gin.Context) {
	var req bulkDetachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkDetach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) bulkDelete(c *gin.Context) {
	var req bulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkDelete(&a.inboundService, req.Emails, req.KeepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) bulkCreate(c *gin.Context) {
	var payloads []service.ClientCreatePayload
	if err := c.ShouldBindJSON(&payloads); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkCreate(&a.inboundService, payloads)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) delDepleted(c *gin.Context) {
	deleted, needRestart, err := a.clientService.DelDepleted(&a.inboundService)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"deleted": deleted}, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

func (a *ClientController) resetTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	needRestart, err := a.clientService.ResetTrafficByEmail(&a.inboundService, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type trafficUpdateRequest struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

func (a *ClientController) updateTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	var req trafficUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.inboundService.UpdateClientTrafficByEmail(email, req.Upload, req.Download); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	notifyClientsChanged()
}

func (a *ClientController) getIps(c *gin.Context) {
	email := c.Param("email")
	ips, err := a.inboundService.GetInboundClientIps(email)
	if err != nil || ips == "" {
		jsonObj(c, "No IP Record", nil)
		return
	}
	type ipWithTimestamp struct {
		IP        string `json:"ip"`
		Timestamp int64  `json:"timestamp"`
	}
	var ipsWithTime []ipWithTimestamp
	if err := json.Unmarshal([]byte(ips), &ipsWithTime); err == nil && len(ipsWithTime) > 0 {
		formatted := make([]string, 0, len(ipsWithTime))
		for _, item := range ipsWithTime {
			if item.IP == "" {
				continue
			}
			if item.Timestamp > 0 {
				ts := time.Unix(item.Timestamp, 0).Local().Format("2006-01-02 15:04:05")
				formatted = append(formatted, fmt.Sprintf("%s (%s)", item.IP, ts))
				continue
			}
			formatted = append(formatted, item.IP)
		}
		jsonObj(c, formatted, nil)
		return
	}
	var oldIps []string
	if err := json.Unmarshal([]byte(ips), &oldIps); err == nil && len(oldIps) > 0 {
		jsonObj(c, oldIps, nil)
		return
	}
	jsonObj(c, ips, nil)
}

func (a *ClientController) clearIps(c *gin.Context) {
	email := c.Param("email")
	if err := a.inboundService.ClearClientIps(email); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), nil)
}

func (a *ClientController) onlines(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClients(), nil)
}

func (a *ClientController) onlinesByGuid(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClientsByGuid(), nil)
}

func (a *ClientController) activeInbounds(c *gin.Context) {
	jsonObj(c, a.inboundService.GetActiveInboundsByGuid(), nil)
}

func (a *ClientController) lastOnline(c *gin.Context) {
	data, err := a.inboundService.GetClientsLastOnline()
	jsonObj(c, data, err)
}

func (a *ClientController) getTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	traffic, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return
	}
	jsonObj(c, traffic, nil)
}

func (a *ClientController) getSubLinks(c *gin.Context) {
	links, err := a.inboundService.GetSubLinks(resolveHost(c), c.Param("subId"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *ClientController) getClientLinks(c *gin.Context) {
	links, err := a.inboundService.GetAllClientLinks(resolveHost(c), c.Param("email"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *ClientController) detach(c *gin.Context) {
	email := c.Param("email")
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.clientService.DetachByEmailMany(&a.inboundService, email, body.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(body.InboundIds)), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	notifyClientsChanged()
}

type bulkResetRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkResetTraffic(c *gin.Context) {
	var req bulkResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	affected, err := a.clientService.BulkResetTraffic(&a.inboundService, req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	a.xrayService.SetToNeedRestart()
	notifyClientsChanged()
}

func isRoutableHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return !ip.IsLoopback() && !ip.IsUnspecified()
	}
	return true
}

// resolveInboundAddress resolves the publicly reachable address for an inbound.
// Priority: node address > routable listen address > request host.
func resolveInboundAddress(inbound *model.Inbound, host string) string {
	if listen := inbound.Listen; listen != "" && listen[0] != '@' && listen[0] != '/' && isRoutableHost(listen) {
		return listen
	}
	return host
}

func sanitizeStreamSettings(raw string) map[string]any {
	if raw == "" || raw == "{}" {
		return nil
	}
	var ss map[string]any
	if err := json.Unmarshal([]byte(raw), &ss); err != nil || len(ss) == 0 {
		return nil
	}
	delete(ss, "sockopt")

	switch security, _ := ss["security"].(string); security {
	case "tls":
		if tls, ok := ss["tlsSettings"].(map[string]any); ok {
			clean := map[string]any{
				"serverName": tls["serverName"],
				"alpn":       tls["alpn"],
			}
			if settings, _ := tls["settings"].(map[string]any); settings != nil {
				if fp, _ := settings["fingerprint"].(string); fp != "" {
					clean["fingerprint"] = fp
				}
				if ech, _ := settings["echConfigList"].(string); ech != "" {
					clean["echConfigList"] = ech
				}
				if pins, _ := settings["pinnedPeerCertSha256"].([]any); len(pins) > 0 {
					clean["pinnedPeerCertSha256"] = pins
				}
			}
			ss["tlsSettings"] = clean
		}
	case "reality":
		if reality, ok := ss["realitySettings"].(map[string]any); ok {
			clean := map[string]any{
				"publicKey":   reality["publicKey"],
				"fingerprint": reality["fingerprint"],
			}
			if settings, _ := reality["settings"].(map[string]any); settings != nil {
				if pk, _ := settings["publicKey"].(string); pk != "" {
					clean["publicKey"] = pk
				}
				if fp, _ := settings["fingerprint"].(string); fp != "" {
					clean["fingerprint"] = fp
				}
				if mldsa, _ := settings["mldsa65Verify"].(string); mldsa != "" {
					clean["mldsa65Verify"] = mldsa
				}
			}
			if serverNames, _ := reality["serverNames"].([]any); len(serverNames) > 0 {
				clean["serverName"] = serverNames[0].(string)
			}
			if shortIds, _ := reality["shortIds"].([]any); len(shortIds) > 0 {
				clean["shortId"] = shortIds[0].(string)
			}
			clean["spiderX"] = "/" + randomString(15)
			ss["realitySettings"] = clean
		}
	}

	network, _ := ss["network"].(string)
	switch network {
	case "tcp":
		if tcp, ok := ss["tcpSettings"].(map[string]any); ok {
			delete(tcp, "acceptProxyProtocol")
		}
	case "ws":
		if ws, ok := ss["wsSettings"].(map[string]any); ok {
			delete(ws, "acceptProxyProtocol")
		}
	case "httpupgrade":
		if hu, ok := ss["httpupgradeSettings"].(map[string]any); ok {
			delete(hu, "acceptProxyProtocol")
		}
	case "xhttp":
		if xh, ok := ss["xhttpSettings"].(map[string]any); ok {
			delete(xh, "acceptProxyProtocol")
			delete(xh, "noSSEHeader")
			delete(xh, "scMaxBufferedPosts")
			delete(xh, "scStreamUpServerSecs")
			delete(xh, "serverMaxHeaderBytes")
		}
	case "grpc":
		if grpc, ok := ss["grpcSettings"].(map[string]any); ok {
			delete(grpc, "acceptProxyProtocol")
		}
	}
	return ss
}

func randomString(n int) string {
	return random.Seq(n)
}

// buildClientConfig builds a V2Ray JSON config object compatible with v2rayNG
// from an inbound and client record, using the same per-protocol format
// as the v2rayNG V2rayConfig.OutboundBean data model.
func buildClientConfig(inbound *model.Inbound, client *model.ClientRecord, host string) (map[string]any, error) {
	address := resolveInboundAddress(inbound, host)

	streamSettings := sanitizeStreamSettings(inbound.StreamSettings)

	outbound := map[string]any{
		"protocol": string(inbound.Protocol),
		"tag":      "proxy",
	}

	switch inbound.Protocol {
	case model.VMESS:
		security := client.Security
		if security == "" {
			security = "auto"
		}
		outbound["settings"] = map[string]any{
			"vnext": []any{
				map[string]any{
					"address": address,
					"port":    inbound.Port,
					"users": []any{
						map[string]any{
							"id":       client.UUID,
							"security": security,
						},
					},
				},
			},
		}

	case model.VLESS:
		var inboundSettings map[string]any
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		encryption, _ := inboundSettings["encryption"].(string)
		user := map[string]any{
			"id":         client.UUID,
			"encryption": encryption,
		}
		if client.Flow != "" {
			user["flow"] = client.Flow
		}
		outbound["settings"] = map[string]any{
			"vnext": []any{
				map[string]any{
					"address": address,
					"port":    inbound.Port,
					"users":   []any{user},
				},
			},
		}

	case model.Trojan:
		outbound["settings"] = map[string]any{
			"servers": []any{
				map[string]any{
					"address":  address,
					"port":     inbound.Port,
					"password": client.Password,
				},
			},
		}

	case model.Shadowsocks:
		var inboundSettings map[string]any
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		method, _ := inboundSettings["method"].(string)
		password := client.Password
		if strings.HasPrefix(method, "2022") {
			if serverPassword, ok := inboundSettings["password"].(string); ok {
				password = fmt.Sprintf("%s:%s", serverPassword, client.Password)
			}
		}
		outbound["settings"] = map[string]any{
			"servers": []any{
				map[string]any{
					"address":  address,
					"port":     inbound.Port,
					"password": password,
					"method":   method,
				},
			},
		}

	case model.Hysteria:
		var inboundSettings map[string]any
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		version, _ := inboundSettings["version"].(float64)
		outbound["settings"] = map[string]any{
			"version": int(version),
			"address": address,
			"port":    inbound.Port,
		}
		// Build hysteria stream settings
		var stream map[string]any
		json.Unmarshal([]byte(inbound.StreamSettings), &stream)
		hyStream, _ := stream["hysteriaSettings"].(map[string]any)
		if hyStream != nil {
			outHyStream := map[string]any{
				"version": int(version),
				"auth":    client.Auth,
			}
			if udpIdleTimeout, ok := hyStream["udpIdleTimeout"].(float64); ok {
				outHyStream["udpIdleTimeout"] = int(udpIdleTimeout)
			}
			if masquerade, ok := hyStream["masquerade"].(map[string]any); ok {
				outHyStream["masquerade"] = masquerade
			}
			hyStream = outHyStream
		}
		if stream != nil {
			stream["network"] = "hysteria"
			stream["security"] = "tls"
			delete(stream, "sockopt")
			outbound["streamSettings"] = stream
		}
		streamSettings = nil // already handled above
	}

	if streamSettings != nil {
		outbound["streamSettings"] = streamSettings
	}

	config := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "socks-in",
				"listen":   "127.0.0.1",
				"port":     10808,
				"protocol": "socks",
				"settings": map[string]any{
					"auth": "noauth",
					"udp":  true,
				},
				"sniffing": map[string]any{
					"enabled":     true,
					"destOverride": []any{"http", "tls"},
				},
			},
		},
		"outbounds": []any{
			outbound,
			map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": map[string]any{
					"domainStrategy": "UseIP",
				},
			},
			map[string]any{
				"tag":      "block",
				"protocol": "blackhole",
				"settings": map[string]any{
					"response": map[string]any{
						"type": "http",
					},
				},
			},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{
					"type":        "field",
					"inboundTag":  []any{"socks-in"},
					"outboundTag": "proxy",
				},
			},
		},
		"dns": map[string]any{
			"hosts":  map[string]any{},
			"servers": []any{},
		},
	}

	return config, nil
}
