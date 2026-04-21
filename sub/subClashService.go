package sub

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"
	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

type SubClashService struct {
	inboundService service.InboundService
	SubService     *SubService
}

type ClashConfig struct {
	Proxies     []map[string]any `yaml:"proxies"`
	ProxyGroups []map[string]any `yaml:"proxy-groups"`
	Rules       []string         `yaml:"rules"`
}

func NewSubClashService(subService *SubService) *SubClashService {
	return &SubClashService{SubService: subService}
}

func (s *SubClashService) GetClash(subId string, host string) (string, string, error) {
	inbounds, err := s.SubService.getInboundsBySubId(subId)
	if err != nil || len(inbounds) == 0 {
		return "", "", err
	}

	var traffic xray.ClientTraffic
	var clientTraffics []xray.ClientTraffic
	var proxies []map[string]any

	for _, inbound := range inbounds {
		clients, err := s.inboundService.GetClients(inbound)
		if err != nil {
			logger.Error("SubClashService - GetClients: Unable to get clients from inbound")
		}
		if clients == nil {
			continue
		}
		if len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
			listen, port, streamSettings, err := s.SubService.getFallbackMaster(inbound.Listen, inbound.StreamSettings)
			if err == nil {
				inbound.Listen = listen
				inbound.Port = port
				inbound.StreamSettings = streamSettings
			}
		}
		for _, client := range clients {
			if client.Enable && client.SubID == subId {
				clientTraffics = append(clientTraffics, s.SubService.getClientTraffics(inbound.ClientStats, client.Email))
				proxies = append(proxies, s.getProxies(inbound, client, host)...)
			}
		}
	}

	if len(proxies) == 0 {
		return "", "", nil
	}

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

	proxyNames := make([]string, 0, len(proxies)+1)
	for _, proxy := range proxies {
		if name, ok := proxy["name"].(string); ok && name != "" {
			proxyNames = append(proxyNames, name)
		}
	}
	proxyNames = append(proxyNames, "DIRECT")

	config := ClashConfig{
		Proxies: proxies,
		ProxyGroups: []map[string]any{{
			"name":    "PROXY",
			"type":    "select",
			"proxies": proxyNames,
		}},
		Rules: []string{"MATCH,PROXY"},
	}

	finalYAML, err := yaml.Marshal(config)
	if err != nil {
		return "", "", err
	}

	header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(finalYAML), header, nil
}

func (s *SubClashService) getProxies(inbound *model.Inbound, client model.Client, host string) []map[string]any {
	stream := s.streamData(inbound.StreamSettings)
	externalProxies, ok := stream["externalProxy"].([]any)
	if !ok || len(externalProxies) == 0 {
		externalProxies = []any{map[string]any{
			"forceTls": "same",
			"dest":     host,
			"port":     float64(inbound.Port),
			"remark":   "",
		}}
	}
	delete(stream, "externalProxy")

	proxies := make([]map[string]any, 0, len(externalProxies))
	for _, ep := range externalProxies {
		extPrxy := ep.(map[string]any)
		workingInbound := *inbound
		workingInbound.Listen = extPrxy["dest"].(string)
		workingInbound.Port = int(extPrxy["port"].(float64))
		workingStream := cloneMap(stream)

		switch extPrxy["forceTls"].(string) {
		case "tls":
			if workingStream["security"] != "tls" {
				workingStream["security"] = "tls"
				workingStream["tlsSettings"] = map[string]any{}
			}
		case "none":
			if workingStream["security"] != "none" {
				workingStream["security"] = "none"
				delete(workingStream, "tlsSettings")
				delete(workingStream, "realitySettings")
			}
		}

		proxy := s.buildProxy(&workingInbound, client, workingStream, extPrxy["remark"].(string))
		if len(proxy) > 0 {
			proxies = append(proxies, proxy)
		}
	}
	return proxies
}

func (s *SubClashService) buildProxy(inbound *model.Inbound, client model.Client, stream map[string]any, extraRemark string) map[string]any {
	proxy := map[string]any{
		"name":   s.SubService.genRemark(inbound, client.Email, extraRemark),
		"server": inbound.Listen,
		"port":   inbound.Port,
		"udp":    true,
	}

	network, _ := stream["network"].(string)
	if !s.applyTransport(proxy, network, stream) {
		return nil
	}

	switch inbound.Protocol {
	case model.VMESS:
		proxy["type"] = "vmess"
		proxy["uuid"] = client.ID
		proxy["alterId"] = 0
		cipher := client.Security
		if cipher == "" {
			cipher = "auto"
		}
		proxy["cipher"] = cipher
	case model.VLESS:
		proxy["type"] = "vless"
		proxy["uuid"] = client.ID
		if client.Flow != "" && network == "tcp" {
			proxy["flow"] = client.Flow
		}
		var inboundSettings map[string]any
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		if encryption, ok := inboundSettings["encryption"].(string); ok && encryption != "" {
			proxy["packet-encoding"] = encryption
		}
	case model.Trojan:
		proxy["type"] = "trojan"
		proxy["password"] = client.Password
	case model.Shadowsocks:
		proxy["type"] = "ss"
		proxy["password"] = client.Password
		var inboundSettings map[string]any
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		method, _ := inboundSettings["method"].(string)
		if method == "" {
			return nil
		}
		proxy["cipher"] = method
		if strings.HasPrefix(method, "2022") {
			if serverPassword, ok := inboundSettings["password"].(string); ok && serverPassword != "" {
				proxy["password"] = fmt.Sprintf("%s:%s", serverPassword, client.Password)
			}
		}
	default:
		return nil
	}

	security, _ := stream["security"].(string)
	if !s.applySecurity(proxy, security, stream) {
		return nil
	}

	return proxy
}

func (s *SubClashService) applyTransport(proxy map[string]any, network string, stream map[string]any) bool {
	switch network {
	case "", "tcp":
		proxy["network"] = "tcp"
		tcp, _ := stream["tcpSettings"].(map[string]any)
		if tcp != nil {
			header, _ := tcp["header"].(map[string]any)
			if header != nil {
				typeStr, _ := header["type"].(string)
				if typeStr != "" && typeStr != "none" {
					return false
				}
			}
		}
		return true
	case "ws":
		proxy["network"] = "ws"
		ws, _ := stream["wsSettings"].(map[string]any)
		wsOpts := map[string]any{}
		if ws != nil {
			if path, ok := ws["path"].(string); ok && path != "" {
				wsOpts["path"] = path
			}
			host := ""
			if v, ok := ws["host"].(string); ok && v != "" {
				host = v
			} else if headers, ok := ws["headers"].(map[string]any); ok {
				host = searchHost(headers)
			}
			if host != "" {
				wsOpts["headers"] = map[string]any{"Host": host}
			}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
		return true
	case "grpc":
		proxy["network"] = "grpc"
		grpc, _ := stream["grpcSettings"].(map[string]any)
		grpcOpts := map[string]any{}
		if grpc != nil {
			if serviceName, ok := grpc["serviceName"].(string); ok && serviceName != "" {
				grpcOpts["grpc-service-name"] = serviceName
			}
		}
		if len(grpcOpts) > 0 {
			proxy["grpc-opts"] = grpcOpts
		}
		return true
	default:
		return false
	}
}

func (s *SubClashService) applySecurity(proxy map[string]any, security string, stream map[string]any) bool {
	switch security {
	case "", "none":
		proxy["tls"] = false
		return true
	case "tls":
		proxy["tls"] = true
		tlsSettings, _ := stream["tlsSettings"].(map[string]any)
		if tlsSettings != nil {
			if serverName, ok := tlsSettings["serverName"].(string); ok && serverName != "" {
				proxy["servername"] = serverName
				switch proxy["type"] {
				case "trojan":
					proxy["sni"] = serverName
				}
			}
			if fingerprint, ok := tlsSettings["fingerprint"].(string); ok && fingerprint != "" {
				proxy["client-fingerprint"] = fingerprint
			}
		}
		return true
	case "reality":
		proxy["tls"] = true
		realitySettings, _ := stream["realitySettings"].(map[string]any)
		if realitySettings == nil {
			return false
		}
		if serverName, ok := realitySettings["serverName"].(string); ok && serverName != "" {
			proxy["servername"] = serverName
		}
		realityOpts := map[string]any{}
		if publicKey, ok := realitySettings["publicKey"].(string); ok && publicKey != "" {
			realityOpts["public-key"] = publicKey
		}
		if shortID, ok := realitySettings["shortId"].(string); ok && shortID != "" {
			realityOpts["short-id"] = shortID
		}
		if len(realityOpts) > 0 {
			proxy["reality-opts"] = realityOpts
		}
		if fingerprint, ok := realitySettings["fingerprint"].(string); ok && fingerprint != "" {
			proxy["client-fingerprint"] = fingerprint
		}
		return true
	default:
		return false
	}
}

func (s *SubClashService) streamData(stream string) map[string]any {
	var streamSettings map[string]any
	json.Unmarshal([]byte(stream), &streamSettings)
	security, _ := streamSettings["security"].(string)
	switch security {
	case "tls":
		if tlsSettings, ok := streamSettings["tlsSettings"].(map[string]any); ok {
			streamSettings["tlsSettings"] = s.tlsData(tlsSettings)
		}
	case "reality":
		if realitySettings, ok := streamSettings["realitySettings"].(map[string]any); ok {
			streamSettings["realitySettings"] = s.realityData(realitySettings)
		}
	}
	delete(streamSettings, "sockopt")
	return streamSettings
}

func (s *SubClashService) tlsData(tData map[string]any) map[string]any {
	tlsData := make(map[string]any, 1)
	tlsClientSettings, _ := tData["settings"].(map[string]any)
	tlsData["serverName"] = tData["serverName"]
	tlsData["alpn"] = tData["alpn"]
	if fingerprint, ok := tlsClientSettings["fingerprint"].(string); ok {
		tlsData["fingerprint"] = fingerprint
	}
	return tlsData
}

func (s *SubClashService) realityData(rData map[string]any) map[string]any {
	rDataOut := make(map[string]any, 1)
	realityClientSettings, _ := rData["settings"].(map[string]any)
	if publicKey, ok := realityClientSettings["publicKey"].(string); ok {
		rDataOut["publicKey"] = publicKey
	}
	if fingerprint, ok := realityClientSettings["fingerprint"].(string); ok {
		rDataOut["fingerprint"] = fingerprint
	}
	if serverNames, ok := rData["serverNames"].([]any); ok && len(serverNames) > 0 {
		rDataOut["serverName"] = fmt.Sprint(serverNames[0])
	}
	if shortIDs, ok := rData["shortIds"].([]any); ok && len(shortIDs) > 0 {
		rDataOut["shortId"] = fmt.Sprint(shortIDs[0])
	}
	return rDataOut
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
