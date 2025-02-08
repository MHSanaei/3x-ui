package sub

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/json_util"
	"x-ui/util/random"
	"x-ui/web/service"
	"x-ui/xray"
)

//go:embed default.json
var defaultJson string

type SubJsonService struct {
	configJson       map[string]interface{}
	defaultOutbounds []json_util.RawMessage
	fragment         string
	noises           string
	mux              string

	inboundService service.InboundService
	SubService     *SubService
}

func NewSubJsonService(fragment string, noises string, mux string, rules string, subService *SubService) *SubJsonService {
	var configJson map[string]interface{}
	var defaultOutbounds []json_util.RawMessage
	json.Unmarshal([]byte(defaultJson), &configJson)
	if outboundSlices, ok := configJson["outbounds"].([]interface{}); ok {
		for _, defaultOutbound := range outboundSlices {
			jsonBytes, _ := json.Marshal(defaultOutbound)
			defaultOutbounds = append(defaultOutbounds, jsonBytes)
		}
	}

	if rules != "" {
		var newRules []interface{}
		routing, _ := configJson["routing"].(map[string]interface{})
		defaultRules, _ := routing["rules"].([]interface{})
		json.Unmarshal([]byte(rules), &newRules)
		defaultRules = append(newRules, defaultRules...)
		routing["rules"] = defaultRules
		configJson["routing"] = routing
	}

	if fragment != "" {
		defaultOutbounds = append(defaultOutbounds, json_util.RawMessage(fragment))
	}

	if noises != "" {
		defaultOutbounds = append(defaultOutbounds, json_util.RawMessage(noises))
	}

	return &SubJsonService{
		configJson:       configJson,
		defaultOutbounds: defaultOutbounds,
		fragment:         fragment,
		noises:           noises,
		mux:              mux,
		SubService:       subService,
	}
}

func (s *SubJsonService) GetJson(subId string, host string) (string, string, error) {
	inbounds, err := s.SubService.getInboundsBySubId(subId)
	if err != nil || len(inbounds) == 0 {
		return "", "", err
	}

	var header string
	var traffic xray.ClientTraffic
	var clientTraffics []xray.ClientTraffic
	var configArray []json_util.RawMessage

	// Prepare Inbounds
	for _, inbound := range inbounds {
		clients, err := s.inboundService.GetClients(inbound)
		if err != nil {
			logger.Error("SubJsonService - GetClients: Unable to get clients from inbound")
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
				newConfigs := s.getConfig(inbound, client, host)
				configArray = append(configArray, newConfigs...)
			}
		}
	}

	if len(configArray) == 0 {
		return "", "", nil
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

	// Combile outbounds
	var finalJson []byte
	if len(configArray) == 1 {
		finalJson, _ = json.MarshalIndent(configArray[0], "", "  ")
	} else {
		finalJson, _ = json.MarshalIndent(configArray, "", "  ")
	}

	header = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(finalJson), header, nil
}

func (s *SubJsonService) getConfig(inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
	var newJsonArray []json_util.RawMessage
	stream := s.streamData(inbound.StreamSettings)

	externalProxies, ok := stream["externalProxy"].([]interface{})
	if !ok || len(externalProxies) == 0 {
		externalProxies = []interface{}{
			map[string]interface{}{
				"forceTls": "same",
				"dest":     host,
				"port":     float64(inbound.Port),
				"remark":   "",
			},
		}
	}

	delete(stream, "externalProxy")

	for _, ep := range externalProxies {
		extPrxy := ep.(map[string]interface{})
		inbound.Listen = extPrxy["dest"].(string)
		inbound.Port = int(extPrxy["port"].(float64))
		newStream := stream
		switch extPrxy["forceTls"].(string) {
		case "tls":
			if newStream["security"] != "tls" {
				newStream["security"] = "tls"
				newStream["tslSettings"] = map[string]interface{}{}
			}
		case "none":
			if newStream["security"] != "none" {
				newStream["security"] = "none"
				delete(newStream, "tslSettings")
			}
		}
		streamSettings, _ := json.MarshalIndent(newStream, "", "  ")

		var newOutbounds []json_util.RawMessage

		switch inbound.Protocol {
		case "vmess", "vless":
			newOutbounds = append(newOutbounds, s.genVnext(inbound, streamSettings, client))
		case "trojan", "shadowsocks":
			newOutbounds = append(newOutbounds, s.genServer(inbound, streamSettings, client))
		}

		newOutbounds = append(newOutbounds, s.defaultOutbounds...)
		newConfigJson := make(map[string]interface{})
		for key, value := range s.configJson {
			newConfigJson[key] = value
		}
		newConfigJson["outbounds"] = newOutbounds
		newConfigJson["remarks"] = s.SubService.genRemark(inbound, client.Email, extPrxy["remark"].(string))

		newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
		newJsonArray = append(newJsonArray, newConfig)
	}

	return newJsonArray
}

func (s *SubJsonService) streamData(stream string) map[string]interface{} {
	var streamSettings map[string]interface{}
	json.Unmarshal([]byte(stream), &streamSettings)
	security, _ := streamSettings["security"].(string)
	if security == "tls" {
		streamSettings["tlsSettings"] = s.tlsData(streamSettings["tlsSettings"].(map[string]interface{}))
	} else if security == "reality" {
		streamSettings["realitySettings"] = s.realityData(streamSettings["realitySettings"].(map[string]interface{}))
	}
	delete(streamSettings, "sockopt")

	if s.fragment != "" {
		streamSettings["sockopt"] = json_util.RawMessage(`{"dialerProxy": "fragment", "tcpKeepAliveIdle": 100, "tcpMptcp": true, "penetrate": true}`)
	}

	// remove proxy protocol
	network, _ := streamSettings["network"].(string)
	switch network {
	case "tcp":
		streamSettings["tcpSettings"] = s.removeAcceptProxy(streamSettings["tcpSettings"])
	case "ws":
		streamSettings["wsSettings"] = s.removeAcceptProxy(streamSettings["wsSettings"])
	case "httpupgrade":
		streamSettings["httpupgradeSettings"] = s.removeAcceptProxy(streamSettings["httpupgradeSettings"])
	}
	return streamSettings
}

func (s *SubJsonService) removeAcceptProxy(setting interface{}) map[string]interface{} {
	netSettings, ok := setting.(map[string]interface{})
	if ok {
		delete(netSettings, "acceptProxyProtocol")
	}
	return netSettings
}

func (s *SubJsonService) tlsData(tData map[string]interface{}) map[string]interface{} {
	tlsData := make(map[string]interface{}, 1)
	tlsClientSettings, _ := tData["settings"].(map[string]interface{})

	tlsData["serverName"] = tData["serverName"]
	tlsData["alpn"] = tData["alpn"]
	if allowInsecure, ok := tlsClientSettings["allowInsecure"].(bool); ok {
		tlsData["allowInsecure"] = allowInsecure
	}
	if fingerprint, ok := tlsClientSettings["fingerprint"].(string); ok {
		tlsData["fingerprint"] = fingerprint
	}
	return tlsData
}

func (s *SubJsonService) realityData(rData map[string]interface{}) map[string]interface{} {
	rltyData := make(map[string]interface{}, 1)
	rltyClientSettings, _ := rData["settings"].(map[string]interface{})

	rltyData["show"] = false
	rltyData["publicKey"] = rltyClientSettings["publicKey"]
	rltyData["fingerprint"] = rltyClientSettings["fingerprint"]

	// Set random data
	rltyData["spiderX"] = "/" + random.Seq(15)
	shortIds, ok := rData["shortIds"].([]interface{})
	if ok && len(shortIds) > 0 {
		rltyData["shortId"] = shortIds[random.Num(len(shortIds))].(string)
	} else {
		rltyData["shortId"] = ""
	}
	serverNames, ok := rData["serverNames"].([]interface{})
	if ok && len(serverNames) > 0 {
		rltyData["serverName"] = serverNames[random.Num(len(serverNames))].(string)
	} else {
		rltyData["serverName"] = ""
	}

	return rltyData
}

func (s *SubJsonService) genVnext(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client) json_util.RawMessage {
	outbound := Outbound{}
	usersData := make([]UserVnext, 1)

	usersData[0].ID = client.ID
	usersData[0].Level = 8
	if inbound.Protocol == model.VMESS {
		usersData[0].Security = client.Security
	}
	if inbound.Protocol == model.VLESS {
		usersData[0].Flow = client.Flow
		usersData[0].Encryption = "none"
	}

	vnextData := make([]VnextSetting, 1)
	vnextData[0] = VnextSetting{
		Address: inbound.Listen,
		Port:    inbound.Port,
		Users:   usersData,
	}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if s.mux != "" {
		outbound.Mux = json_util.RawMessage(s.mux)
	}
	outbound.StreamSettings = streamSettings
	outbound.Settings = OutboundSettings{
		Vnext: vnextData,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genServer(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client) json_util.RawMessage {
	outbound := Outbound{}

	serverData := make([]ServerSetting, 1)
	serverData[0] = ServerSetting{
		Address:  inbound.Listen,
		Port:     inbound.Port,
		Level:    8,
		Password: client.Password,
	}

	if inbound.Protocol == model.Shadowsocks {
		var inboundSettings map[string]interface{}
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		method, _ := inboundSettings["method"].(string)
		serverData[0].Method = method

		// server password in multi-user 2022 protocols
		if strings.HasPrefix(method, "2022") {
			if serverPassword, ok := inboundSettings["password"].(string); ok {
				serverData[0].Password = fmt.Sprintf("%s:%s", serverPassword, client.Password)
			}
		}
	}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if s.mux != "" {
		outbound.Mux = json_util.RawMessage(s.mux)
	}
	outbound.StreamSettings = streamSettings
	outbound.Settings = OutboundSettings{
		Servers: serverData,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

type Outbound struct {
	Protocol       string                 `json:"protocol"`
	Tag            string                 `json:"tag"`
	StreamSettings json_util.RawMessage   `json:"streamSettings"`
	Mux            json_util.RawMessage   `json:"mux,omitempty"`
	ProxySettings  map[string]interface{} `json:"proxySettings,omitempty"`
	Settings       OutboundSettings       `json:"settings,omitempty"`
}

type OutboundSettings struct {
	Vnext   []VnextSetting  `json:"vnext,omitempty"`
	Servers []ServerSetting `json:"servers,omitempty"`
}

type VnextSetting struct {
	Address string      `json:"address"`
	Port    int         `json:"port"`
	Users   []UserVnext `json:"users"`
}

type UserVnext struct {
	Encryption string `json:"encryption,omitempty"`
	Flow       string `json:"flow,omitempty"`
	ID         string `json:"id"`
	Security   string `json:"security,omitempty"`
	Level      int    `json:"level"`
}

type ServerSetting struct {
	Password string `json:"password"`
	Level    int    `json:"level"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Flow     string `json:"flow,omitempty"`
	Method   string `json:"method,omitempty"`
}
