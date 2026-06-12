package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func (s *SubService) getAttachedOutboundsBySubId(subId string) ([]map[string]any, []model.ClientRecord, error) {
	db := database.GetDB()
	var recs []model.ClientRecord
	if err := db.Where("sub_id = ?", subId).Find(&recs).Error; err != nil {
		return nil, nil, err
	}
	if len(recs) == 0 {
		return nil, nil, nil
	}
	clientIds := make([]int, 0, len(recs))
	byId := make(map[int]model.ClientRecord, len(recs))
	for _, rec := range recs {
		clientIds = append(clientIds, rec.Id)
		byId[rec.Id] = rec
	}
	var links []model.ClientOutbound
	if err := db.Where("client_id IN ?", clientIds).Order("outbound_tag ASC").Find(&links).Error; err != nil {
		return nil, nil, err
	}
	if len(links) == 0 {
		return nil, nil, nil
	}

	wanted := make(map[string]struct{}, len(links))
	for _, l := range links {
		wanted[l.OutboundTag] = struct{}{}
	}
	all, err := (&service.ClientService{}).OutboundOptionsWithRaw(&service.XraySettingService{}, service.NewOutboundSubscriptionService())
	if err != nil {
		return nil, nil, err
	}
	byTag := make(map[string]map[string]any, len(all))
	for _, raw := range all {
		tag, _ := raw["tag"].(string)
		if _, ok := wanted[tag]; ok {
			byTag[tag] = raw
		}
	}

	var out []map[string]any
	var owners []model.ClientRecord
	for _, l := range links {
		raw, ok := byTag[l.OutboundTag]
		if !ok {
			continue
		}
		rec := byId[l.ClientId]
		out = append(out, cloneOutboundMap(raw))
		owners = append(owners, rec)
	}
	return out, owners, nil
}

func cloneOutboundMap(in map[string]any) map[string]any {
	b, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func personalizeOutbound(raw map[string]any, rec model.ClientRecord) map[string]any {
	ob := cloneOutboundMap(raw)
	delete(ob, "_source")
	delete(ob, "clientExternalConfig")
	ob["tag"] = "proxy"
	return ob
}

func (s *SubService) outboundShareLink(raw map[string]any, rec model.ClientRecord) string {
	ob := personalizeOutbound(raw, rec)
	protocol, _ := ob["protocol"].(string)
	settings, _ := ob["settings"].(map[string]any)
	stream, _ := ob["streamSettings"].(map[string]any)
	remark := rec.Email
	if tag, _ := raw["tag"].(string); tag != "" {
		remark = fmt.Sprintf("%s-%s", tag, rec.Email)
	}
	switch protocol {
	case "vmess":
		return outboundVmessLink(settings, stream, remark)
	case "vless":
		return outboundVlessLink(settings, stream, remark)
	case "trojan":
		return outboundTrojanLink(settings, stream, remark)
	case "shadowsocks":
		return outboundShadowsocksLink(settings, stream, remark)
	}
	return ""
}

func outboundVmessLink(settings, stream map[string]any, remark string) string {
	vnext, _ := settings["vnext"].([]any)
	if len(vnext) == 0 {
		return ""
	}
	vn, _ := vnext[0].(map[string]any)
	users, _ := vn["users"].([]any)
	if vn == nil || len(users) == 0 {
		return ""
	}
	user, _ := users[0].(map[string]any)
	network := firstString(stream["network"], "tcp")
	obj := map[string]any{
		"v":    "2",
		"ps":   remark,
		"add":  vn["address"],
		"port": vn["port"],
		"id":   user["id"],
		"scy":  firstString(user["security"], "auto"),
		"type": "none",
	}
	applyVmessNetworkParams(stream, network, obj)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskObj(finalmask, obj)
	}
	security := firstString(stream["security"], "none")
	obj["tls"] = security
	if security == "tls" {
		applyOutboundTLSObj(stream, obj)
	}
	b, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

func outboundVlessLink(settings, stream map[string]any, remark string) string {
	addr := fmt.Sprint(settings["address"])
	port := intFromAny(settings["port"])
	id := fmt.Sprint(settings["id"])
	streamNetwork := firstString(stream["network"], "tcp")
	params := map[string]string{"type": streamNetwork}
	applyShareNetworkParams(stream, streamNetwork, params)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, params)
	}
	applyOutboundSecurityParams(stream, params)
	if flow, _ := settings["flow"].(string); flow != "" {
		params["flow"] = flow
	}
	if enc, _ := settings["encryption"].(string); enc != "" {
		params["encryption"] = enc
	}
	return buildLinkWithParams(fmt.Sprintf("vless://%s@%s:%d", id, addr, port), params, remark)
}

func outboundTrojanLink(settings, stream map[string]any, remark string) string {
	servers, _ := settings["servers"].([]any)
	if len(servers) == 0 {
		return ""
	}
	server, _ := servers[0].(map[string]any)
	streamNetwork := firstString(stream["network"], "tcp")
	paramMap := map[string]string{"type": streamNetwork}
	applyShareNetworkParams(stream, streamNetwork, paramMap)
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, paramMap)
	}
	applyOutboundSecurityParams(stream, paramMap)
	return buildLinkWithParams(fmt.Sprintf("trojan://%s@%s:%d",
		encodeUserinfo(fmt.Sprint(server["password"])),
		fmt.Sprint(server["address"]),
		intFromAny(server["port"]),
	), paramMap, remark)
}

func outboundShadowsocksLink(settings, stream map[string]any, remark string) string {
	server := settings
	if servers, _ := settings["servers"].([]any); len(servers) > 0 {
		if first, _ := servers[0].(map[string]any); first != nil {
			server = first
		}
	}
	method := fmt.Sprint(server["method"])
	password := fmt.Sprint(server["password"])
	addr := fmt.Sprint(server["address"])
	port := intFromAny(server["port"])
	streamNetwork := firstString(stream["network"], "tcp")
	paramMap := map[string]string{}
	if streamNetwork != "" && streamNetwork != "tcp" {
		paramMap["type"] = streamNetwork
		applyShareNetworkParams(stream, streamNetwork, paramMap)
	}
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		applyFinalMaskParams(finalmask, paramMap)
	}
	if firstString(stream["security"], "none") == "tls" {
		applyOutboundTLSParams(stream, paramMap)
	}
	user := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	link := fmt.Sprintf("ss://%s@%s:%d", user, addr, port)
	return buildLinkWithParams(link, paramMap, remark)
}

func applyOutboundSecurityParams(stream map[string]any, params map[string]string) {
	switch firstString(stream["security"], "none") {
	case "tls":
		applyOutboundTLSParams(stream, params)
	case "reality":
		applyOutboundRealityParams(stream, params)
	default:
		params["security"] = "none"
	}
}

func applyOutboundTLSParams(stream map[string]any, params map[string]string) {
	params["security"] = "tls"
	tlsSetting, _ := stream["tlsSettings"].(map[string]any)
	if tlsSetting == nil {
		return
	}
	if alpn, ok := shareAlpn(tlsSetting["alpn"]); ok {
		params["alpn"] = alpn
	}
	if sni, _ := tlsSetting["serverName"].(string); sni != "" {
		params["sni"] = sni
	}
	inner, _ := tlsSetting["settings"].(map[string]any)
	if fp := firstNonEmptyString(tlsSetting["fingerprint"], inner["fingerprint"]); fp != "" {
		params["fp"] = fp
	}
	if ech := firstNonEmptyString(tlsSetting["echConfigList"], inner["echConfigList"]); ech != "" {
		params["ech"] = ech
	}
	if pcs := sharePins(tlsSetting["pinnedPeerCertSha256"], inner["pinnedPeerCertSha256"]); pcs != "" {
		params["pcs"] = pcs
	}
}

func applyOutboundTLSObj(stream map[string]any, obj map[string]any) {
	params := map[string]string{}
	applyOutboundTLSParams(stream, params)
	for k, v := range params {
		if k == "security" {
			continue
		}
		obj[k] = v
	}
}

func applyOutboundRealityParams(stream map[string]any, params map[string]string) {
	params["security"] = "reality"
	realitySetting, _ := stream["realitySettings"].(map[string]any)
	if realitySetting == nil {
		return
	}
	inner, _ := realitySetting["settings"].(map[string]any)
	if sni := firstNonEmptyString(realitySetting["serverName"], inner["serverName"], firstStringFromArray(realitySetting["serverNames"])); sni != "" {
		params["sni"] = sni
	}
	if pbk := firstNonEmptyString(realitySetting["publicKey"], inner["publicKey"]); pbk != "" {
		params["pbk"] = pbk
	}
	if sid := firstNonEmptyString(realitySetting["shortId"], firstStringFromArray(realitySetting["shortIds"])); sid != "" {
		params["sid"] = sid
	}
	if fp := firstNonEmptyString(realitySetting["fingerprint"], inner["fingerprint"]); fp != "" {
		params["fp"] = fp
	}
	if pqv := firstNonEmptyString(realitySetting["mldsa65Verify"], inner["mldsa65Verify"]); pqv != "" {
		params["pqv"] = pqv
	}
	if spx := firstNonEmptyString(realitySetting["spiderX"], inner["spiderX"]); spx != "" {
		params["spx"] = spx
	}
}

func shareAlpn(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, v != ""
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				items = append(items, s)
			}
		}
		return strings.Join(items, ","), len(items) > 0
	default:
		return "", false
	}
}

func sharePins(values ...any) string {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if v != "" {
				return v
			}
		case []any:
			items := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					items = append(items, s)
				}
			}
			if len(items) > 0 {
				return strings.Join(items, ",")
			}
		}
	}
	return ""
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if s, ok := value.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func firstStringFromArray(value any) string {
	if arr, ok := value.([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func firstString(v any, fallback string) string {
	if s, _ := v.(string); strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}

func personalizedOutboundConfig(raw map[string]any, rec model.ClientRecord) json_util.RawMessage {
	ob := personalizeOutbound(raw, rec)
	b, _ := json.MarshalIndent(ob, "", "  ")
	return b
}
