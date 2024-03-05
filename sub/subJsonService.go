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
	fragmanet string

	inboundService service.InboundService
	SubService
}

func NewSubJsonService(fragment string) *SubJsonService {
	return &SubJsonService{
		fragmanet: fragment,
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
	var configJson map[string]interface{}
	var defaultOutbounds []json_util.RawMessage

	json.Unmarshal([]byte(defaultJson), &configJson)
	if outboundSlices, ok := configJson["outbounds"].([]interface{}); ok {
		for _, defaultOutbound := range outboundSlices {
			jsonBytes, _ := json.Marshal(defaultOutbound)
			defaultOutbounds = append(defaultOutbounds, jsonBytes)
		}
	}

	outbounds := []json_util.RawMessage{}
	startIndex := 0
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
			listen, port, streamSettings, err := s.getFallbackMaster(inbound.Listen, inbound.StreamSettings)
			if err == nil {
				inbound.Listen = listen
				inbound.Port = port
				inbound.StreamSettings = streamSettings
			}
		}

		var subClients []model.Client
		for _, client := range clients {
			if client.Enable && client.SubID == subId {
				subClients = append(subClients, client)
				clientTraffics = append(clientTraffics, s.SubService.getClientTraffics(inbound.ClientStats, client.Email))
			}
		}

		outbound := s.getOutbound(inbound, subClients, host, startIndex)
		if outbound != nil {
			outbounds = append(outbounds, outbound...)
			startIndex += len(outbound)
		}
	}

	if len(outbounds) == 0 {
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

	if s.fragmanet != "" {
		outbounds = append(outbounds, json_util.RawMessage(s.fragmanet))
	}

	// Combile outbounds
	outbounds = append(outbounds, defaultOutbounds...)
	configJson["outbounds"] = outbounds
	finalJson, _ := json.MarshalIndent(configJson, "", "  ")

	header = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(finalJson), header, nil
}

func (s *SubJsonService) getOutbound(inbound *model.Inbound, clients []model.Client, host string, startIndex int) []json_util.RawMessage {
	var newOutbounds []json_util.RawMessage
	stream := s.streamData(inbound.StreamSettings)

	externalProxies, ok := stream["externalProxy"].([]interface{})
	if !ok || len(externalProxies) == 0 {
		externalProxies = []interface{}{
			map[string]interface{}{
				"forceTls": "same",
				"dest":     host,
				"port":     float64(inbound.Port),
			},
		}
	}

	delete(stream, "externalProxy")

	config_index := startIndex
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
		inbound.StreamSettings = string(streamSettings)

		for _, client := range clients {
			inbound.Tag = fmt.Sprintf("proxy_%d", config_index)
			switch inbound.Protocol {
			case "vmess", "vless":
				newOutbounds = append(newOutbounds, s.genVnext(inbound, client))
			case "trojan", "shadowsocks":
				newOutbounds = append(newOutbounds, s.genServer(inbound, client))
			}
			config_index += 1
		}
	}

	return newOutbounds
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

	if s.fragmanet != "" {
		streamSettings["sockopt"] = json_util.RawMessage(`{"dialerProxy": "fragment", "tcpKeepAliveIdle": 100, "tcpNoDelay": true}`)
	}

	// remove proxy protocol
	network, _ := streamSettings["network"].(string)
	switch network {
	case "tcp":
		streamSettings["tcpSettings"] = s.removeAcceptProxy(streamSettings["tcpSettings"])
	case "ws":
		streamSettings["wsSettings"] = s.removeAcceptProxy(streamSettings["wsSettings"])
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
	tlsClientSettings := tData["settings"].(map[string]interface{})

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
	rltyClientSettings := rData["settings"].(map[string]interface{})

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

func (s *SubJsonService) genVnext(inbound *model.Inbound, client model.Client) json_util.RawMessage {
	outbound := Outbound{}
	usersData := make([]UserVnext, 1)

	usersData[0].ID = client.ID
	usersData[0].Level = 8
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
	outbound.Tag = inbound.Tag
	outbound.StreamSettings = json_util.RawMessage(inbound.StreamSettings)
	outbound.Settings = OutboundSettings{
		Vnext: vnextData,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genServer(inbound *model.Inbound, client model.Client) json_util.RawMessage {
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
	outbound.Tag = inbound.Tag
	outbound.StreamSettings = json_util.RawMessage(inbound.StreamSettings)
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
	Mux            map[string]interface{} `json:"mux,omitempty"`
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
