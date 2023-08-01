package sub

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/service"
	"x-ui/xray"

	"github.com/goccy/go-json"
)

type SubService struct {
	address        string
	inboundService service.InboundService
	settingServics service.SettingService
}

func (s *SubService) GetSubs(subId string, host string) ([]string, []string, error) {
	s.address = host
	var result []string
	var headers []string
	var traffic xray.ClientTraffic
	var clientTraffics []xray.ClientTraffic
	inbounds, err := s.getInboundsBySubId(subId)
	if err != nil {
		return nil, nil, err
	}
	for _, inbound := range inbounds {
		clients, err := s.inboundService.GetClients(inbound)
		if err != nil {
			logger.Error("SubService - GetSub: Unable to get clients from inbound")
		}
		if clients == nil {
			continue
		}
		if len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
			fallbackMaster, err := s.getFallbackMaster(inbound.Listen)
			if err == nil {
				inbound.Listen = fallbackMaster.Listen
				inbound.Port = fallbackMaster.Port
				var stream map[string]interface{}
				json.Unmarshal([]byte(inbound.StreamSettings), &stream)
				var masterStream map[string]interface{}
				json.Unmarshal([]byte(fallbackMaster.StreamSettings), &masterStream)
				stream["security"] = masterStream["security"]
				stream["tlsSettings"] = masterStream["tlsSettings"]
				modifiedStream, _ := json.MarshalIndent(stream, "", "  ")
				inbound.StreamSettings = string(modifiedStream)
			}
		}
		for _, client := range clients {
			if client.Enable && client.SubID == subId {
				link := s.getLink(inbound, client.Email, client.ExpiryTime)
				result = append(result, link)
				clientTraffics = append(clientTraffics, s.getClientTraffics(inbound.ClientStats, client.Email))
			}
		}
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
	headers = append(headers, fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000))
	updateInterval, _ := s.settingServics.GetSubUpdates()
	headers = append(headers, fmt.Sprintf("%d", updateInterval))
	headers = append(headers, subId)
	return result, headers, nil
}

func (s *SubService) getInboundsBySubId(subId string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where("settings like ? and enable = ?", fmt.Sprintf(`%%"subId": "%s"%%`, subId), true).Find(&inbounds).Error
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

func (s *SubService) getFallbackMaster(dest string) (*model.Inbound, error) {
	db := database.GetDB()
	var inbound *model.Inbound
	err := db.Model(model.Inbound{}).
		Where("JSON_TYPE(settings, '$.fallbacks') = 'array'").
		Where("EXISTS (SELECT * FROM json_each(settings, '$.fallbacks') WHERE json_extract(value, '$.dest') = ?)", dest).
		Find(&inbound).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (s *SubService) getLink(inbound *model.Inbound, email string, expiryTime int64) string {
	switch inbound.Protocol {
	case "vmess":
		return s.genVmessLink(inbound, email, expiryTime)
	case "vless":
		return s.genVlessLink(inbound, email, expiryTime)
	case "trojan":
		return s.genTrojanLink(inbound, email, expiryTime)
	case "shadowsocks":
		return s.genShadowsocksLink(inbound, email, expiryTime)
	}
	return ""
}

func (s *SubService) genVmessLink(inbound *model.Inbound, email string, expiryTime int64) string {
	if inbound.Protocol != model.VMess {
		return ""
	}

	remainedTraffic := s.getRemainedTraffic(email)
	expiryTimeString := getExpiryTime(expiryTime)
	remark := ""
	isTerminated := strings.Contains(expiryTimeString, "Terminated") || strings.Contains(remainedTraffic, "Terminated")

	if isTerminated {
		remark = fmt.Sprintf("%s: %s⛔️", email, "Terminated")
	} else {
		remark = fmt.Sprintf("%s: %s - %s", email, remainedTraffic, expiryTimeString)
	}

	obj := map[string]interface{}{
		"v":    "2",
		"ps":   remark,
		"add":  s.address,
		"port": inbound.Port,
		"type": "none",
	}
	var stream map[string]interface{}
	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	network, _ := stream["network"].(string)
	obj["net"] = network
	switch network {
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]interface{})
		header, _ := tcp["header"].(map[string]interface{})
		typeStr, _ := header["type"].(string)
		obj["type"] = typeStr
		if typeStr == "http" {
			request := header["request"].(map[string]interface{})
			requestPath, _ := request["path"].([]interface{})
			obj["path"] = requestPath[0].(string)
			headers, _ := request["headers"].(map[string]interface{})
			obj["host"] = searchHost(headers)
		}
	case "kcp":
		kcp, _ := stream["kcpSettings"].(map[string]interface{})
		header, _ := kcp["header"].(map[string]interface{})
		obj["type"], _ = header["type"].(string)
		obj["path"], _ = kcp["seed"].(string)
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]interface{})
		obj["path"] = ws["path"].(string)
		headers, _ := ws["headers"].(map[string]interface{})
		obj["host"] = searchHost(headers)
	case "http":
		obj["net"] = "h2"
		http, _ := stream["httpSettings"].(map[string]interface{})
		obj["path"], _ = http["path"].(string)
		obj["host"] = searchHost(http)
	case "quic":
		quic, _ := stream["quicSettings"].(map[string]interface{})
		header := quic["header"].(map[string]interface{})
		obj["type"], _ = header["type"].(string)
		obj["host"], _ = quic["security"].(string)
		obj["path"], _ = quic["key"].(string)
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]interface{})
		obj["path"] = grpc["serviceName"].(string)
		if grpc["multiMode"].(bool) {
			obj["type"] = "multi"
		}
	}

	security, _ := stream["security"].(string)
	var domains []interface{}
	obj["tls"] = security
	if security == "tls" {
		tlsSetting, _ := stream["tlsSettings"].(map[string]interface{})
		alpns, _ := tlsSetting["alpn"].([]interface{})
		if len(alpns) > 0 {
			var alpn []string
			for _, a := range alpns {
				alpn = append(alpn, a.(string))
			}
			obj["alpn"] = strings.Join(alpn, ",")
		}
		tlsSettings, _ := searchKey(tlsSetting, "settings")
		if tlsSetting != nil {
			if sniValue, ok := searchKey(tlsSettings, "serverName"); ok {
				obj["sni"], _ = sniValue.(string)
			}
			if fpValue, ok := searchKey(tlsSettings, "fingerprint"); ok {
				obj["fp"], _ = fpValue.(string)
			}
			if insecure, ok := searchKey(tlsSettings, "allowInsecure"); ok {
				obj["allowInsecure"], _ = insecure.(bool)
			}
			if domainSettings, ok := searchKey(tlsSettings, "domains"); ok {
				domains, _ = domainSettings.([]interface{})
			}
		}
		serverName, _ := tlsSetting["serverName"].(string)
		if serverName != "" {
			obj["add"] = serverName
		}
	}

	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := -1
	for i, client := range clients {
		if client.Email == email {
			clientIndex = i
			break
		}
	}
	obj["id"] = clients[clientIndex].ID

	if len(domains) > 0 {
		links := ""
		for index, d := range domains {
			domain := d.(map[string]interface{})
			obj["ps"] = remark + "-" + domain["remark"].(string)
			obj["add"] = domain["domain"].(string)
			if index > 0 {
				links += "\n"
			}
			jsonStr, _ := json.MarshalIndent(obj, "", "  ")
			links += "vmess://" + base64.StdEncoding.EncodeToString(jsonStr)
		}
		return links
	}

	jsonStr, _ := json.MarshalIndent(obj, "", "  ")
	return "vmess://" + base64.StdEncoding.EncodeToString(jsonStr)
}

func (s *SubService) genVlessLink(inbound *model.Inbound, email string, expiryTime int64) string {
	address := s.address
	if inbound.Protocol != model.VLESS {
		return ""
	}
	var stream map[string]interface{}
	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := -1
	for i, client := range clients {
		if client.Email == email {
			clientIndex = i
			break
		}
	}
	uuid := clients[clientIndex].ID
	port := inbound.Port
	streamNetwork := stream["network"].(string)
	params := make(map[string]string)
	params["type"] = streamNetwork

	switch streamNetwork {
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]interface{})
		header, _ := tcp["header"].(map[string]interface{})
		typeStr, _ := header["type"].(string)
		if typeStr == "http" {
			request := header["request"].(map[string]interface{})
			requestPath, _ := request["path"].([]interface{})
			params["path"] = requestPath[0].(string)
			headers, _ := request["headers"].(map[string]interface{})
			params["host"] = searchHost(headers)
			params["headerType"] = "http"
		}
	case "kcp":
		kcp, _ := stream["kcpSettings"].(map[string]interface{})
		header, _ := kcp["header"].(map[string]interface{})
		params["headerType"] = header["type"].(string)
		params["seed"] = kcp["seed"].(string)
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]interface{})
		params["path"] = ws["path"].(string)
		headers, _ := ws["headers"].(map[string]interface{})
		params["host"] = searchHost(headers)
	case "http":
		http, _ := stream["httpSettings"].(map[string]interface{})
		params["path"] = http["path"].(string)
		params["host"] = searchHost(http)
	case "quic":
		quic, _ := stream["quicSettings"].(map[string]interface{})
		params["quicSecurity"] = quic["security"].(string)
		params["key"] = quic["key"].(string)
		header := quic["header"].(map[string]interface{})
		params["headerType"] = header["type"].(string)
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]interface{})
		params["serviceName"] = grpc["serviceName"].(string)
		if grpc["multiMode"].(bool) {
			params["mode"] = "multi"
		}
	}

	security, _ := stream["security"].(string)
	var domains []interface{}
	if security == "tls" {
		params["security"] = "tls"
		tlsSetting, _ := stream["tlsSettings"].(map[string]interface{})
		alpns, _ := tlsSetting["alpn"].([]interface{})
		var alpn []string
		for _, a := range alpns {
			alpn = append(alpn, a.(string))
		}
		if len(alpn) > 0 {
			params["alpn"] = strings.Join(alpn, ",")
		}
		tlsSettings, _ := searchKey(tlsSetting, "settings")
		if tlsSetting != nil {
			if sniValue, ok := searchKey(tlsSettings, "serverName"); ok {
				params["sni"], _ = sniValue.(string)
			}
			if fpValue, ok := searchKey(tlsSettings, "fingerprint"); ok {
				params["fp"], _ = fpValue.(string)
			}
			if insecure, ok := searchKey(tlsSettings, "allowInsecure"); ok {
				if insecure.(bool) {
					params["allowInsecure"] = "1"
				}
			}
			if domainSettings, ok := searchKey(tlsSettings, "domains"); ok {
				domains, _ = domainSettings.([]interface{})
			}
		}

		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}

		serverName, _ := tlsSetting["serverName"].(string)
		if serverName != "" {
			address = serverName
		}
	}

	if security == "reality" {
		params["security"] = "reality"
		realitySetting, _ := stream["realitySettings"].(map[string]interface{})
		realitySettings, _ := searchKey(realitySetting, "settings")
		if realitySetting != nil {
			if sniValue, ok := searchKey(realitySetting, "serverNames"); ok {
				sNames, _ := sniValue.([]interface{})
				params["sni"], _ = sNames[0].(string)
			}
			if pbkValue, ok := searchKey(realitySettings, "publicKey"); ok {
				params["pbk"], _ = pbkValue.(string)
			}
			if sidValue, ok := searchKey(realitySetting, "shortIds"); ok {
				shortIds, _ := sidValue.([]interface{})
				params["sid"], _ = shortIds[0].(string)
			}
			if fpValue, ok := searchKey(realitySettings, "fingerprint"); ok {
				if fp, ok := fpValue.(string); ok && len(fp) > 0 {
					params["fp"] = fp
				}
			}
			if spxValue, ok := searchKey(realitySettings, "spiderX"); ok {
				if spx, ok := spxValue.(string); ok && len(spx) > 0 {
					params["spx"] = spx
				}
			}
			if serverName, ok := searchKey(realitySettings, "serverName"); ok {
				if sname, ok := serverName.(string); ok && len(sname) > 0 {
					address = sname
				}
			}
		}

		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}
	}

	if security == "xtls" {
		params["security"] = "xtls"
		xtlsSetting, _ := stream["xtlsSettings"].(map[string]interface{})
		alpns, _ := xtlsSetting["alpn"].([]interface{})
		var alpn []string
		for _, a := range alpns {
			alpn = append(alpn, a.(string))
		}
		if len(alpn) > 0 {
			params["alpn"] = strings.Join(alpn, ",")
		}

		xtlsSettings, _ := searchKey(xtlsSetting, "settings")
		if xtlsSetting != nil {
			if fpValue, ok := searchKey(xtlsSettings, "fingerprint"); ok {
				params["fp"], _ = fpValue.(string)
			}
			if insecure, ok := searchKey(xtlsSettings, "allowInsecure"); ok {
				if insecure.(bool) {
					params["allowInsecure"] = "1"
				}
			}
			if sniValue, ok := searchKey(xtlsSettings, "serverName"); ok {
				params["sni"], _ = sniValue.(string)
			}
		}

		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}

		serverName, _ := xtlsSetting["serverName"].(string)
		if serverName != "" {
			address = serverName
		}
	}

	if security != "tls" && security != "reality" && security != "xtls" {
		params["security"] = "none"
	}

	link := fmt.Sprintf("vless://%s@%s:%d", uuid, address, port)
	url, _ := url.Parse(link)
	q := url.Query()

	for k, v := range params {
		q.Add(k, v)
	}

	// Set the new query values on the URL
	url.RawQuery = q.Encode()

	remainedTraffic := s.getRemainedTraffic(email)
	expiryTimeString := getExpiryTime(expiryTime)
	remark := ""
	isTerminated := strings.Contains(expiryTimeString, "Terminated") || strings.Contains(remainedTraffic, "Terminated")

	if isTerminated {
		remark = fmt.Sprintf("%s: %s⛔️", email, "Terminated")
	} else {
		remark = fmt.Sprintf("%s: %s - %s", email, remainedTraffic, expiryTimeString)
	}

	if len(domains) > 0 {
		links := ""
		for index, d := range domains {
			domain := d.(map[string]interface{})
			url.Fragment = remark + "-" + domain["remark"].(string)
			url.Host = fmt.Sprintf("%s:%d", domain["domain"].(string), port)
			if index > 0 {
				links += "\n"
			}
			links += url.String()
		}
		return links
	}
	url.Fragment = remark
	return url.String()
}

func (s *SubService) genTrojanLink(inbound *model.Inbound, email string, expiryTime int64) string {
	address := s.address
	if inbound.Protocol != model.Trojan {
		return ""
	}
	var stream map[string]interface{}
	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	clients, _ := s.inboundService.GetClients(inbound)
	clientIndex := -1
	for i, client := range clients {
		if client.Email == email {
			clientIndex = i
			break
		}
	}
	password := clients[clientIndex].Password
	port := inbound.Port
	streamNetwork := stream["network"].(string)
	params := make(map[string]string)
	params["type"] = streamNetwork

	switch streamNetwork {
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]interface{})
		header, _ := tcp["header"].(map[string]interface{})
		typeStr, _ := header["type"].(string)
		if typeStr == "http" {
			request := header["request"].(map[string]interface{})
			requestPath, _ := request["path"].([]interface{})
			params["path"] = requestPath[0].(string)
			headers, _ := request["headers"].(map[string]interface{})
			params["host"] = searchHost(headers)
			params["headerType"] = "http"
		}
	case "kcp":
		kcp, _ := stream["kcpSettings"].(map[string]interface{})
		header, _ := kcp["header"].(map[string]interface{})
		params["headerType"] = header["type"].(string)
		params["seed"] = kcp["seed"].(string)
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]interface{})
		params["path"] = ws["path"].(string)
		headers, _ := ws["headers"].(map[string]interface{})
		params["host"] = searchHost(headers)
	case "http":
		http, _ := stream["httpSettings"].(map[string]interface{})
		params["path"] = http["path"].(string)
		params["host"] = searchHost(http)
	case "quic":
		quic, _ := stream["quicSettings"].(map[string]interface{})
		params["quicSecurity"] = quic["security"].(string)
		params["key"] = quic["key"].(string)
		header := quic["header"].(map[string]interface{})
		params["headerType"] = header["type"].(string)
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]interface{})
		params["serviceName"] = grpc["serviceName"].(string)
		if grpc["multiMode"].(bool) {
			params["mode"] = "multi"
		}
	}

	security, _ := stream["security"].(string)
	var domains []interface{}
	if security == "tls" {
		params["security"] = "tls"
		tlsSetting, _ := stream["tlsSettings"].(map[string]interface{})
		alpns, _ := tlsSetting["alpn"].([]interface{})
		var alpn []string
		for _, a := range alpns {
			alpn = append(alpn, a.(string))
		}
		if len(alpn) > 0 {
			params["alpn"] = strings.Join(alpn, ",")
		}
		tlsSettings, _ := searchKey(tlsSetting, "settings")
		if tlsSetting != nil {
			if sniValue, ok := searchKey(tlsSettings, "serverName"); ok {
				params["sni"], _ = sniValue.(string)
			}
			if fpValue, ok := searchKey(tlsSettings, "fingerprint"); ok {
				params["fp"], _ = fpValue.(string)
			}
			if insecure, ok := searchKey(tlsSettings, "allowInsecure"); ok {
				if insecure.(bool) {
					params["allowInsecure"] = "1"
				}
			}
			if domainSettings, ok := searchKey(tlsSettings, "domains"); ok {
				domains, _ = domainSettings.([]interface{})
			}
		}

		serverName, _ := tlsSetting["serverName"].(string)
		if serverName != "" {
			address = serverName
		}
	}

	if security == "reality" {
		params["security"] = "reality"
		realitySetting, _ := stream["realitySettings"].(map[string]interface{})
		realitySettings, _ := searchKey(realitySetting, "settings")
		if realitySetting != nil {
			if sniValue, ok := searchKey(realitySetting, "serverNames"); ok {
				sNames, _ := sniValue.([]interface{})
				params["sni"], _ = sNames[0].(string)
			}
			if pbkValue, ok := searchKey(realitySettings, "publicKey"); ok {
				params["pbk"], _ = pbkValue.(string)
			}
			if sidValue, ok := searchKey(realitySetting, "shortIds"); ok {
				shortIds, _ := sidValue.([]interface{})
				params["sid"], _ = shortIds[0].(string)
			}
			if fpValue, ok := searchKey(realitySettings, "fingerprint"); ok {
				if fp, ok := fpValue.(string); ok && len(fp) > 0 {
					params["fp"] = fp
				}
			}
			if spxValue, ok := searchKey(realitySettings, "spiderX"); ok {
				if spx, ok := spxValue.(string); ok && len(spx) > 0 {
					params["spx"] = spx
				}
			}
			if serverName, ok := searchKey(realitySettings, "serverName"); ok {
				if sname, ok := serverName.(string); ok && len(sname) > 0 {
					address = sname
				}
			}
		}

		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}
	}

	if security == "xtls" {
		params["security"] = "xtls"
		xtlsSetting, _ := stream["xtlsSettings"].(map[string]interface{})
		alpns, _ := xtlsSetting["alpn"].([]interface{})
		var alpn []string
		for _, a := range alpns {
			alpn = append(alpn, a.(string))
		}
		if len(alpn) > 0 {
			params["alpn"] = strings.Join(alpn, ",")
		}

		xtlsSettings, _ := searchKey(xtlsSetting, "settings")
		if xtlsSetting != nil {
			if fpValue, ok := searchKey(xtlsSettings, "fingerprint"); ok {
				params["fp"], _ = fpValue.(string)
			}
			if insecure, ok := searchKey(xtlsSettings, "allowInsecure"); ok {
				if insecure.(bool) {
					params["allowInsecure"] = "1"
				}
			}
			if sniValue, ok := searchKey(xtlsSettings, "serverName"); ok {
				params["sni"], _ = sniValue.(string)
			}
		}

		if streamNetwork == "tcp" && len(clients[clientIndex].Flow) > 0 {
			params["flow"] = clients[clientIndex].Flow
		}

		serverName, _ := xtlsSetting["serverName"].(string)
		if serverName != "" {
			address = serverName
		}
	}

	if security != "tls" && security != "reality" && security != "xtls" {
		params["security"] = "none"
	}

	link := fmt.Sprintf("trojan://%s@%s:%d", password, address, port)

	url, _ := url.Parse(link)
	q := url.Query()

	for k, v := range params {
		q.Add(k, v)
	}

	// Set the new query values on the URL
	url.RawQuery = q.Encode()

	remainedTraffic := s.getRemainedTraffic(email)
	expiryTimeString := getExpiryTime(expiryTime)
	remark := ""
	isTerminated := strings.Contains(expiryTimeString, "Terminated") || strings.Contains(remainedTraffic, "Terminated")

	if isTerminated {
		remark = fmt.Sprintf("%s: %s⛔️", email, "Terminated")
	} else {
		remark = fmt.Sprintf("%s: %s - %s", email, remainedTraffic, expiryTimeString)
	}

	if len(domains) > 0 {
		links := ""
		for index, d := range domains {
			domain := d.(map[string]interface{})
			url.Fragment = remark + "-" + domain["remark"].(string)
			url.Host = fmt.Sprintf("%s:%d", domain["domain"].(string), port)
			if index > 0 {
				links += "\n"
			}
			links += url.String()
		}
		return links
	}

	url.Fragment = remark
	return url.String()
}

func (s *SubService) genShadowsocksLink(inbound *model.Inbound, email string, expiryTime int64) string {
	address := s.address
	if inbound.Protocol != model.Shadowsocks {
		return ""
	}
	var stream map[string]interface{}
	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	clients, _ := s.inboundService.GetClients(inbound)

	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	inboundPassword := settings["password"].(string)
	method := settings["method"].(string)
	clientIndex := -1
	for i, client := range clients {
		if client.Email == email {
			clientIndex = i
			break
		}
	}
	streamNetwork := stream["network"].(string)
	params := make(map[string]string)
	params["type"] = streamNetwork

	switch streamNetwork {
	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]interface{})
		header, _ := tcp["header"].(map[string]interface{})
		typeStr, _ := header["type"].(string)
		if typeStr == "http" {
			request := header["request"].(map[string]interface{})
			requestPath, _ := request["path"].([]interface{})
			params["path"] = requestPath[0].(string)
			headers, _ := request["headers"].(map[string]interface{})
			params["host"] = searchHost(headers)
			params["headerType"] = "http"
		}
	case "kcp":
		kcp, _ := stream["kcpSettings"].(map[string]interface{})
		header, _ := kcp["header"].(map[string]interface{})
		params["headerType"] = header["type"].(string)
		params["seed"] = kcp["seed"].(string)
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]interface{})
		params["path"] = ws["path"].(string)
		headers, _ := ws["headers"].(map[string]interface{})
		params["host"] = searchHost(headers)
	case "http":
		http, _ := stream["httpSettings"].(map[string]interface{})
		params["path"] = http["path"].(string)
		params["host"] = searchHost(http)
	case "quic":
		quic, _ := stream["quicSettings"].(map[string]interface{})
		params["quicSecurity"] = quic["security"].(string)
		params["key"] = quic["key"].(string)
		header := quic["header"].(map[string]interface{})
		params["headerType"] = header["type"].(string)
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]interface{})
		params["serviceName"] = grpc["serviceName"].(string)
		if grpc["multiMode"].(bool) {
			params["mode"] = "multi"
		}
	}

	encPart := fmt.Sprintf("%s:%s", method, clients[clientIndex].Password)
	if method[0] == '2' {
		encPart = fmt.Sprintf("%s:%s:%s", method, inboundPassword, clients[clientIndex].Password)
	}
	link := fmt.Sprintf("ss://%s@%s:%d", base64.StdEncoding.EncodeToString([]byte(encPart)), address, inbound.Port)
	url, _ := url.Parse(link)
	q := url.Query()

	for k, v := range params {
		q.Add(k, v)
	}

	// Set the new query values on the URL
	url.RawQuery = q.Encode()

	remainedTraffic := s.getRemainedTraffic(email)
	expiryTimeString := getExpiryTime(expiryTime)
	remark := ""
	isTerminated := strings.Contains(expiryTimeString, "Terminated") || strings.Contains(remainedTraffic, "Terminated")

	if isTerminated {
		remark = fmt.Sprintf("%s: %s⛔️", clients[clientIndex].Email, "Terminated")
	} else {
		remark = fmt.Sprintf("%s: %s - %s", clients[clientIndex].Email, remainedTraffic, expiryTimeString)
	}
	
	url.Fragment = remark
	return url.String()
}

func searchKey(data interface{}, key string) (interface{}, bool) {
	switch val := data.(type) {
	case map[string]interface{}:
		for k, v := range val {
			if k == key {
				return v, true
			}
			if result, ok := searchKey(v, key); ok {
				return result, true
			}
		}
	case []interface{}:
		for _, v := range val {
			if result, ok := searchKey(v, key); ok {
				return result, true
			}
		}
	}
	return nil, false
}

func searchHost(headers interface{}) string {
	data, _ := headers.(map[string]interface{})
	for k, v := range data {
		if strings.EqualFold(k, "host") {
			switch v.(type) {
			case []interface{}:
				hosts, _ := v.([]interface{})
				if len(hosts) > 0 {
					return hosts[0].(string)
				} else {
					return ""
				}
			case interface{}:
				return v.(string)
			}
		}
	}

	return ""
}

func getExpiryTime(expiryTime int64) string {
	now := time.Now().Unix()
	expiryString := ""

	timeDifference := expiryTime/1000 - now
	isTerminated := timeDifference/3600 <= 0

	if expiryTime == 0 {
		expiryString = "♾ ⏳"
	} else if timeDifference > 172800 {
		expiryString = fmt.Sprintf("%d %s⏳", timeDifference/86400, "Days")
	} else if expiryTime < 0 {
		expiryString = fmt.Sprintf("%d %s⏳", expiryTime/-86400000, "Days")
	} else if isTerminated {
		expiryString = fmt.Sprintf("%s⛔️", "Terminated")
	} else {
		expiryString = fmt.Sprintf("%d %s⏳", timeDifference/3600, "Hours")
	}

	return expiryString
}

func (s *SubService) getRemainedTraffic(email string) string {
	traffic, err := s.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		logger.Warning(err)
	}

	remainedTraffic := ""
	isTerminated := traffic.Total-(traffic.Up+traffic.Down) < 0

	if traffic.Total == 0 {
		remainedTraffic = "♾ 📊"
	} else if isTerminated {
		remainedTraffic = fmt.Sprintf("%s⛔️", "Terminated")
	} else {
		remainedTraffic = fmt.Sprintf("%s%s", common.FormatTraffic(traffic.Total-(traffic.Up+traffic.Down)), "📊")
	}

	return remainedTraffic
}
