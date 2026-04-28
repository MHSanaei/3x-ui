package sub

import (
	"encoding/base64"
	"fmt"
	"maps"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/random"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// SubService provides business logic for generating subscription links and managing subscription data.
type SubService struct {
	address        string
	showInfo       bool
	remarkModel    string
	datepicker     string
	inboundService service.InboundService
	settingService service.SettingService
}

// NewSubService creates a new subscription service with the given configuration.
func NewSubService(showInfo bool, remarkModel string) *SubService {
	return &SubService{
		showInfo:    showInfo,
		remarkModel: remarkModel,
	}
}

// GetSubs retrieves subscription links for a given subscription ID and host.
func (s *SubService) GetSubs(subId string, host string) ([]string, int64, xray.ClientTraffic, error) {
	s.address = host
	var result []string
	var traffic xray.ClientTraffic
	var lastOnline int64
	var clientTraffics []xray.ClientTraffic
	inbounds, err := s.getInboundsBySubId(subId)
	if err != nil {
		return nil, 0, traffic, err
	}

	if len(inbounds) == 0 {
		return nil, 0, traffic, common.NewError("No inbounds found with ", subId)
	}

	s.datepicker, err = s.settingService.GetDatepicker()
	if err != nil {
		s.datepicker = "gregorian"
	}
	for _, inbound := range inbounds {
		clients, err := s.inboundService.GetClients(inbound)
		if err != nil {
			logger.Error("SubService - GetClients: Unable to get clients from inbound")
		}
		if clients == nil {
			continue
		}
		if len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
			listen, port, streamSettings, err := s.getFallbackMaster(inbound.Listen, inbound.StreamSettings)
			if err == nil {
				inbound.Listen = listen
				inbound.Port = port
				inbound.StreamSettings = streamSettings
			}
		}
		for _, client := range clients {
			if client.Enable && client.SubID == subId {
				link := s.getLink(inbound, client.Email)
				result = append(result, link)
				ct := s.getClientTraffics(inbound.ClientStats, client.Email)
				clientTraffics = append(clientTraffics, ct)
				if ct.LastOnline > lastOnline {
					lastOnline = ct.LastOnline
				}
			}
		}
	}

	// Prepare statistics
	for index, clientTraffic := range clientTraffics {
		if index == 0 {
			traffic.Up = clientTraffic.Up
			traffic.Down = clientTraffic.Down
			traffic.Total = clientTraffic.Total
			if clientTraffic.ExpiryTime > 0 {
				traffic.ExpiryTime = clientTraffic.ExpiryTime
			}
		} else {
			traffic.Up += clientTraffic.Up
			traffic.Down += clientTraffic.Down
			if traffic.Total == 0 || clientTraffic.Total == 0 {
				traffic.Total = 0
			} else {
				traffic.Total += clientTraffic.Total
			}
			if clientTraffic.ExpiryTime != traffic.ExpiryTime {
				traffic.ExpiryTime = 0
			}
		}
	}
	return result, lastOnline, traffic, nil
}

func (s *SubService) getInboundsBySubId(subId string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	// allow "hysteria2" so imports stored with the literal v2 protocol
	// string still surface here (#4081)
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where(`id in (
		SELECT DISTINCT inbounds.id
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE
			protocol in ('vmess','vless','trojan','shadowsocks','hysteria','hysteria2')
			AND JSON_EXTRACT(client.value, '$.subId') = ? AND enable = ?
	)`, subId, true).Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *SubService) getClientTraffics(traffics []xray.ClientTraffic, email string) xray.ClientTraffic {
	for _, traffic := range traffics {
		if traffic.Email == email {
			return traffic
		}
	}
	return xray.ClientTraffic{}
}

func (s *SubService) getFallbackMaster(dest string, streamSettings string) (string, int, string, error) {
	db := database.GetDB()
	var inbound *model.Inbound
	err := db.Model(model.Inbound{}).
		Where("JSON_TYPE(settings, '$.fallbacks') = 'array'").
		Where("EXISTS (SELECT * FROM json_each(settings, '$.fallbacks') WHERE json_extract(value, '$.dest') = ?)", dest).
		Find(&inbound).Error
	if err != nil {
		return "", 0, "", err
	}

	var stream map[string]any
	json.Unmarshal([]byte(streamSettings), &stream)
	var masterStream map[string]any
	json.Unmarshal([]byte(inbound.StreamSettings), &masterStream)
	stream["security"] = masterStream["security"]
	stream["tlsSettings"] = masterStream["tlsSettings"]
	stream["externalProxy"] = masterStream["externalProxy"]
	modifiedStream, _ := json.MarshalIndent(stream, "", "  ")

	return inbound.Listen, inbound.Port, string(modifiedStream), nil
}

func (s *SubService) getLink(inbound *model.Inbound, email string) string {
	switch inbound.Protocol {
	case "vmess":
		return s.genVmessLink(inbound, email)
	case "vless":
		return s.genVlessLink(inbound, email)
	case "trojan":
		return s.genTrojanLink(inbound, email)
	case "shadowsocks":
		return s.genShadowsocksLink(inbound, email)
	case "hysteria", "hysteria2":
		return s.genHysteriaLink(inbound, email)
	}
	return ""
}

// Protocol link generators are intentionally ordered as:
// vmess -> vless -> trojan -> shadowsocks -> hysteria.
func (s *SubService) genVmessLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.VMESS {
		return ""
	}
	address := s.resolveInboundAddress(inbound)
	obj := map[string]any{
		"v":    "2",
		"add":  address,
		"port": inbound.Port,
		"type": "none",
	}
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	network, _ := stream["network"].(string)
	applyVmessNetworkParams(stream, network, obj)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskObj(finalmask, obj)
	}
	security, _ := stream["security"].(string)
	obj["tls"] = security
	if security == "tls" {
		applyVmessTLSParams(stream, obj)
	}

	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := findClientIndex(clients, email)
	obj["id"] = clients[clientIndex].ID
	obj["scy"] = clients[clientIndex].Security

	externalProxies, _ := stream["externalProxy"].([]any)

	if len(externalProxies) > 0 {
		return s.buildVmessExternalProxyLinks(externalProxies, obj, inbound, email)
	}

	obj["ps"] = s.genRemark(inbound, email, "")
	return buildVmessLink(obj)
}

func (s *SubService) genVlessLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.VLESS {
		return ""
	}
	address := s.resolveInboundAddress(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := findClientIndex(clients, email)
	uuid := clients[clientIndex].ID
	port := inbound.Port
	streamNetwork := stream["network"].(string)
	params := make(map[string]string)
	params["type"] = streamNetwork

	// Add encryption parameter for VLESS from inbound settings
	var settings map[string]any
	json.Unmarshal([]byte(inbound.Settings), &settings)
	if encryption, ok := settings["encryption"].(string); ok {
		params["encryption"] = encryption
	}

	applyShareNetworkParams(stream, streamNetwork, params)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, params)
	}
	security, _ := stream["security"].(string)
	switch security {
	case "tls":
		applyShareTLSParams(stream, params)
		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}
	case "reality":
		applyShareRealityParams(stream, params)
		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}
	default:
		params["security"] = "none"
	}

	externalProxies, _ := stream["externalProxy"].([]any)

	if len(externalProxies) > 0 {
		return s.buildExternalProxyURLLinks(
			externalProxies,
			params,
			security,
			func(dest string, port int) string {
				return fmt.Sprintf("vless://%s@%s:%d", uuid, dest, port)
			},
			func(ep map[string]any) string {
				return s.genRemark(inbound, email, ep["remark"].(string))
			},
		)
	}

	link := fmt.Sprintf("vless://%s@%s:%d", uuid, address, port)
	return buildLinkWithParams(link, params, s.genRemark(inbound, email, ""))
}

func (s *SubService) genTrojanLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.Trojan {
		return ""
	}
	address := s.resolveInboundAddress(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := findClientIndex(clients, email)
	password := clients[clientIndex].Password
	port := inbound.Port
	streamNetwork := stream["network"].(string)
	params := make(map[string]string)
	params["type"] = streamNetwork

	applyShareNetworkParams(stream, streamNetwork, params)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, params)
	}
	security, _ := stream["security"].(string)
	switch security {
	case "tls":
		applyShareTLSParams(stream, params)
	case "reality":
		applyShareRealityParams(stream, params)
		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}
	default:
		params["security"] = "none"
	}

	externalProxies, _ := stream["externalProxy"].([]any)

	if len(externalProxies) > 0 {
		return s.buildExternalProxyURLLinks(
			externalProxies,
			params,
			security,
			func(dest string, port int) string {
				return fmt.Sprintf("trojan://%s@%s:%d", password, dest, port)
			},
			func(ep map[string]any) string {
				return s.genRemark(inbound, email, ep["remark"].(string))
			},
		)
	}

	link := fmt.Sprintf("trojan://%s@%s:%d", password, address, port)
	return buildLinkWithParams(link, params, s.genRemark(inbound, email, ""))
}

func (s *SubService) genShadowsocksLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.Shadowsocks {
		return ""
	}
	address := s.resolveInboundAddress(inbound)
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	clients, _ := s.inboundService.GetClients(inbound)

	var settings map[string]any
	json.Unmarshal([]byte(inbound.Settings), &settings)
	inboundPassword := settings["password"].(string)
	method := settings["method"].(string)
	clientIndex := findClientIndex(clients, email)
	streamNetwork := stream["network"].(string)
	params := make(map[string]string)
	params["type"] = streamNetwork

	applyShareNetworkParams(stream, streamNetwork, params)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, params)
	}

	security, _ := stream["security"].(string)
	if security == "tls" {
		applyShareTLSParams(stream, params)
	}

	encPart := fmt.Sprintf("%s:%s", method, clients[clientIndex].Password)
	if method[0] == '2' {
		encPart = fmt.Sprintf("%s:%s:%s", method, inboundPassword, clients[clientIndex].Password)
	}

	externalProxies, _ := stream["externalProxy"].([]any)

	if len(externalProxies) > 0 {
		proxyParams := cloneStringMap(params)
		proxyParams["security"] = security
		return s.buildExternalProxyURLLinks(
			externalProxies,
			proxyParams,
			security,
			func(dest string, port int) string {
				return fmt.Sprintf("ss://%s@%s:%d", base64.StdEncoding.EncodeToString([]byte(encPart)), dest, port)
			},
			func(ep map[string]any) string {
				return s.genRemark(inbound, email, ep["remark"].(string))
			},
		)
	}

	link := fmt.Sprintf("ss://%s@%s:%d", base64.StdEncoding.EncodeToString([]byte(encPart)), address, inbound.Port)
	return buildLinkWithParams(link, params, s.genRemark(inbound, email, ""))
}

func (s *SubService) genHysteriaLink(inbound *model.Inbound, email string) string {
	if !model.IsHysteria(inbound.Protocol) {
		return ""
	}
	var stream map[string]any
	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := -1
	for i, client := range clients {
		if client.Email == email {
			clientIndex = i
			break
		}
	}
	auth := clients[clientIndex].Auth
	params := make(map[string]string)

	params["security"] = "tls"
	tlsSetting, _ := stream["tlsSettings"].(map[string]any)
	alpns, _ := tlsSetting["alpn"].([]any)
	var alpn []string
	for _, a := range alpns {
		alpn = append(alpn, a.(string))
	}
	if len(alpn) > 0 {
		params["alpn"] = strings.Join(alpn, ",")
	}
	if sniValue, ok := searchKey(tlsSetting, "serverName"); ok {
		params["sni"], _ = sniValue.(string)
	}

	tlsSettings, _ := searchKey(tlsSetting, "settings")
	if tlsSetting != nil {
		if fpValue, ok := searchKey(tlsSettings, "fingerprint"); ok {
			params["fp"], _ = fpValue.(string)
		}
		if insecure, ok := searchKey(tlsSettings, "allowInsecure"); ok {
			if insecure.(bool) {
				params["insecure"] = "1"
			}
		}
	}

	// salamander obfs (Hysteria2). The panel-side link generator already
	// emits these; keep the subscription output in sync so a client has
	// the obfs password to match the server.
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, params)
		if udpMasks, ok := finalmask["udp"].([]any); ok {
			for _, m := range udpMasks {
				mask, _ := m.(map[string]any)
				if mask == nil || mask["type"] != "salamander" {
					continue
				}
				settings, _ := mask["settings"].(map[string]any)
				if pw, ok := settings["password"].(string); ok && pw != "" {
					params["obfs"] = "salamander"
					params["obfs-password"] = pw
					break
				}
			}
		}
	}

	var settings map[string]any
	json.Unmarshal([]byte(inbound.Settings), &settings)
	version, _ := settings["version"].(float64)
	protocol := "hysteria2"
	if int(version) == 1 {
		protocol = "hysteria"
	}

	// Fan out one link per External Proxy entry if any. Previously this
	// generator ignored `externalProxy` entirely, so the link kept the
	// server's own IP/port even when the admin configured an alternate
	// endpoint (e.g. a CDN hostname + port that forwards to the node).
	// Matches the behaviour of genVlessLink / genTrojanLink / ….
	externalProxies, _ := stream["externalProxy"].([]any)
	if len(externalProxies) > 0 {
		links := make([]string, 0, len(externalProxies))
		for _, externalProxy := range externalProxies {
			ep, ok := externalProxy.(map[string]any)
			if !ok {
				continue
			}
			dest, _ := ep["dest"].(string)
			portF, okPort := ep["port"].(float64)
			if dest == "" || !okPort {
				continue
			}
			epRemark, _ := ep["remark"].(string)

			link := fmt.Sprintf("%s://%s@%s:%d", protocol, auth, dest, int(portF))
			u, _ := url.Parse(link)
			q := u.Query()
			for k, v := range params {
				q.Add(k, v)
			}
			u.RawQuery = q.Encode()
			u.Fragment = s.genRemark(inbound, email, epRemark)
			links = append(links, u.String())
		}
		return strings.Join(links, "\n")
	}

	// No external proxy configured — fall back to the request host.
	link := fmt.Sprintf("%s://%s@%s:%d", protocol, auth, s.address, inbound.Port)
	url, _ := url.Parse(link)
	q := url.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	url.RawQuery = q.Encode()
	url.Fragment = s.genRemark(inbound, email, "")
	return url.String()
}

func (s *SubService) resolveInboundAddress(inbound *model.Inbound) string {
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		return s.address
	}
	return inbound.Listen
}

func findClientIndex(clients []model.Client, email string) int {
	for i, client := range clients {
		if client.Email == email {
			return i
		}
	}
	return -1
}

func unmarshalStreamSettings(streamSettings string) map[string]any {
	var stream map[string]any
	json.Unmarshal([]byte(streamSettings), &stream)
	return stream
}

func applyPathAndHostParams(settings map[string]any, params map[string]string) {
	params["path"] = settings["path"].(string)
	if host, ok := settings["host"].(string); ok && len(host) > 0 {
		params["host"] = host
	} else {
		headers, _ := settings["headers"].(map[string]any)
		params["host"] = searchHost(headers)
	}
}

func applyPathAndHostObj(settings map[string]any, obj map[string]any) {
	obj["path"] = settings["path"].(string)
	if host, ok := settings["host"].(string); ok && len(host) > 0 {
		obj["host"] = host
	} else {
		headers, _ := settings["headers"].(map[string]any)
		obj["host"] = searchHost(headers)
	}
}

func applyShareNetworkParams(stream map[string]any, streamNetwork string, params map[string]string) {
	switch streamNetwork {
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]any)
		header, _ := tcp["header"].(map[string]any)
		typeStr, _ := header["type"].(string)
		if typeStr == "http" {
			request := header["request"].(map[string]any)
			requestPath, _ := request["path"].([]any)
			params["path"] = requestPath[0].(string)
			headers, _ := request["headers"].(map[string]any)
			params["host"] = searchHost(headers)
			params["headerType"] = "http"
		}
	case "kcp":
		applyKcpShareParams(stream, params)
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		applyPathAndHostParams(ws, params)
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		params["serviceName"] = grpc["serviceName"].(string)
		params["authority"], _ = grpc["authority"].(string)
		if grpc["multiMode"].(bool) {
			params["mode"] = "multi"
		}
	case "httpupgrade":
		httpupgrade, _ := stream["httpupgradeSettings"].(map[string]any)
		applyPathAndHostParams(httpupgrade, params)
	case "xhttp":
		xhttp, _ := stream["xhttpSettings"].(map[string]any)
		applyPathAndHostParams(xhttp, params)
		params["mode"], _ = xhttp["mode"].(string)
		applyXhttpPaddingParams(xhttp, params)
	}
}

func applyXhttpPaddingObj(xhttp map[string]any, obj map[string]any) {
	// VMess base64 JSON supports arbitrary keys; copy the padding
	// settings through so clients can match the server's xhttp
	// xPaddingBytes range and, when the admin opted into obfs
	// mode, the custom key / header / placement / method.
	if xpb, ok := xhttp["xPaddingBytes"].(string); ok && len(xpb) > 0 {
		obj["x_padding_bytes"] = xpb
	}
	if obfs, ok := xhttp["xPaddingObfsMode"].(bool); ok && obfs {
		obj["xPaddingObfsMode"] = true
		for _, field := range []string{"xPaddingKey", "xPaddingHeader", "xPaddingPlacement", "xPaddingMethod"} {
			if v, ok := xhttp[field].(string); ok && len(v) > 0 {
				obj[field] = v
			}
		}
	}
}

func applyVmessNetworkParams(stream map[string]any, network string, obj map[string]any) {
	obj["net"] = network
	switch network {
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]any)
		header, _ := tcp["header"].(map[string]any)
		typeStr, _ := header["type"].(string)
		obj["type"] = typeStr
		if typeStr == "http" {
			request := header["request"].(map[string]any)
			requestPath, _ := request["path"].([]any)
			obj["path"] = requestPath[0].(string)
			headers, _ := request["headers"].(map[string]any)
			obj["host"] = searchHost(headers)
		}
	case "kcp":
		applyKcpShareObj(stream, obj)
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		applyPathAndHostObj(ws, obj)
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		obj["path"] = grpc["serviceName"].(string)
		obj["authority"] = grpc["authority"].(string)
		if grpc["multiMode"].(bool) {
			obj["type"] = "multi"
		}
	case "httpupgrade":
		httpupgrade, _ := stream["httpupgradeSettings"].(map[string]any)
		applyPathAndHostObj(httpupgrade, obj)
	case "xhttp":
		xhttp, _ := stream["xhttpSettings"].(map[string]any)
		applyPathAndHostObj(xhttp, obj)
		obj["mode"], _ = xhttp["mode"].(string)
		applyXhttpPaddingObj(xhttp, obj)
	}
}

func applyShareTLSParams(stream map[string]any, params map[string]string) {
	params["security"] = "tls"
	tlsSetting, _ := stream["tlsSettings"].(map[string]any)
	alpns, _ := tlsSetting["alpn"].([]any)
	var alpn []string
	for _, a := range alpns {
		alpn = append(alpn, a.(string))
	}
	if len(alpn) > 0 {
		params["alpn"] = strings.Join(alpn, ",")
	}
	if sniValue, ok := searchKey(tlsSetting, "serverName"); ok {
		params["sni"], _ = sniValue.(string)
	}

	tlsSettings, _ := searchKey(tlsSetting, "settings")
	if tlsSetting != nil {
		if fpValue, ok := searchKey(tlsSettings, "fingerprint"); ok {
			params["fp"], _ = fpValue.(string)
		}
	}
}

func applyVmessTLSParams(stream map[string]any, obj map[string]any) {
	tlsSetting, _ := stream["tlsSettings"].(map[string]any)
	alpns, _ := tlsSetting["alpn"].([]any)
	if len(alpns) > 0 {
		var alpn []string
		for _, a := range alpns {
			alpn = append(alpn, a.(string))
		}
		obj["alpn"] = strings.Join(alpn, ",")
	}
	if sniValue, ok := searchKey(tlsSetting, "serverName"); ok {
		obj["sni"], _ = sniValue.(string)
	}

	tlsSettings, _ := searchKey(tlsSetting, "settings")
	if tlsSetting != nil {
		if fpValue, ok := searchKey(tlsSettings, "fingerprint"); ok {
			obj["fp"], _ = fpValue.(string)
		}
	}
}

func applyShareRealityParams(stream map[string]any, params map[string]string) {
	params["security"] = "reality"
	realitySetting, _ := stream["realitySettings"].(map[string]any)
	realitySettings, _ := searchKey(realitySetting, "settings")
	if realitySetting != nil {
		if sniValue, ok := searchKey(realitySetting, "serverNames"); ok {
			sNames, _ := sniValue.([]any)
			params["sni"] = sNames[random.Num(len(sNames))].(string)
		}
		if pbkValue, ok := searchKey(realitySettings, "publicKey"); ok {
			params["pbk"], _ = pbkValue.(string)
		}
		if sidValue, ok := searchKey(realitySetting, "shortIds"); ok {
			shortIds, _ := sidValue.([]any)
			params["sid"] = shortIds[random.Num(len(shortIds))].(string)
		}
		if fpValue, ok := searchKey(realitySettings, "fingerprint"); ok {
			if fp, ok := fpValue.(string); ok && len(fp) > 0 {
				params["fp"] = fp
			}
		}
		if pqvValue, ok := searchKey(realitySettings, "mldsa65Verify"); ok {
			if pqv, ok := pqvValue.(string); ok && len(pqv) > 0 {
				params["pqv"] = pqv
			}
		}
		params["spx"] = "/" + random.Seq(15)
	}
}

func buildVmessLink(obj map[string]any) string {
	jsonStr, _ := json.MarshalIndent(obj, "", "  ")
	return "vmess://" + base64.StdEncoding.EncodeToString(jsonStr)
}

func cloneVmessShareObj(baseObj map[string]any, newSecurity string) map[string]any {
	newObj := map[string]any{}
	for key, value := range baseObj {
		if !(newSecurity == "none" && (key == "alpn" || key == "sni" || key == "fp")) {
			newObj[key] = value
		}
	}
	return newObj
}

func (s *SubService) buildVmessExternalProxyLinks(externalProxies []any, baseObj map[string]any, inbound *model.Inbound, email string) string {
	var links strings.Builder
	for index, externalProxy := range externalProxies {
		ep, _ := externalProxy.(map[string]any)
		newSecurity, _ := ep["forceTls"].(string)
		newObj := cloneVmessShareObj(baseObj, newSecurity)
		newObj["ps"] = s.genRemark(inbound, email, ep["remark"].(string))
		newObj["add"] = ep["dest"].(string)
		newObj["port"] = int(ep["port"].(float64))

		if newSecurity != "same" {
			newObj["tls"] = newSecurity
		}
		if index > 0 {
			links.WriteString("\n")
		}
		links.WriteString(buildVmessLink(newObj))
	}
	return links.String()
}

func buildLinkWithParams(link string, params map[string]string, fragment string) string {
	parsedURL, _ := url.Parse(link)
	q := parsedURL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	parsedURL.RawQuery = q.Encode()
	parsedURL.Fragment = fragment
	return parsedURL.String()
}

func buildLinkWithParamsAndSecurity(link string, params map[string]string, fragment, security string, omitTLSFields bool) string {
	parsedURL, _ := url.Parse(link)
	q := parsedURL.Query()
	for k, v := range params {
		if k == "security" {
			v = security
		}
		if omitTLSFields && (k == "alpn" || k == "sni" || k == "fp") {
			continue
		}
		q.Add(k, v)
	}
	parsedURL.RawQuery = q.Encode()
	parsedURL.Fragment = fragment
	return parsedURL.String()
}

func (s *SubService) buildExternalProxyURLLinks(
	externalProxies []any,
	params map[string]string,
	baseSecurity string,
	makeLink func(dest string, port int) string,
	makeRemark func(ep map[string]any) string,
) string {
	links := make([]string, 0, len(externalProxies))
	for _, externalProxy := range externalProxies {
		ep, _ := externalProxy.(map[string]any)
		newSecurity, _ := ep["forceTls"].(string)
		dest, _ := ep["dest"].(string)
		port := int(ep["port"].(float64))

		securityToApply := baseSecurity
		if newSecurity != "same" {
			securityToApply = newSecurity
		}

		links = append(
			links,
			buildLinkWithParamsAndSecurity(
				makeLink(dest, port),
				params,
				makeRemark(ep),
				securityToApply,
				newSecurity == "none",
			),
		)
	}
	return strings.Join(links, "\n")
}

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}

func (s *SubService) genRemark(inbound *model.Inbound, email string, extra string) string {
	separationChar := string(s.remarkModel[0])
	orderChars := s.remarkModel[1:]
	orders := map[byte]string{
		'i': "",
		'e': "",
		'o': "",
	}
	if len(email) > 0 {
		orders['e'] = email
	}
	if len(inbound.Remark) > 0 {
		orders['i'] = inbound.Remark
	}
	if len(extra) > 0 {
		orders['o'] = extra
	}

	var remark []string
	for i := 0; i < len(orderChars); i++ {
		char := orderChars[i]
		order, exists := orders[char]
		if exists && order != "" {
			remark = append(remark, order)
		}
	}

	if s.showInfo {
		statsExist := false
		var stats xray.ClientTraffic
		for _, clientStat := range inbound.ClientStats {
			if clientStat.Email == email {
				stats = clientStat
				statsExist = true
				break
			}
		}

		// Get remained days
		if statsExist {
			if !stats.Enable {
				return fmt.Sprintf("⛔️N/A%s%s", separationChar, strings.Join(remark, separationChar))
			}
			if vol := stats.Total - (stats.Up + stats.Down); vol > 0 {
				remark = append(remark, fmt.Sprintf("%s%s", common.FormatTraffic(vol), "📊"))
			}
			now := time.Now().Unix()
			switch exp := stats.ExpiryTime / 1000; {
			case exp > 0:
				remainingSeconds := exp - now
				days := remainingSeconds / 86400
				hours := (remainingSeconds % 86400) / 3600
				minutes := (remainingSeconds % 3600) / 60
				if days > 0 {
					if hours > 0 {
						remark = append(remark, fmt.Sprintf("%dD,%dH⏳", days, hours))
					} else {
						remark = append(remark, fmt.Sprintf("%dD⏳", days))
					}
				} else if hours > 0 {
					remark = append(remark, fmt.Sprintf("%dH⏳", hours))
				} else {
					remark = append(remark, fmt.Sprintf("%dM⏳", minutes))
				}
			case exp < 0:
				days := exp / -86400
				hours := (exp % -86400) / 3600
				minutes := (exp % -3600) / 60
				if days > 0 {
					if hours > 0 {
						remark = append(remark, fmt.Sprintf("%dD,%dH⏳", days, hours))
					} else {
						remark = append(remark, fmt.Sprintf("%dD⏳", days))
					}
				} else if hours > 0 {
					remark = append(remark, fmt.Sprintf("%dH⏳", hours))
				} else {
					remark = append(remark, fmt.Sprintf("%dM⏳", minutes))
				}
			}
		}
	}
	return strings.Join(remark, separationChar)
}

func searchKey(data any, key string) (any, bool) {
	switch val := data.(type) {
	case map[string]any:
		for k, v := range val {
			if k == key {
				return v, true
			}
			if result, ok := searchKey(v, key); ok {
				return result, true
			}
		}
	case []any:
		for _, v := range val {
			if result, ok := searchKey(v, key); ok {
				return result, true
			}
		}
	}
	return nil, false
}

// applyXhttpPaddingParams copies the xPadding* fields from an xhttpSettings
// map into the URL query params of a vless:// / trojan:// / ss:// link.
//
// Before this helper existed, only path / host / mode were propagated,
// so a server configured with a non-default xPaddingBytes (e.g. 80-600)
// or with xPaddingObfsMode=true + custom xPaddingKey / xPaddingHeader
// would silently diverge from the client: the client kept defaults,
// hit the server, and was rejected by its padding validation
// ("invalid padding" in the inbound log) — the client-visible symptom
// was "xhttp doesn't connect" on OpenWRT / sing-box.
//
// Two encodings are written so every popular client can read at least one:
//
//   - x_padding_bytes=<range>  — flat param, understood by sing-box and its
//     derivatives (Podkop, OpenWRT sing-box, Karing, NekoBox, …).
//   - extra=<url-encoded-json> — full xhttp settings blob, which is how
//     xray-core clients (v2rayNG, Happ, Furious, Exclave, …) pick up the
//     obfs-mode key / header / placement / method.
//
// Anything that doesn't map to a non-empty value is skipped, so simple
// inbounds (no custom padding) produce exactly the same URL as before.
func applyXhttpPaddingParams(xhttp map[string]any, params map[string]string) {
	if xhttp == nil {
		return
	}

	if xpb, ok := xhttp["xPaddingBytes"].(string); ok && len(xpb) > 0 {
		params["x_padding_bytes"] = xpb
	}

	extra := map[string]any{}
	if xpb, ok := xhttp["xPaddingBytes"].(string); ok && len(xpb) > 0 {
		extra["xPaddingBytes"] = xpb
	}
	if obfs, ok := xhttp["xPaddingObfsMode"].(bool); ok && obfs {
		extra["xPaddingObfsMode"] = true
		// The obfs-mode-only fields: only populate the ones the admin
		// actually set, so xray-core falls back to its own defaults for
		// the rest instead of seeing spurious empty strings.
		for _, field := range []string{"xPaddingKey", "xPaddingHeader", "xPaddingPlacement", "xPaddingMethod"} {
			if v, ok := xhttp[field].(string); ok && len(v) > 0 {
				extra[field] = v
			}
		}
	}

	if len(extra) > 0 {
		if b, err := json.Marshal(extra); err == nil {
			params["extra"] = string(b)
		}
	}
}

var kcpMaskToHeaderType = map[string]string{
	"header-dns":       "dns",
	"header-dtls":      "dtls",
	"header-srtp":      "srtp",
	"header-utp":       "utp",
	"header-wechat":    "wechat-video",
	"header-wireguard": "wireguard",
}

var validFinalMaskUDPTypes = map[string]struct{}{
	"salamander":       {},
	"mkcp-aes128gcm":   {},
	"header-dns":       {},
	"header-dtls":      {},
	"header-srtp":      {},
	"header-utp":       {},
	"header-wechat":    {},
	"header-wireguard": {},
	"mkcp-original":    {},
	"xdns":             {},
	"xicmp":            {},
	"noise":            {},
	"header-custom":    {},
}

var validFinalMaskTCPTypes = map[string]struct{}{
	"header-custom": {},
	"fragment":      {},
	"sudoku":        {},
}

// applyKcpShareParams reconstructs legacy KCP share-link fields from either
// the historical kcpSettings.header/seed shape or the current finalmask model.
// This keeps subscription output compatible while avoiding panics when older
// keys are absent from modern inbounds.
func applyKcpShareParams(stream map[string]any, params map[string]string) {
	extractKcpShareFields(stream).applyToParams(params)
}

func applyKcpShareObj(stream map[string]any, obj map[string]any) {
	extractKcpShareFields(stream).applyToObj(obj)
}

type kcpShareFields struct {
	headerType string
	seed       string
	mtu        int
	tti        int
}

func (f kcpShareFields) applyToParams(params map[string]string) {
	if f.headerType != "" && f.headerType != "none" {
		params["headerType"] = f.headerType
	}
	setStringParam(params, "seed", f.seed)
	setIntParam(params, "mtu", f.mtu)
	setIntParam(params, "tti", f.tti)
}

func (f kcpShareFields) applyToObj(obj map[string]any) {
	if f.headerType != "" && f.headerType != "none" {
		obj["type"] = f.headerType
	}
	setStringField(obj, "path", f.seed)
	setIntField(obj, "mtu", f.mtu)
	setIntField(obj, "tti", f.tti)
}

func extractKcpShareFields(stream map[string]any) kcpShareFields {
	fields := kcpShareFields{headerType: "none"}

	if kcp, ok := stream["kcpSettings"].(map[string]any); ok {
		if header, ok := kcp["header"].(map[string]any); ok {
			if value, ok := header["type"].(string); ok && value != "" {
				fields.headerType = value
			}
		}
		if value, ok := kcp["seed"].(string); ok && value != "" {
			fields.seed = value
		}
		if value, ok := readPositiveInt(kcp["mtu"]); ok {
			fields.mtu = value
		}
		if value, ok := readPositiveInt(kcp["tti"]); ok {
			fields.tti = value
		}
	}

	for _, rawMask := range normalizedFinalMaskUDPMasks(stream["finalmask"]) {
		mask, _ := rawMask.(map[string]any)
		if mask == nil {
			continue
		}
		maskType, _ := mask["type"].(string)
		if mapped, ok := kcpMaskToHeaderType[maskType]; ok {
			fields.headerType = mapped
			continue
		}

		switch maskType {
		case "mkcp-original":
			fields.seed = ""
		case "mkcp-aes128gcm":
			fields.seed = ""
			settings, _ := mask["settings"].(map[string]any)
			if value, ok := settings["password"].(string); ok && value != "" {
				fields.seed = value
			}
		}
	}

	return fields
}

func readPositiveInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, number > 0
	case int32:
		return int(number), number > 0
	case int64:
		return int(number), number > 0
	case float32:
		parsed := int(number)
		return parsed, parsed > 0
	case float64:
		parsed := int(number)
		return parsed, parsed > 0
	default:
		return 0, false
	}
}

func setStringParam(params map[string]string, key, value string) {
	if value == "" {
		delete(params, key)
		return
	}
	params[key] = value
}

func setIntParam(params map[string]string, key string, value int) {
	if value <= 0 {
		delete(params, key)
		return
	}
	params[key] = fmt.Sprintf("%d", value)
}

func setStringField(obj map[string]any, key, value string) {
	if value == "" {
		delete(obj, key)
		return
	}
	obj[key] = value
}

func setIntField(obj map[string]any, key string, value int) {
	if value <= 0 {
		delete(obj, key)
		return
	}
	obj[key] = value
}

// applyFinalMaskParams exports the finalmask payload as the compact
// `fm=<json>` share-link field used by v2rayN-compatible clients.
func applyFinalMaskParams(finalmask map[string]any, params map[string]string) {
	if fm, ok := marshalFinalMask(finalmask); ok {
		params["fm"] = fm
	}
}

func applyFinalMaskObj(finalmask map[string]any, obj map[string]any) {
	if fm, ok := marshalFinalMask(finalmask); ok {
		obj["fm"] = fm
	}
}

func marshalFinalMask(finalmask map[string]any) (string, bool) {
	normalized := normalizeFinalMask(finalmask)
	if !hasFinalMaskContent(normalized) {
		return "", false
	}
	b, err := json.Marshal(normalized)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return "", false
	}
	return string(b), true
}

func normalizeFinalMask(finalmask map[string]any) map[string]any {
	tcpMasks := normalizedFinalMaskTCPMasks(finalmask)
	udpMasks := normalizedFinalMaskUDPMasks(finalmask)
	quicParams, hasQuicParams := finalmask["quicParams"].(map[string]any)

	if len(tcpMasks) == 0 && len(udpMasks) == 0 && !hasQuicParams {
		return nil
	}

	result := map[string]any{}
	if len(tcpMasks) > 0 {
		result["tcp"] = tcpMasks
	}
	if len(udpMasks) > 0 {
		result["udp"] = udpMasks
	}
	if hasQuicParams && len(quicParams) > 0 {
		result["quicParams"] = quicParams
	}
	return result
}

func normalizedFinalMaskTCPMasks(value any) []any {
	finalmask, _ := value.(map[string]any)
	if finalmask == nil {
		return nil
	}
	rawMasks, _ := finalmask["tcp"].([]any)
	if len(rawMasks) == 0 {
		return nil
	}

	normalized := make([]any, 0, len(rawMasks))
	for _, rawMask := range rawMasks {
		mask, _ := rawMask.(map[string]any)
		if mask == nil {
			continue
		}
		maskType, _ := mask["type"].(string)
		if _, ok := validFinalMaskTCPTypes[maskType]; !ok || maskType == "" {
			continue
		}

		normalizedMask := map[string]any{"type": maskType}
		if settings, ok := mask["settings"].(map[string]any); ok && len(settings) > 0 {
			normalizedMask["settings"] = settings
		}
		normalized = append(normalized, normalizedMask)
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizedFinalMaskUDPMasks(value any) []any {
	finalmask, _ := value.(map[string]any)
	if finalmask == nil {
		return nil
	}
	rawMasks, _ := finalmask["udp"].([]any)
	if len(rawMasks) == 0 {
		return nil
	}

	normalized := make([]any, 0, len(rawMasks))
	for _, rawMask := range rawMasks {
		mask, _ := rawMask.(map[string]any)
		if mask == nil {
			continue
		}
		maskType, _ := mask["type"].(string)
		if _, ok := validFinalMaskUDPTypes[maskType]; !ok || maskType == "" {
			continue
		}

		normalizedMask := map[string]any{"type": maskType}
		if settings, ok := mask["settings"].(map[string]any); ok && len(settings) > 0 {
			normalizedMask["settings"] = settings
		}
		normalized = append(normalized, normalizedMask)
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func hasFinalMaskContent(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return len(v) > 0
	case map[string]any:
		for _, item := range v {
			if hasFinalMaskContent(item) {
				return true
			}
		}
		return false
	case []any:
		return slices.ContainsFunc(v, hasFinalMaskContent)
	default:
		return true
	}
}

func searchHost(headers any) string {
	data, _ := headers.(map[string]any)
	for k, v := range data {
		if strings.EqualFold(k, "host") {
			switch v.(type) {
			case []any:
				hosts, _ := v.([]any)
				if len(hosts) > 0 {
					return hosts[0].(string)
				} else {
					return ""
				}
			case any:
				return v.(string)
			}
		}
	}

	return ""
}

// PageData is a view model for subpage.html
// PageData contains data for rendering the subscription information page.
type PageData struct {
	Host         string
	BasePath     string
	SId          string
	Download     string
	Upload       string
	Total        string
	Used         string
	Remained     string
	Expire       int64
	LastOnline   int64
	Datepicker   string
	DownloadByte int64
	UploadByte   int64
	TotalByte    int64
	SubUrl       string
	SubJsonUrl   string
	SubClashUrl  string
	Result       []string
}

// ResolveRequest extracts scheme and host info from request/headers consistently.
// ResolveRequest extracts scheme, host, and header information from an HTTP request.
func (s *SubService) ResolveRequest(c *gin.Context) (scheme string, host string, hostWithPort string, hostHeader string) {
	// scheme
	scheme = "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}

	// base host (no port)
	if h, err := getHostFromXFH(c.GetHeader("X-Forwarded-Host")); err == nil && h != "" {
		host = h
	}
	if host == "" {
		host = c.GetHeader("X-Real-IP")
	}
	if host == "" {
		var err error
		host, _, err = net.SplitHostPort(c.Request.Host)
		if err != nil {
			host = c.Request.Host
		}
	}

	// host:port for URLs
	hostWithPort = c.GetHeader("X-Forwarded-Host")
	if hostWithPort == "" {
		hostWithPort = c.Request.Host
	}
	if hostWithPort == "" {
		hostWithPort = host
	}

	// header display host
	hostHeader = c.GetHeader("X-Forwarded-Host")
	if hostHeader == "" {
		hostHeader = c.GetHeader("X-Real-IP")
	}
	if hostHeader == "" {
		hostHeader = host
	}
	return
}

// BuildURLs constructs absolute subscription and JSON subscription URLs for a given subscription ID.
// It prioritizes configured URIs, then individual settings, and finally falls back to request-derived components.
func (s *SubService) BuildURLs(scheme, hostWithPort, subPath, subJsonPath, subClashPath, subId string) (subURL, subJsonURL, subClashURL string) {
	if subId == "" {
		return "", "", ""
	}

	configuredSubURI, _ := s.settingService.GetSubURI()
	configuredSubJsonURI, _ := s.settingService.GetSubJsonURI()
	configuredSubClashURI, _ := s.settingService.GetSubClashURI()

	var baseScheme, baseHostWithPort string
	if configuredSubURI == "" || configuredSubJsonURI == "" || configuredSubClashURI == "" {
		baseScheme, baseHostWithPort = s.getBaseSchemeAndHost(scheme, hostWithPort)
	}

	subURL = s.buildSingleURL(configuredSubURI, baseScheme, baseHostWithPort, subPath, subId)
	subJsonURL = s.buildSingleURL(configuredSubJsonURI, baseScheme, baseHostWithPort, subJsonPath, subId)
	subClashURL = s.buildSingleURL(configuredSubClashURI, baseScheme, baseHostWithPort, subClashPath, subId)

	return subURL, subJsonURL, subClashURL
}

// getBaseSchemeAndHost determines the base scheme and host from settings or falls back to request values
func (s *SubService) getBaseSchemeAndHost(requestScheme, requestHostWithPort string) (string, string) {
	subDomain, err := s.settingService.GetSubDomain()
	if err != nil || subDomain == "" {
		return requestScheme, requestHostWithPort
	}

	// Get port and TLS settings
	subPort, _ := s.settingService.GetSubPort()
	subKeyFile, _ := s.settingService.GetSubKeyFile()
	subCertFile, _ := s.settingService.GetSubCertFile()

	// Determine scheme from TLS configuration
	scheme := "http"
	if subKeyFile != "" && subCertFile != "" {
		scheme = "https"
	}

	// Build host:port, always include port for clarity
	hostWithPort := fmt.Sprintf("%s:%d", subDomain, subPort)

	return scheme, hostWithPort
}

// buildSingleURL constructs a single URL using configured URI or base components
func (s *SubService) buildSingleURL(configuredURI, baseScheme, baseHostWithPort, basePath, subId string) string {
	if configuredURI != "" {
		return s.joinPathWithID(configuredURI, subId)
	}

	baseURL := fmt.Sprintf("%s://%s", baseScheme, baseHostWithPort)
	return s.joinPathWithID(baseURL+basePath, subId)
}

// joinPathWithID safely joins a base path with a subscription ID
func (s *SubService) joinPathWithID(basePath, subId string) string {
	if strings.HasSuffix(basePath, "/") {
		return basePath + subId
	}
	return basePath + "/" + subId
}

// BuildPageData parses header and prepares the template view model.
// BuildPageData constructs page data for rendering the subscription information page.
func (s *SubService) BuildPageData(subId string, hostHeader string, traffic xray.ClientTraffic, lastOnline int64, subs []string, subURL, subJsonURL, subClashURL string, basePath string) PageData {
	download := common.FormatTraffic(traffic.Down)
	upload := common.FormatTraffic(traffic.Up)
	total := "∞"
	used := common.FormatTraffic(traffic.Up + traffic.Down)
	remained := ""
	if traffic.Total > 0 {
		total = common.FormatTraffic(traffic.Total)
		left := max(traffic.Total-(traffic.Up+traffic.Down), 0)
		remained = common.FormatTraffic(left)
	}

	datepicker := s.datepicker
	if datepicker == "" {
		datepicker = "gregorian"
	}

	return PageData{
		Host:         hostHeader,
		BasePath:     basePath,
		SId:          subId,
		Download:     download,
		Upload:       upload,
		Total:        total,
		Used:         used,
		Remained:     remained,
		Expire:       traffic.ExpiryTime / 1000,
		LastOnline:   lastOnline,
		Datepicker:   datepicker,
		DownloadByte: traffic.Down,
		UploadByte:   traffic.Up,
		TotalByte:    traffic.Total,
		SubUrl:       subURL,
		SubJsonUrl:   subJsonURL,
		SubClashUrl:  subClashURL,
		Result:       subs,
	}
}

func getHostFromXFH(s string) (string, error) {
	if strings.Contains(s, ":") {
		realHost, _, err := net.SplitHostPort(s)
		if err != nil {
			return "", err
		}
		return realHost, nil
	}
	return s, nil
}
