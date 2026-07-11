package sub

import (
	"fmt"
	"maps"
	"strings"

	"github.com/goccy/go-json"
	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

type SubClashService struct {
	enableRouting bool
	clashRules    string
	SubService    *SubService
}

func NewSubClashService(enableRouting bool, clashRules string, subService *SubService) *SubClashService {
	return &SubClashService{enableRouting: enableRouting, clashRules: clashRules, SubService: subService}
}

func (s *SubClashService) GetClash(subId string, host string) (string, string, error) {
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

	var proxies []map[string]any

	seenEmails := make(map[string]struct{})
	for _, inbound := range inbounds {
		clients := subReq.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		subReq.projectThroughFallbackMaster(inbound)
		if hostEps := subReq.hostEndpoints(inbound, "clash"); len(hostEps) > 0 {
			injectExternalProxy(inbound, hostEps)
		}
		for _, client := range clients {
			seenEmails[client.Email] = struct{}{}
			proxies = append(proxies, s.getProxies(subReq, inbound, client, host)...)
		}
	}
	for _, ext := range externalLinks {
		for _, el := range expandEntry(ext) {
			name := el.Name
			if name == "" {
				name = ext.Email
			}
			if proxy := s.clashProxyFromExternal(el.Link, name); proxy != nil {
				seenEmails[ext.Email] = struct{}{}
				proxies = append(proxies, proxy)
			}
		}
	}

	if len(proxies) == 0 {
		return "", "", nil
	}

	ensureUniqueProxyNames(proxies)

	emails := make([]string, 0, len(seenEmails))
	for e := range seenEmails {
		emails = append(emails, e)
	}
	traffic, _ := subReq.AggregateTrafficByEmails(emails)

	proxyNames := make([]string, 0, len(proxies)+1)
	for _, proxy := range proxies {
		if name, ok := proxy["name"].(string); ok && name != "" {
			proxyNames = append(proxyNames, name)
		}
	}
	proxyNames = append(proxyNames, "DIRECT")

	config := map[string]any{
		"proxies": proxies,
		"proxy-groups": []map[string]any{{
			"name":    "PROXY",
			"type":    "select",
			"proxies": proxyNames,
		}},
		"rules": []string{"MATCH,PROXY"},
	}

	if s.enableRouting {
		if err := mergeClashRulesYAML(config, s.clashRules); err != nil {
			return "", "", err
		}
	}

	finalYAML, err := yaml.Marshal(config)
	if err != nil {
		return "", "", err
	}

	header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(finalYAML), header, nil
}

// ensureUniqueProxyNames keeps every proxy "name" non-empty and unique:
// mihomo rejects the whole config on a duplicate name (the empty string
// genRemark returns for a remark-less inbound counts), vanishing the Clash
// profile on refresh. See issue #4641.
func ensureUniqueProxyNames(proxies []map[string]any) {
	seen := make(map[string]struct{}, len(proxies))
	for i, proxy := range proxies {
		base, _ := proxy["name"].(string)
		if base == "" {
			base = fallbackProxyName(proxy, i)
		}
		name := base
		for n := 2; ; n++ {
			if _, dup := seen[name]; !dup {
				break
			}
			name = fmt.Sprintf("%s-%d", base, n)
		}
		seen[name] = struct{}{}
		proxy["name"] = name
	}
}

func fallbackProxyName(proxy map[string]any, idx int) string {
	typ, _ := proxy["type"].(string)
	server, _ := proxy["server"].(string)
	if typ != "" && server != "" {
		return fmt.Sprintf("%s-%s-%v", typ, server, proxy["port"])
	}
	return fmt.Sprintf("proxy-%d", idx+1)
}

func (s *SubClashService) getProxies(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []map[string]any {
	stream := s.streamData(inbound.StreamSettings)
	// For node-managed inbounds the Clash proxy "server" must be the
	// node's address, not the request host. resolveInboundAddress handles
	// the node→subscriber-host fallback chain.
	defaultDest := subReq.resolveInboundAddress(inbound)
	if defaultDest == "" {
		defaultDest = host
	}
	externalProxies, ok := stream["externalProxy"].([]any)
	hasExternalProxy := ok && len(externalProxies) > 0
	if !hasExternalProxy {
		externalProxies = []any{map[string]any{
			"forceTls": "same",
			"dest":     defaultDest,
			"port":     float64(inbound.Port),
			"remark":   "",
		}}
	}
	delete(stream, "externalProxy")
	network, _ := stream["network"].(string)

	proxies := make([]map[string]any, 0, len(externalProxies))
	for _, ep := range externalProxies {
		extPrxy := ep.(map[string]any)
		// Expand the host's {{VAR}} remark template for this client (no-op for
		// the synthetic/legacy entry) before it becomes the proxy name.
		subReq.renderHostRemark(inbound, client, extPrxy, network)
		workingInbound := *inbound
		workingInbound.Listen = extPrxy["dest"].(string)
		workingInbound.Port = int(extPrxy["port"].(float64))
		workingStream := cloneStreamForExternalProxy(stream)

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
		security, _ := workingStream["security"].(string)
		if hasExternalProxy {
			applyExternalProxyTLSToStream(extPrxy, workingStream, security)
		}
		applyHostStreamOverrides(extPrxy, workingStream)

		proxy := s.buildProxy(subReq, &workingInbound, client, workingStream, extPrxy)
		if len(proxy) > 0 {
			// Host-only mihomo knob: ip-version is a top-level proxy field, set
			// last so it cannot be clobbered. Absent for legacy externalProxy.
			if v, _ := extPrxy["mihomoIpVersion"].(string); v != "" {
				proxy["ip-version"] = v
			}
			proxies = append(proxies, proxy)
		}
	}
	return proxies
}

func (s *SubClashService) buildProxy(subReq *SubService, inbound *model.Inbound, client model.Client, stream map[string]any, ep map[string]any) map[string]any {
	// Hysteria has its own transport + TLS model, applyTransport /
	// applySecurity don't fit.
	if inbound.Protocol == model.Hysteria {
		return s.buildHysteriaProxy(subReq, inbound, client, ep)
	}
	if inbound.Protocol == model.WireGuard {
		return s.buildWireguardProxy(subReq, inbound, client, ep)
	}

	network, _ := stream["network"].(string)

	proxy := map[string]any{
		"name":   subReq.endpointRemark(inbound, client.Email, ep, network),
		"server": inbound.Listen,
		"port":   inbound.Port,
		"udp":    true,
	}
	if !s.applyTransport(proxy, network, stream) {
		return nil
	}

	switch inbound.Protocol {
	case model.VMESS:
		proxy["type"] = "vmess"
		proxy["uuid"] = client.ID
		proxy["alterId"] = 0
		proxy["cipher"] = normalizeVmessSecurity(client.Security)
	case model.VLESS:
		proxy["type"] = "vless"
		proxy["uuid"] = applyVlessRoute(client.ID, hostVlessRoute(ep))
		inboundSettings := subReq.linkSettings(inbound)
		streamSecurity, _ := stream["security"].(string)
		if client.Flow != "" && vlessFlowAllowed(network, streamSecurity, inboundSettings) {
			proxy["flow"] = client.Flow
		}
		if encryption, ok := inboundSettings["encryption"].(string); ok {
			encryption = strings.TrimSpace(encryption)
			if encryption != "" && encryption != "none" {
				proxy["encryption"] = encryption
			}
		}
	case model.Trojan:
		proxy["type"] = "trojan"
		proxy["password"] = client.Password
	case model.Shadowsocks:
		proxy["type"] = "ss"
		proxy["password"] = client.Password
		inboundSettings := subReq.linkSettings(inbound)
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

// buildHysteriaProxy produces a mihomo-compatible Clash entry for a
// Hysteria (v1) or Hysteria2 inbound. It reads `inbound.StreamSettings`
// directly instead of going through streamData/tlsData, because those
// helpers prune fields (like `allowInsecure` / the salamander obfs
// block) that the hysteria proxy wants preserved.
func (s *SubClashService) buildHysteriaProxy(subReq *SubService, inbound *model.Inbound, client model.Client, ep map[string]any) map[string]any {
	inboundSettings := subReq.linkSettings(inbound)

	proxyType := "hysteria2"
	authKey := "password"
	if v, ok := inboundSettings["version"].(float64); ok && int(v) == 1 {
		proxyType = "hysteria"
		authKey = "auth-str"
	}

	proxy := map[string]any{
		"name":   subReq.endpointRemark(inbound, client.Email, ep, "quic"),
		"type":   proxyType,
		"server": inbound.Listen,
		"port":   inbound.Port,
		"udp":    true,
		authKey:  client.Auth,
	}

	var rawStream map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &rawStream)

	// TLS details — hysteria always uses TLS.
	if tlsSettings, ok := rawStream["tlsSettings"].(map[string]any); ok {
		if serverName, ok := tlsSettings["serverName"].(string); ok && serverName != "" {
			proxy["sni"] = serverName
		}
		if alpnList, ok := tlsSettings["alpn"].([]any); ok && len(alpnList) > 0 {
			out := make([]string, 0, len(alpnList))
			for _, a := range alpnList {
				if s, ok := a.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				proxy["alpn"] = out
			}
		}
		if inner, ok := tlsSettings["settings"].(map[string]any); ok {
			if insecure, ok := inner["allowInsecure"].(bool); ok && insecure {
				proxy["skip-cert-verify"] = true
			}
			if fp, ok := inner["fingerprint"].(string); ok && fp != "" {
				proxy["client-fingerprint"] = fp
			}
		}
	}
	if insecure, ok := ep["allowInsecure"].(bool); ok && insecure {
		proxy["skip-cert-verify"] = true
	}

	// Salamander obfs (Hysteria2). Read the same finalmask.udp[salamander]
	// block the subscription link generator uses.
	if finalmask, ok := rawStream["finalmask"].(map[string]any); ok {
		if udpMasks, ok := finalmask["udp"].([]any); ok {
			for _, m := range udpMasks {
				mask, _ := m.(map[string]any)
				if mask == nil || mask["type"] != "salamander" {
					continue
				}
				settings, _ := mask["settings"].(map[string]any)
				if pw, ok := settings["password"].(string); ok && pw != "" {
					proxy["obfs"] = "salamander"
					proxy["obfs-password"] = pw
					break
				}
			}
		}
	}

	// UDP port hopping. mihomo reads the range from a dedicated `ports`
	// field (the base `port` stays as the redirect target).
	if hopPorts := hysteriaHopPorts(rawStream); hopPorts != "" {
		proxy["ports"] = hopPorts
	}

	return proxy
}

// buildWireguardProxy produces a mihomo-compatible Clash entry for a native
// WireGuard inbound, mirroring genWireguardLink: the peer public key is derived
// from the inbound secretKey, while the private key, tunnel address, and
// pre-shared key come from the client. Returns nil when the client has no key.
func (s *SubClashService) buildWireguardProxy(subReq *SubService, inbound *model.Inbound, client model.Client, ep map[string]any) map[string]any {
	if client.PrivateKey == "" {
		return nil
	}

	var inboundSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
	secretKey, _ := inboundSettings["secretKey"].(string)

	proxy := map[string]any{
		"name":        subReq.endpointRemark(inbound, client.Email, ep, ""),
		"type":        "wireguard",
		"server":      inbound.Listen,
		"port":        inbound.Port,
		"udp":         true,
		"private-key": client.PrivateKey,
	}
	if secretKey != "" {
		if pub, err := wgutil.PublicKeyFromPrivate(secretKey); err == nil {
			proxy["public-key"] = pub
		}
	}
	if client.PreSharedKey != "" {
		proxy["pre-shared-key"] = client.PreSharedKey
	}
	if client.KeepAlive > 0 {
		proxy["persistent-keepalive"] = client.KeepAlive
	}
	for _, addr := range client.AllowedIPs {
		ip := stripCIDR(addr)
		if ip == "" {
			continue
		}
		if strings.Contains(ip, ":") {
			proxy["ipv6"] = ip
		} else {
			proxy["ip"] = ip
		}
	}
	if mtu, ok := inboundSettings["mtu"].(float64); ok && mtu > 0 {
		proxy["mtu"] = int(mtu)
	}
	if dns, _ := inboundSettings["dns"].(string); dns != "" {
		servers := make([]string, 0)
		for _, server := range strings.Split(dns, ",") {
			if server = strings.TrimSpace(server); server != "" {
				servers = append(servers, server)
			}
		}
		if len(servers) > 0 {
			proxy["dns"] = servers
		}
	}

	return proxy
}

// buildXhttpClashOpts converts xhttpSettings from 3x-ui's camelCase JSON
// storage into the kebab-case map that Mihomo expects under xhttp-opts.
//
// Only client-relevant fields are included (allowlist approach).
// Server-only fields (noSSEHeader, scMaxBufferedPosts, scStreamUpServerSecs,
// serverMaxHeaderBytes) are automatically excluded because they are not in
// the mapping. This is intentional — when Mihomo adds new fields, the mapping
// must be updated explicitly rather than leaking unverified fields to clients.
//
// Returns nil if no non-trivial fields are present.
func buildXhttpClashOpts(xhttp map[string]any) map[string]any {
	if xhttp == nil {
		return nil
	}
	opts := map[string]any{}

	// Direct fields: path, mode
	if v, ok := xhttp["path"].(string); ok && v != "" {
		opts["path"] = v
	}
	if v, ok := xhttp["mode"].(string); ok && v != "" {
		opts["mode"] = v
	}

	// Host: explicit host field wins, then fall back to headers.Host
	host := ""
	if v, ok := xhttp["host"].(string); ok && v != "" {
		host = v
	} else if headers, ok := xhttp["headers"].(map[string]any); ok {
		host = searchHost(headers)
	}
	if host != "" {
		opts["host"] = host
	}

	type xhttpStringField struct{ src, dst, skipValue string }

	stringFields := []xhttpStringField{
		{"xPaddingBytes", "x-padding-bytes", ""},
		{"uplinkHTTPMethod", "uplink-http-method", ""},
		{"sessionIDPlacement", "session-id-placement", ""},
		{"sessionIDKey", "session-id-key", ""},
		{"sessionIDTable", "session-id-table", ""},
		{"sessionIDLength", "session-id-length", ""},
		{"seqPlacement", "seq-placement", ""},
		{"seqKey", "seq-key", ""},
		{"uplinkDataPlacement", "uplink-data-placement", ""},
		{"uplinkDataKey", "uplink-data-key", ""},
		{"scMaxEachPostBytes", "sc-max-each-post-bytes", "1000000"},
		{"scMinPostsIntervalMs", "sc-min-posts-interval-ms", "30"},
	}

	for _, f := range stringFields {
		if v, ok := xhttp[f.src].(string); ok && v != "" && (f.skipValue == "" || v != f.skipValue) {
			opts[f.dst] = v
		}
	}

	// Legacy inbounds (pre xray-core #6258) stored sessionPlacement/sessionKey.
	// Fall back to them so not-yet-resaved configs still map. Mirrors the
	// frontend migration.
	for _, f := range []xhttpStringField{
		{"sessionPlacement", "session-id-placement", ""},
		{"sessionKey", "session-id-key", ""},
	} {
		if _, exists := opts[f.dst]; exists {
			continue
		}
		if v, ok := xhttp[f.src].(string); ok && v != "" {
			opts[f.dst] = v
		}
	}

	// Bool fields (truthy only)
	if v, ok := xhttp["noGRPCHeader"].(bool); ok && v {
		opts["no-grpc-header"] = true
	}
	if v, ok := xhttp["xPaddingObfsMode"].(bool); ok && v {
		opts["x-padding-obfs-mode"] = true
		// Padding obfs gated fields
		for _, field := range []struct{ src, dst string }{
			{"xPaddingKey", "x-padding-key"},
			{"xPaddingHeader", "x-padding-header"},
			{"xPaddingPlacement", "x-padding-placement"},
			{"xPaddingMethod", "x-padding-method"},
		} {
			if v, ok := xhttp[field.src].(string); ok && v != "" {
				opts[field.dst] = v
			}
		}
	}

	// Non-zero value fields
	if v, ok := nonZeroShareValue(xhttp["uplinkChunkSize"]); ok {
		opts["uplink-chunk-size"] = v
	}

	// Nested object: xmux → reuse-settings
	if xmux, ok := xhttp["xmux"].(map[string]any); ok && len(xmux) > 0 {
		reuse := map[string]any{}
		for _, f := range []struct{ src, dst string }{
			{"maxConcurrency", "max-concurrency"},
			{"maxConnections", "max-connections"},
			{"cMaxReuseTimes", "c-max-reuse-times"},
			{"hMaxRequestTimes", "h-max-request-times"},
			{"hMaxReusableSecs", "h-max-reusable-secs"},
		} {
			if v, ok := xmux[f.src].(string); ok && v != "" {
				reuse[f.dst] = v
			}
		}
		if v, ok := nonZeroShareValue(xmux["hKeepAlivePeriod"]); ok {
			reuse["h-keep-alive-period"] = v
		}
		if len(reuse) > 0 {
			opts["reuse-settings"] = reuse
		}
	}

	// Headers (drop Host key)
	if rawHeaders, ok := xhttp["headers"].(map[string]any); ok && len(rawHeaders) > 0 {
		out := map[string]any{}
		for k, v := range rawHeaders {
			if strings.EqualFold(k, "host") {
				continue
			}
			out[k] = v
		}
		if len(out) > 0 {
			opts["headers"] = out
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return opts
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
	case "httpupgrade":
		proxy["network"] = "httpupgrade"
		hu, _ := stream["httpupgradeSettings"].(map[string]any)
		opts := map[string]any{}
		if hu != nil {
			if path, ok := hu["path"].(string); ok && path != "" {
				opts["path"] = path
			}
			host := ""
			if v, ok := hu["host"].(string); ok && v != "" {
				host = v
			} else if headers, ok := hu["headers"].(map[string]any); ok {
				host = searchHost(headers)
			}
			if host != "" {
				opts["headers"] = map[string]any{"Host": host}
			}
		}
		if len(opts) > 0 {
			proxy["http-upgrade-opts"] = opts
		}
		return true
	case "xhttp":
		proxy["network"] = "xhttp"
		xhttp, _ := stream["xhttpSettings"].(map[string]any)
		opts := buildXhttpClashOpts(xhttp)
		if opts != nil {
			proxy["xhttp-opts"] = opts
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
			if alpn, ok := externalProxyALPNList(tlsSettings["alpn"]); ok {
				out := make([]string, 0, len(alpn))
				for _, item := range alpn {
					if s, ok := item.(string); ok && s != "" {
						out = append(out, s)
					}
				}
				if len(out) > 0 {
					proxy["alpn"] = out
				}
			}
			if inner, ok := tlsSettings["settings"].(map[string]any); ok {
				if insecure, ok := inner["allowInsecure"].(bool); ok && insecure {
					proxy["skip-cert-verify"] = true
				}
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
	_ = json.Unmarshal([]byte(stream), &streamSettings)
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
	if pins, ok := tlsClientSettings["pinnedPeerCertSha256"].([]any); ok && len(pins) > 0 {
		tlsData["pin-sha256"] = pins
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
	maps.Copy(dst, src)
	return dst
}

func mergeClashRulesYAML(base map[string]any, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var custom any
	if err := yaml.Unmarshal([]byte(raw), &custom); err != nil {
		mergeClashRules(base, linesToClashRules(raw))
		return nil
	}

	switch typed := custom.(type) {
	case []any:
		mergeClashRules(base, typed)
	case map[string]any:
		for key, value := range typed {
			if key == "rules" {
				if ruleList, ok := asAnySlice(value); ok {
					mergeClashRules(base, ruleList)
				}
				continue
			}
			base[key] = value
		}
	default:
		mergeClashRules(base, linesToClashRules(raw))
	}

	return nil
}

func mergeClashRules(base map[string]any, customRules []any) {
	if len(customRules) == 0 {
		return
	}

	baseRules, _ := asAnySlice(base["rules"])
	if hasClashMatchRule(customRules) {
		base["rules"] = customRules
		return
	}

	merged := make([]any, 0, len(customRules)+len(baseRules))
	merged = append(merged, customRules...)
	merged = append(merged, baseRules...)
	base["rules"] = merged
}

func asAnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, true
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func hasClashMatchRule(rules []any) bool {
	for _, rule := range rules {
		ruleText, ok := rule.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(ruleText, ",", 2)
		if strings.EqualFold(strings.TrimSpace(parts[0]), "MATCH") {
			return true
		}
	}
	return false
}

func linesToClashRules(raw string) []any {
	lines := strings.Split(raw, "\n")
	rules := make([]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rules = append(rules, line)
	}
	return rules
}
