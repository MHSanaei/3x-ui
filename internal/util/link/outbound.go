// Package link provides parsers for VPN share links (vmess://, vless://, etc.)
// and subscription bodies (typically base64-encoded newline lists of such links).
// The output shape matches the wire format used by the panel's Xray template
// outbounds array so that parsed objects can be injected directly.
package link

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Outbound is the minimal shape we emit for each parsed link.
// Extra fields (mux, etc.) are carried inside settings/streamSettings.
type Outbound map[string]any

// ParseResult holds a parsed outbound together with a stable identity string
// that can be used to correlate the same logical server across refreshes
// (even if the remark changes).
type ParseResult struct {
	Outbound Outbound
	Identity string
}

// ParseSubscriptionBody accepts the raw body returned by a subscription URL.
// It handles the common case where the body is a base64-encoded blob of
// newline-separated links, and also tolerates an already-decoded text body.
// It returns the list of successfully parsed outbounds (in order) and their
// corresponding identities.
func ParseSubscriptionBody(body []byte) ([]Outbound, []string, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, nil, nil
	}

	// Try base64 decode first (standard and URL-safe variants).
	if decoded, ok := tryBase64(text); ok {
		text = strings.TrimSpace(decoded)
	}

	lines := splitLines(text)
	var outbounds []Outbound
	var identities []string

	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		res, err := ParseLink(ln)
		if err != nil || res == nil {
			// Ignore unparseable lines (comments, unsupported protocols, etc.)
			continue
		}
		outbounds = append(outbounds, res.Outbound)
		identities = append(identities, res.Identity)
	}
	return outbounds, identities, nil
}

func tryBase64(s string) (string, bool) {
	// Remove whitespace that some providers insert.
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)

	// Common padding fix
	for len(clean)%4 != 0 {
		clean += "="
	}

	// Standard
	if b, err := base64.StdEncoding.DecodeString(clean); err == nil {
		return string(b), true
	}
	// URL-safe (no padding)
	if b, err := base64.RawURLEncoding.DecodeString(clean); err == nil {
		return string(b), true
	}
	// URL-safe with padding
	if b, err := base64.URLEncoding.DecodeString(clean); err == nil {
		return string(b), true
	}
	return "", false
}

func splitLines(s string) []string {
	// Accept \n, \r\n, and also some providers use literal \n in the text.
	s = strings.ReplaceAll(s, `\n`, "\n")
	return strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' })
}

// ParseLink parses a single share link and returns the outbound object plus
// a stable identity for tag correlation. Supported schemes:
//   - vmess://
//   - vless://
//   - trojan://
//   - ss:// (modern and legacy)
//   - hysteria2:// (also hy2://)
//   - wireguard:// (also wg://)
func ParseLink(link string) (*ParseResult, error) {
	link = strings.TrimSpace(link)
	switch {
	case strings.HasPrefix(link, "vmess://"):
		return parseVmess(link)
	case strings.HasPrefix(link, "vless://"):
		return parseVless(link)
	case strings.HasPrefix(link, "trojan://"):
		return parseTrojan(link)
	case strings.HasPrefix(link, "ss://"):
		return parseShadowsocks(link)
	case strings.HasPrefix(link, "hysteria2://"), strings.HasPrefix(link, "hy2://"):
		return parseHysteria2(link)
	case strings.HasPrefix(link, "wireguard://"), strings.HasPrefix(link, "wg://"):
		return parseWireguard(link)
	default:
		return nil, fmt.Errorf("unsupported link scheme")
	}
}

// --- vmess ---

func parseVmess(link string) (*ParseResult, error) {
	b64 := strings.TrimPrefix(link, "vmess://")
	// vmess:// base64(json)
	raw, err := base64.StdEncoding.DecodeString(padBase64(b64))
	if err != nil {
		// Some providers use raw URL-safe
		raw, err = base64.RawURLEncoding.DecodeString(b64)
	}
	if err != nil {
		return nil, fmt.Errorf("vmess decode: %w", err)
	}
	var j map[string]any
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}

	identity := vmessIdentity(j)

	network := getString(j, "net", "tcp")
	security := "none"
	if tls, _ := j["tls"].(string); tls == "tls" {
		security = "tls"
	}
	stream := buildStream(network, security)

	// Map known fields (best effort, matching frontend parser coverage)
	switch network {
	case "ws":
		if host, ok := j["host"].(string); ok {
			setWS(stream, host, getString(j, "path", "/"))
		}
	case "grpc":
		svc := getString(j, "path", "")
		if auth, ok := j["authority"].(string); ok && auth != "" {
			(stream["grpcSettings"].(map[string]any))["authority"] = auth
		}
		(stream["grpcSettings"].(map[string]any))["serviceName"] = svc
		(stream["grpcSettings"].(map[string]any))["multiMode"] = getString(j, "type", "") == "multi"
	case "httpupgrade":
		setHTTPUpgrade(stream, getString(j, "host", ""), getString(j, "path", "/"))
	case "xhttp":
		xh := stream["xhttpSettings"].(map[string]any)
		xh["host"] = getString(j, "host", "")
		xh["path"] = getString(j, "path", "/")
		if m := getString(j, "mode", ""); m != "" {
			xh["mode"] = m
		}
		// xhttp advanced keys are passed through if present in the json
		for _, k := range []string{"xPaddingBytes", "scMaxEachPostBytes", "scMinPostsIntervalMs"} {
			if v, ok := j[k]; ok {
				xh[k] = v
			}
		}
	case "tcp":
		if getString(j, "type", "") == "http" {
			stream["tcpSettings"] = map[string]any{
				"header": map[string]any{
					"type": "http",
					"request": map[string]any{
						"version": "1.1",
						"method":  "GET",
						"path":    splitComma(getString(j, "path", "/")),
						"headers": map[string]any{"Host": splitComma(getString(j, "host", ""))},
					},
				},
			}
		}
	}

	if security == "tls" {
		tls := stream["tlsSettings"].(map[string]any)
		tls["serverName"] = getString(j, "sni", "")
		tls["fingerprint"] = getString(j, "fp", "")
		if alpn := getString(j, "alpn", ""); alpn != "" {
			tls["alpn"] = splitComma(alpn)
		}
	}

	port := num(j["port"])
	scy := getString(j, "scy", "auto")
	if scy == "none" || scy == "zero" {
		scy = "auto"
	}
	ob := Outbound{
		"protocol": "vmess",
		"tag":      getString(j, "ps", ""),
		"settings": map[string]any{
			"vnext": []any{
				map[string]any{
					"address": getString(j, "add", ""),
					"port":    port,
					"users": []any{
						map[string]any{
							"id":       getString(j, "id", ""),
							"security": scy,
						},
					},
				},
			},
		},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: ob, Identity: identity}, nil
}

func vmessIdentity(j map[string]any) string {
	// Remove ps (remark) for identity
	core := map[string]any{}
	for k, v := range j {
		if k == "ps" {
			continue
		}
		core[k] = v
	}
	b, _ := json.Marshal(core)
	return "vmess:" + string(b)
}

// --- vless / trojan (URL forms) ---

func parseVless(link string) (*ParseResult, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "vless" {
		return nil, fmt.Errorf("not vless")
	}
	id := u.User.Username()
	host := u.Hostname()
	port := defaultPort(u.Port(), 443)
	params := u.Query()
	network := params.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := params.Get("security")
	if security == "" {
		security = "none"
	}
	stream := buildStream(network, security)
	applyTransport(stream, params)
	applySecurity(stream, params)
	applyFinalMask(stream, params)

	identity := "vless:" + u.Scheme + "://" + id + "@" + host + ":" + strconv.Itoa(port) + "?" + canonicalQuery(params)

	ob := Outbound{
		"protocol": "vless",
		"tag":      decodeHash(u.Fragment),
		"settings": map[string]any{
			"address":    host,
			"port":       port,
			"id":         id,
			"flow":       params.Get("flow"),
			"encryption": firstNonEmpty(params.Get("encryption"), "none"),
		},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: ob, Identity: identity}, nil
}

func parseTrojan(link string) (*ParseResult, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "trojan" {
		return nil, fmt.Errorf("not trojan")
	}
	pw := u.User.Username()
	host := u.Hostname()
	port := defaultPort(u.Port(), 443)
	params := u.Query()
	network := params.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := params.Get("security")
	if security == "" {
		security = "tls"
	}
	stream := buildStream(network, security)
	applyTransport(stream, params)
	applySecurity(stream, params)
	applyFinalMask(stream, params)

	identity := "trojan:" + u.Scheme + "://" + pw + "@" + host + ":" + strconv.Itoa(port) + "?" + canonicalQuery(params)

	ob := Outbound{
		"protocol": "trojan",
		"tag":      decodeHash(u.Fragment),
		"settings": map[string]any{
			"servers": []any{
				map[string]any{"address": host, "port": port, "password": pw},
			},
		},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: ob, Identity: identity}, nil
}

// --- shadowsocks ---

func parseShadowsocks(link string) (*ParseResult, error) {
	// Two shapes:
	//   ss://base64(method:pass)@host:port#remark
	//   ss://base64(method:pass@host:port)#remark
	remark := ""
	if i := strings.Index(link, "#"); i >= 0 {
		remark, _ = url.QueryUnescape(link[i+1:])
		link = link[:i]
	}
	if i := strings.Index(link, "?"); i >= 0 {
		link = link[:i]
	}
	core := strings.TrimPrefix(link, "ss://")
	at := strings.Index(core, "@")
	if at >= 0 {
		// modern
		userB64 := core[:at]
		hp := strings.TrimRight(core[at+1:], "/")
		userInfo, err := base64DecodeFlexible(userB64)
		if err != nil {
			// SIP022 (2022-blake3-*) userinfo is percent-encoded, not base64.
			if dec, uerr := url.QueryUnescape(userB64); uerr == nil {
				userInfo = dec
			} else {
				userInfo = userB64 // not b64, rare
			}
		}
		colon := strings.LastIndex(hp, ":")
		if colon < 0 {
			return nil, fmt.Errorf("bad ss host:port")
		}
		host := hp[:colon]
		port, err := strconv.Atoi(hp[colon+1:])
		if err != nil {
			return nil, fmt.Errorf("bad ss port %q: %w", hp[colon+1:], err)
		}
		method, pass := splitMethodPass(userInfo)
		identity := "ss:" + method + ":" + pass + "@" + host + ":" + strconv.Itoa(port)
		ob := Outbound{
			"protocol": "shadowsocks",
			"tag":      remark,
			"settings": map[string]any{
				"servers": []any{
					map[string]any{"address": host, "port": port, "password": pass, "method": method},
				},
			},
		}
		return &ParseResult{Outbound: ob, Identity: identity}, nil
	}
	// legacy: whole thing b64
	dec, err := base64DecodeFlexible(core)
	if err != nil {
		return nil, err
	}
	at = strings.Index(dec, "@")
	if at < 0 {
		return nil, fmt.Errorf("bad legacy ss")
	}
	userInfo := dec[:at]
	hp := dec[at+1:]
	colon := strings.LastIndex(hp, ":")
	if colon < 0 {
		return nil, fmt.Errorf("bad legacy ss hp")
	}
	host := hp[:colon]
	port, err := strconv.Atoi(hp[colon+1:])
	if err != nil {
		return nil, fmt.Errorf("bad legacy ss port %q: %w", hp[colon+1:], err)
	}
	method, pass := splitMethodPass(userInfo)
	identity := "ss:" + method + ":" + pass + "@" + host + ":" + strconv.Itoa(port)
	ob := Outbound{
		"protocol": "shadowsocks",
		"tag":      remark,
		"settings": map[string]any{
			"servers": []any{
				map[string]any{"address": host, "port": port, "password": pass, "method": method},
			},
		},
	}
	return &ParseResult{Outbound: ob, Identity: identity}, nil
}

func splitMethodPass(userInfo string) (string, string) {
	before, after, ok := strings.Cut(userInfo, ":")
	if !ok {
		return "2022-blake3-aes-128-gcm", userInfo // guess
	}
	return before, after
}

// --- hysteria2 ---

func parseHysteria2(link string) (*ParseResult, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "hysteria2" && u.Scheme != "hy2" {
		return nil, fmt.Errorf("not hysteria2")
	}
	auth := u.User.Username()
	host := u.Hostname()
	port := defaultPort(u.Port(), 443)
	params := u.Query()

	stream := map[string]any{
		"network":  "hysteria",
		"security": "tls",
		"hysteriaSettings": map[string]any{
			"version":        2,
			"auth":           auth,
			"udpIdleTimeout": 60,
		},
		"tlsSettings": map[string]any{
			"serverName":           params.Get("sni"),
			"alpn":                 splitCommaOrDefault(params.Get("alpn"), []string{"h3"}),
			"fingerprint":          params.Get("fp"),
			"echConfigList":        params.Get("ech"),
			"verifyPeerCertByName": "",
			"pinnedPeerCertSha256": params.Get("pinSHA256"),
		},
	}
	applyFinalMask(stream, params)

	identity := "hysteria2:" + auth + "@" + host + ":" + strconv.Itoa(port) + "?" + canonicalQuery(params)

	ob := Outbound{
		"protocol":       "hysteria",
		"tag":            decodeHash(u.Fragment),
		"settings":       map[string]any{"address": host, "port": port, "version": 2},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: ob, Identity: identity}, nil
}

// --- wireguard ---

func parseWireguard(link string) (*ParseResult, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "wireguard" && u.Scheme != "wg" {
		return nil, fmt.Errorf("not wireguard")
	}
	secret, _ := url.QueryUnescape(u.User.Username())
	params := u.Query()
	host := u.Hostname()
	portStr := u.Port()
	endpoint := host
	if portStr != "" {
		endpoint = host + ":" + portStr
	}

	addrRaw := firstParam(params, "address", "ip")
	allowedRaw := firstParam(params, "allowedips", "allowed_ips")
	addrs := splitComma(addrRaw)
	if len(addrs) == 0 {
		addrs = []string{"0.0.0.0/0", "::/0"}
	}
	allowed := splitComma(allowedRaw)
	if len(allowed) == 0 {
		allowed = []string{"0.0.0.0/0", "::/0"}
	}

	peer := map[string]any{
		"publicKey":  firstParam(params, "publickey", "publicKey", "public_key", "peerPublicKey"),
		"endpoint":   endpoint,
		"allowedIPs": allowed,
	}
	if psk := firstParam(params, "presharedkey", "preshared_key", "pre-shared-key", "psk"); psk != "" {
		peer["preSharedKey"] = psk
	}
	if ka := firstParam(params, "keepalive", "persistentkeepalive", "persistent_keepalive"); ka != "" {
		if n, err := strconv.Atoi(ka); err == nil {
			peer["keepAlive"] = n
		}
	}

	settings := map[string]any{
		"secretKey": secret,
		"address":   addrs,
		"peers":     []any{peer},
	}
	if mtu := params.Get("mtu"); mtu != "" {
		if n, err := strconv.Atoi(mtu); err == nil {
			settings["mtu"] = n
		}
	}
	if res := params.Get("reserved"); res != "" {
		parts := splitComma(res)
		var iv []int
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				iv = append(iv, n)
			}
		}
		if len(iv) > 0 {
			settings["reserved"] = iv
		}
	}

	identity := "wireguard:" + secret + "@" + endpoint + "?" + canonicalQuery(params)

	ob := Outbound{
		"protocol": "wireguard",
		"tag":      decodeHash(u.Fragment),
		"settings": settings,
	}
	return &ParseResult{Outbound: ob, Identity: identity}, nil
}

// --- helpers ---

func buildStream(network, security string) map[string]any {
	stream := map[string]any{"network": network, "security": security}
	switch network {
	case "tcp":
		stream["tcpSettings"] = map[string]any{"header": map[string]any{"type": "none"}}
	case "kcp":
		stream["kcpSettings"] = map[string]any{
			"mtu": 1350, "tti": 20, "uplinkCapacity": 5, "downlinkCapacity": 20,
			"cwndMultiplier": 1, "maxSendingWindow": 2097152,
		}
	case "ws":
		stream["wsSettings"] = map[string]any{"path": "/", "host": "", "headers": map[string]any{}, "heartbeatPeriod": 0}
	case "grpc":
		stream["grpcSettings"] = map[string]any{"serviceName": "", "authority": "", "multiMode": false}
	case "httpupgrade":
		stream["httpupgradeSettings"] = map[string]any{"path": "/", "host": "", "headers": map[string]any{}}
	case "xhttp":
		// No scMaxEachPostBytes/scMinPostsIntervalMs seed: xray-core's own
		// defaults apply, and the literal values fingerprint traffic (#5141).
		stream["xhttpSettings"] = map[string]any{
			"path": "/", "host": "", "mode": "auto", "headers": map[string]any{},
			"xPaddingBytes": "100-1000",
		}
	default:
		stream["tcpSettings"] = map[string]any{"header": map[string]any{"type": "none"}}
	}
	switch security {
	case "tls":
		stream["tlsSettings"] = map[string]any{
			"serverName": "", "alpn": []any{}, "fingerprint": "",
			"echConfigList": "", "verifyPeerCertByName": "", "pinnedPeerCertSha256": "",
		}
	case "reality":
		stream["realitySettings"] = map[string]any{
			"publicKey": "", "fingerprint": "chrome", "serverName": "",
			"shortId": "", "spiderX": "", "mldsa65Verify": "",
		}
	}
	return stream
}

func setWS(stream map[string]any, host, path string) {
	ws := stream["wsSettings"].(map[string]any)
	ws["host"] = host
	ws["path"] = path
}

func setHTTPUpgrade(stream map[string]any, host, path string) {
	h := stream["httpupgradeSettings"].(map[string]any)
	h["host"] = host
	h["path"] = path
}

func applyTransport(stream map[string]any, p url.Values) {
	net := stream["network"].(string)
	host := p.Get("host")
	path := firstNonEmpty(p.Get("path"), "/")
	switch net {
	case "ws":
		setWS(stream, host, path)
	case "grpc":
		gs := stream["grpcSettings"].(map[string]any)
		gs["serviceName"] = firstNonEmpty(p.Get("serviceName"), p.Get("path"))
		gs["authority"] = p.Get("authority")
		gs["multiMode"] = p.Get("mode") == "multi"
	case "httpupgrade":
		setHTTPUpgrade(stream, host, path)
	case "xhttp":
		xh := stream["xhttpSettings"].(map[string]any)
		xh["host"] = host
		xh["path"] = path
		if m := p.Get("mode"); m != "" {
			xh["mode"] = m
		}
		// A few advanced xhttp fields that are commonly carried
		for _, k := range []string{"xPaddingBytes", "scMaxEachPostBytes", "scMinPostsIntervalMs", "uplinkChunkSize"} {
			if v := p.Get(k); v != "" {
				xh[k] = v
			}
		}
	case "tcp":
		if p.Get("headerType") == "http" || p.Get("type") == "http" {
			stream["tcpSettings"] = map[string]any{
				"header": map[string]any{
					"type": "http",
					"request": map[string]any{
						"version": "1.1",
						"method":  "GET",
						"path":    splitComma(path),
						"headers": map[string]any{"Host": splitComma(host)},
					},
				},
			}
		}
	}
}

func applySecurity(stream map[string]any, p url.Values) {
	sec := stream["security"].(string)
	switch sec {
	case "tls":
		tls := stream["tlsSettings"].(map[string]any)
		tls["serverName"] = p.Get("sni")
		tls["fingerprint"] = p.Get("fp")
		if alpn := p.Get("alpn"); alpn != "" {
			tls["alpn"] = splitComma(alpn)
		}
		tls["echConfigList"] = p.Get("ech")
		tls["pinnedPeerCertSha256"] = p.Get("pcs")
	case "reality":
		re := stream["realitySettings"].(map[string]any)
		re["serverName"] = p.Get("sni")
		re["fingerprint"] = firstNonEmpty(p.Get("fp"), "chrome")
		re["publicKey"] = p.Get("pbk")
		re["shortId"] = p.Get("sid")
		re["spiderX"] = p.Get("spx")
		re["mldsa65Verify"] = p.Get("pqv")
	}
}

func applyFinalMask(stream map[string]any, p url.Values) {
	if fm := p.Get("fm"); fm != "" {
		var parsed any
		if json.Unmarshal([]byte(fm), &parsed) == nil {
			sanitizeFinalMaskQuicParams(parsed)
			stream["finalmask"] = parsed
		}
	}
}

// sanitizeFinalMaskQuicParams coerces the strictly numeric quicParams fields
// of a finalmask blob taken verbatim from a share link's fm= parameter.
// Xray-core rejects the whole config at startup when e.g. keepAlivePeriod
// arrives as a duration string like "10s" or an out-of-range integer, so
// numeric strings are parsed, duration strings are converted to whole
// seconds, the ranged fields are clamped to what xray accepts, and anything
// non-finite, negative, absurdly large, or unparseable is dropped so a bad
// value falls back to xray's default instead of killing the config (#5783).
func sanitizeFinalMaskQuicParams(parsed any) {
	fm, ok := parsed.(map[string]any)
	if !ok {
		return
	}
	qp, ok := fm["quicParams"].(map[string]any)
	if !ok {
		return
	}
	numericKeys := []string{
		"initStreamReceiveWindow", "maxStreamReceiveWindow",
		"initConnectionReceiveWindow", "maxConnectionReceiveWindow",
		"maxIdleTimeout", "keepAlivePeriod", "maxIncomingStreams",
	}
	for _, key := range numericKeys {
		raw, exists := qp[key]
		if !exists {
			continue
		}
		n, ok := coerceQuicNumeric(raw)
		if ok {
			n, ok = clampQuicNumeric(key, n)
		}
		if !ok {
			delete(qp, key)
			continue
		}
		qp[key] = int64(n)
	}
}

func coerceQuicNumeric(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return math.Trunc(v), true
	case string:
		if n, err := strconv.ParseFloat(v, 64); err == nil && !math.IsInf(n, 0) && !math.IsNaN(n) {
			return math.Trunc(n), true
		}
		if d, err := time.ParseDuration(v); err == nil {
			return math.Trunc(d.Seconds()), true
		}
	}
	return 0, false
}

// clampQuicNumeric enforces xray-core's QuicParamsConfig validation so a
// coerced value cannot still fail the config load: keepAlivePeriod is 0 or
// 2-60, maxIdleTimeout is 0 or 4-120, maxIncomingStreams is 0 or >= 8.
// quicNumericMax keeps values in plain-integer JSON territory and far below
// the uint64 window fields' range.
const quicNumericMax = float64(1e15)

func clampQuicNumeric(key string, n float64) (float64, bool) {
	if n < 0 || n > quicNumericMax {
		return 0, false
	}
	if n == 0 {
		return 0, true
	}
	switch key {
	case "keepAlivePeriod":
		return math.Min(math.Max(n, 2), 60), true
	case "maxIdleTimeout":
		return math.Min(math.Max(n, 4), 120), true
	case "maxIncomingStreams":
		return math.Max(n, 8), true
	}
	return n, true
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstParam(p url.Values, keys ...string) string {
	for _, k := range keys {
		if v := p.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func canonicalQuery(p url.Values) string {
	// Sort keys for stable identity
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	// simple sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		for _, v := range p[k] {
			parts = append(parts, k+"="+v)
		}
	}
	return strings.Join(parts, "&")
}

func decodeHash(h string) string {
	if h == "" {
		return ""
	}
	if dec, err := url.QueryUnescape(h); err == nil {
		return dec
	}
	return h
}

func defaultPort(p string, def int) int {
	if p == "" {
		return def
	}
	n, err := strconv.Atoi(p)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func num(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

func getString(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitCommaOrDefault(s string, def []string) []string {
	if s == "" {
		return def
	}
	return splitComma(s)
}

func padBase64(s string) string {
	for len(s)%4 != 0 {
		s += "="
	}
	return s
}

func base64DecodeFlexible(s string) (string, error) {
	s = padBase64(s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(b), nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil {
		return string(b), nil
	}
	return "", fmt.Errorf("base64 decode failed")
}

// SlugRemark turns a free-form remark into a tag segment, keeping Unicode
// letters and digits (so non-ASCII remarks like Cyrillic stay readable) and
// replacing every other run of characters with a single dash.
var slugRe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func SlugRemark(remark string) string {
	s := strings.ToLower(strings.TrimSpace(remark))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return ""
	}
	// collapse runs of dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// SuggestTag builds a tag from a prefix and a remark (or index fallback).
// It is intended for initial assignment; stability is handled by the service layer.
func SuggestTag(prefix, remark string, idx int) string {
	base := SlugRemark(remark)
	if base == "" {
		base = fmt.Sprintf("%d", idx)
	}
	p := strings.TrimSuffix(prefix, "-")
	if p != "" {
		return p + "-" + base
	}
	return base
}
