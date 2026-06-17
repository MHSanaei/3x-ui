package sub

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

//go:embed default.json
var defaultJson string

// SubJsonService handles JSON subscription configuration generation and management.
type SubJsonService struct {
	configJson       map[string]any
	defaultOutbounds []json_util.RawMessage
	finalMask        string
	mux              string

	SubService *SubService
}

// NewSubJsonService creates a new JSON subscription service with the given configuration.
func NewSubJsonService(mux string, rules string, finalMask string, subService *SubService) *SubJsonService {
	var configJson map[string]any
	var defaultOutbounds []json_util.RawMessage
	json.Unmarshal([]byte(defaultJson), &configJson)
	if outboundSlices, ok := configJson["outbounds"].([]any); ok {
		for _, defaultOutbound := range outboundSlices {
			jsonBytes, _ := json.Marshal(defaultOutbound)
			defaultOutbounds = append(defaultOutbounds, jsonBytes)
		}
	}

	if rules != "" {
		var newRules []any
		routing, _ := configJson["routing"].(map[string]any)
		defaultRules, _ := routing["rules"].([]any)
		json.Unmarshal([]byte(rules), &newRules)
		defaultRules = append(newRules, defaultRules...)
		routing["rules"] = defaultRules
		configJson["routing"] = routing
	}

	return &SubJsonService{
		configJson:       configJson,
		defaultOutbounds: defaultOutbounds,
		finalMask:        finalMask,
		mux:              mux,
		SubService:       subService,
	}
}

// GetJson generates a JSON subscription configuration for the given subscription ID and host.
func (s *SubJsonService) GetJson(subId string, host string) (string, string, error) {
	subReq := s.SubService.ForRequest(host)
	subReq.subscriptionBody = true
	inbounds, err := subReq.getInboundsBySubId(subId)
	if err != nil {
		return "", "", err
	}
	externalLinks, err := subReq.getClientExternalLinksBySubId(subId)
	if err != nil {
		return "", "", err
	}
	if len(inbounds) == 0 && len(externalLinks) == 0 {
		return "", "", nil
	}

	var header string
	var configArray []json_util.RawMessage

	seenEmails := make(map[string]struct{})
	// Prepare Inbounds
	for _, inbound := range inbounds {
		clients := subReq.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		subReq.projectThroughFallbackMaster(inbound)
		if hostEps := subReq.hostEndpoints(inbound, "json"); len(hostEps) > 0 {
			injectExternalProxy(inbound, hostEps)
		}

		for _, client := range clients {
			seenEmails[client.Email] = struct{}{}
			configArray = append(configArray, s.getConfig(subReq, inbound, client, host)...)
		}
	}
	for _, ext := range externalLinks {
		for _, el := range expandEntry(ext) {
			outbound := parsedExternalOutbound(el.Link)
			if outbound == nil {
				continue
			}
			seenEmails[ext.Email] = struct{}{}
			remark := el.Name
			if remark == "" {
				remark = ext.Email
			}
			newOutbounds := []json_util.RawMessage{outbound}
			newOutbounds = append(newOutbounds, s.defaultOutbounds...)
			newConfigJson := make(map[string]any)
			maps.Copy(newConfigJson, s.configJson)
			newConfigJson["outbounds"] = newOutbounds
			newConfigJson["remarks"] = remark
			newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
			configArray = append(configArray, newConfig)
		}
	}

	if len(configArray) == 0 {
		return "", "", nil
	}

	emails := make([]string, 0, len(seenEmails))
	for e := range seenEmails {
		emails = append(emails, e)
	}
	traffic, _ := subReq.AggregateTrafficByEmails(emails)

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

func (s *SubJsonService) getConfig(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
	var newJsonArray []json_util.RawMessage
	stream := s.streamData(inbound.StreamSettings)

	// When externalProxy is empty the JSON config falls back to a
	// synthetic one whose `dest` is the host the client connects to.
	// For node-managed inbounds we want the node's address — request
	// host won't reach the right xray. resolveInboundAddress already
	// implements the node→subscriber-host fallback chain.
	defaultDest := subReq.resolveInboundAddress(inbound)
	if defaultDest == "" {
		defaultDest = host
	}

	externalProxies, ok := stream["externalProxy"].([]any)
	hasExternalProxy := ok && len(externalProxies) > 0
	if !hasExternalProxy {
		externalProxies = []any{
			map[string]any{
				"forceTls": "same",
				"dest":     defaultDest,
				"port":     float64(inbound.Port),
				"remark":   "",
			},
		}
	}

	delete(stream, "externalProxy")

	for _, ep := range externalProxies {
		extPrxy := ep.(map[string]any)
		// Expand the host's {{VAR}} remark template for this client (no-op for
		// the synthetic/legacy entry) before it's used as the config remark.
		subReq.renderHostRemark(inbound, client, extPrxy)
		inbound.Listen = extPrxy["dest"].(string)
		inbound.Port = int(extPrxy["port"].(float64))
		newStream := cloneStreamForExternalProxy(stream)
		switch extPrxy["forceTls"].(string) {
		case "tls":
			if newStream["security"] != "tls" {
				newStream["security"] = "tls"
				newStream["tlsSettings"] = map[string]any{}
			}
		case "none":
			if newStream["security"] != "none" {
				newStream["security"] = "none"
				delete(newStream, "tlsSettings")
			}
		}
		security, _ := newStream["security"].(string)
		if hasExternalProxy {
			applyExternalProxyTLSToStream(extPrxy, newStream, security)
		}
		applyHostStreamOverrides(extPrxy, newStream)
		streamSettings, _ := json.MarshalIndent(newStream, "", "  ")
		hostMux := hostMuxOverride(extPrxy)

		var newOutbounds []json_util.RawMessage

		switch inbound.Protocol {
		case "vmess":
			newOutbounds = append(newOutbounds, s.genVnext(inbound, streamSettings, client, hostMux))
		case "vless":
			newOutbounds = append(newOutbounds, s.genVless(inbound, streamSettings, client, hostMux))
		case "trojan", "shadowsocks":
			newOutbounds = append(newOutbounds, s.genServer(inbound, streamSettings, client, hostMux))
		case "hysteria":
			newOutbounds = append(newOutbounds, s.genHy(inbound, newStream, client))
		}

		newOutbounds = append(newOutbounds, s.defaultOutbounds...)
		newConfigJson := make(map[string]any)
		maps.Copy(newConfigJson, s.configJson)

		newConfigJson["outbounds"] = newOutbounds
		newConfigJson["remarks"] = subReq.endpointRemark(inbound, client.Email, extPrxy)

		newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
		newJsonArray = append(newJsonArray, newConfig)
	}

	return newJsonArray
}

func (s *SubJsonService) streamData(stream string) map[string]any {
	var streamSettings map[string]any
	json.Unmarshal([]byte(stream), &streamSettings)
	security, _ := streamSettings["security"].(string)
	switch security {
	case "tls":
		streamSettings["tlsSettings"] = s.tlsData(streamSettings["tlsSettings"].(map[string]any))
	case "reality":
		streamSettings["realitySettings"] = s.realityData(streamSettings["realitySettings"].(map[string]any))
	}
	delete(streamSettings, "sockopt")

	if s.finalMask != "" {
		s.applyGlobalFinalMask(streamSettings)
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
	case "xhttp":
		streamSettings["xhttpSettings"] = s.removeAcceptProxy(streamSettings["xhttpSettings"])
		if xhttp, ok := streamSettings["xhttpSettings"].(map[string]any); ok {
			delete(xhttp, "noSSEHeader")
			delete(xhttp, "scMaxBufferedPosts")
			delete(xhttp, "scStreamUpServerSecs")
			delete(xhttp, "serverMaxHeaderBytes")
			// Values matching xray-core's own defaults stay off the wire:
			// old panels seeded them into every stored config and the
			// literal scMinPostsIntervalMs=30 is a DPI fingerprint (#5141).
			if v, _ := xhttp["scMaxEachPostBytes"].(string); v == "" || v == "1000000" {
				delete(xhttp, "scMaxEachPostBytes")
			}
			if v, _ := xhttp["scMinPostsIntervalMs"].(string); v == "" || v == "30" {
				delete(xhttp, "scMinPostsIntervalMs")
			}
		}
	}
	return streamSettings
}

func (s *SubJsonService) applyGlobalFinalMask(streamSettings map[string]any) {
	var fm map[string]any
	if err := json.Unmarshal([]byte(s.finalMask), &fm); err != nil || len(fm) == 0 {
		return
	}
	merged := mergeFinalMask(streamSettings["finalmask"], fm)
	if len(merged) > 0 {
		streamSettings["finalmask"] = merged
	}
}

func (s *SubJsonService) removeAcceptProxy(setting any) map[string]any {
	netSettings, ok := setting.(map[string]any)
	if ok {
		delete(netSettings, "acceptProxyProtocol")
	}
	return netSettings
}

func (s *SubJsonService) tlsData(tData map[string]any) map[string]any {
	tlsData := make(map[string]any, 1)
	tlsClientSettings, _ := tData["settings"].(map[string]any)

	tlsData["serverName"] = tData["serverName"]
	tlsData["alpn"] = tData["alpn"]
	if fingerprint, ok := tlsClientSettings["fingerprint"].(string); ok {
		tlsData["fingerprint"] = fingerprint
	}
	if ech, ok := tlsClientSettings["echConfigList"].(string); ok && ech != "" {
		tlsData["echConfigList"] = ech
	}
	if pins, ok := tlsClientSettings["pinnedPeerCertSha256"].([]any); ok && len(pins) > 0 {
		tlsData["pinnedPeerCertSha256"] = pins
	}
	return tlsData
}

func (s *SubJsonService) realityData(rData map[string]any) map[string]any {
	rltyData := make(map[string]any, 1)
	rltyClientSettings, _ := rData["settings"].(map[string]any)

	rltyData["show"] = false
	rltyData["publicKey"] = rltyClientSettings["publicKey"]
	rltyData["fingerprint"] = rltyClientSettings["fingerprint"]
	rltyData["mldsa65Verify"] = rltyClientSettings["mldsa65Verify"]

	// Set random data
	rltyData["spiderX"] = "/" + random.Seq(15)
	shortIds, ok := rData["shortIds"].([]any)
	if ok && len(shortIds) > 0 {
		rltyData["shortId"] = shortIds[random.Num(len(shortIds))].(string)
	} else {
		rltyData["shortId"] = ""
	}
	serverNames, ok := rData["serverNames"].([]any)
	if ok && len(serverNames) > 0 {
		rltyData["serverName"] = serverNames[random.Num(len(serverNames))].(string)
	} else {
		rltyData["serverName"] = ""
	}

	return rltyData
}

// jsonMux picks the per-host mux override when present, else the global mux.
func jsonMux(global, override string) string {
	if override != "" {
		return override
	}
	return global
}

func (s *SubJsonService) genVnext(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, muxOverride string) json_util.RawMessage {
	outbound := Outbound{}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if mux := jsonMux(s.mux, muxOverride); mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}
	outbound.StreamSettings = streamSettings

	security := client.Security
	if security == "" {
		security = "auto"
	}
	outbound.Settings = map[string]any{
		"address":  inbound.Listen,
		"port":     inbound.Port,
		"id":       client.ID,
		"security": security,
		"level":    8,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genVless(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, muxOverride string) json_util.RawMessage {
	outbound := Outbound{}
	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if mux := jsonMux(s.mux, muxOverride); mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}
	outbound.StreamSettings = streamSettings

	// Add encryption for VLESS outbound from inbound settings
	var inboundSettings map[string]any
	json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
	encryption, _ := inboundSettings["encryption"].(string)

	settings := map[string]any{
		"address":    inbound.Listen,
		"port":       inbound.Port,
		"id":         client.ID,
		"encryption": encryption,
		"level":      8,
	}
	if client.Flow != "" {
		settings["flow"] = client.Flow
	}
	outbound.Settings = settings
	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genServer(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, muxOverride string) json_util.RawMessage {
	outbound := Outbound{}

	serverData := make([]ServerSetting, 1)
	serverData[0] = ServerSetting{
		Address:  inbound.Listen,
		Port:     inbound.Port,
		Level:    8,
		Password: client.Password,
	}

	if inbound.Protocol == model.Shadowsocks {
		var inboundSettings map[string]any
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
	if mux := jsonMux(s.mux, muxOverride); mux != "" {
		outbound.Mux = json_util.RawMessage(mux)
	}
	outbound.StreamSettings = streamSettings

	// Wrap the endpoint in a "servers" array (the standard Xray schema for
	// Shadowsocks/Trojan outbounds). The flat top-level form only parses on very
	// recent xray-core; older bundled cores (e.g. in v2rayN) reject it, so SS
	// links fail to connect. See genVnext/genVless for the VMess/VLESS shape.
	server := map[string]any{
		"address":  serverData[0].Address,
		"port":     serverData[0].Port,
		"password": serverData[0].Password,
		"level":    8,
	}
	if inbound.Protocol == model.Shadowsocks {
		server["method"] = serverData[0].Method
	}
	outbound.Settings = map[string]any{
		"servers": []any{server},
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genHy(inbound *model.Inbound, newStream map[string]any, client model.Client) json_util.RawMessage {
	outbound := Outbound{}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"

	if s.mux != "" {
		outbound.Mux = json_util.RawMessage(s.mux)
	}

	var settings, stream map[string]any
	json.Unmarshal([]byte(inbound.Settings), &settings)
	version, _ := settings["version"].(float64)
	outbound.Settings = map[string]any{
		"version": int(version),
		"address": inbound.Listen,
		"port":    inbound.Port,
	}

	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	hyStream := stream["hysteriaSettings"].(map[string]any)
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
	newStream["hysteriaSettings"] = outHyStream

	if finalmask, ok := hyStream["finalmask"].(map[string]any); ok {
		newStream["finalmask"] = mergeFinalMask(newStream["finalmask"], finalmask)
	}

	newStream["network"] = "hysteria"
	newStream["security"] = "tls"

	outbound.StreamSettings, _ = json.MarshalIndent(newStream, "", "  ")

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func mergeFinalMask(base any, extra map[string]any) map[string]any {
	merged := map[string]any{}
	if baseMap, ok := base.(map[string]any); ok {
		for key, value := range baseMap {
			switch key {
			case "tcp", "udp":
				if masks, ok := value.([]any); ok {
					merged[key] = append([]any(nil), masks...)
				}
			default:
				merged[key] = value
			}
		}
	}

	for key, value := range extra {
		switch key {
		case "tcp", "udp":
			baseMasks, _ := merged[key].([]any)
			extraMasks, _ := value.([]any)
			if len(extraMasks) > 0 {
				merged[key] = append(baseMasks, extraMasks...)
			}
		case "quicParams":
			if _, exists := merged[key]; !exists {
				merged[key] = value
			}
		default:
			merged[key] = value
		}
	}

	return merged
}

type Outbound struct {
	Protocol       string               `json:"protocol"`
	Tag            string               `json:"tag"`
	StreamSettings json_util.RawMessage `json:"streamSettings"`
	Mux            json_util.RawMessage `json:"mux,omitempty"`
	Settings       map[string]any       `json:"settings,omitempty"`
}

type ServerSetting struct {
	Password string `json:"password"`
	Level    int    `json:"level"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Flow     string `json:"flow,omitempty"`
	Method   string `json:"method,omitempty"`
}
