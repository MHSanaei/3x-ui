package sub

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"maps"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// SubService provides business logic for generating subscription links and managing subscription data.
type SubService struct {
	address        string
	remarkTemplate string
	datepicker     string
	// subscriptionBody is true only when rendering the actual subscription
	// content a client app imports (raw /sub fetch, /json, /clash). The remark
	// template's per-client info is emitted there (on the first link); every
	// other context — the sub info page, the panel's link/QR displays — renders
	// the name-only template, like Remnawave.
	subscriptionBody bool
	// usageShown tracks, per client email, whether the info part of the template
	// has already been emitted this request, so it appears on the first body
	// link only. Per-request state; reset in PrepareForRequest.
	usageShown     map[string]bool
	inboundService service.InboundService
	settingService service.SettingService
	// nodesByID is populated per request from the Node table so
	// resolveInboundAddress can return the node's address for any
	// inbound whose NodeID is set. Keeps the per-link host derivation
	// O(1) instead of O(N) DB hits.
	nodesByID map[int]*model.Node
	// statsByEmail maps a client email to its traffic row across ALL inbounds
	// loaded for the request. client_traffics.email is globally unique, so this
	// lets statsForClient resolve usage for a client even on an inbound that
	// doesn't own its row (multi-inbound subscriptions). Filled in
	// getInboundsBySubId; reset per request in PrepareForRequest.
	statsByEmail map[string]xray.ClientTraffic
}

// NewSubService creates a new subscription service with the given configuration.
func NewSubService(remarkTemplate string) *SubService {
	return &SubService{
		remarkTemplate: remarkTemplate,
	}
}

// ForRequest returns a shallow copy with request-scoped state populated.
// Subscription controllers share one base SubService, so request-specific
// fields such as address and nodesByID must live on a per-request copy.
func (s *SubService) ForRequest(host string) *SubService {
	req := *s
	req.PrepareForRequest(host)
	return &req
}

// PrepareForRequest sets per-request state (host + nodes map) on this
// SubService instance. HTTP handlers should call ForRequest instead so the
// controller's shared base service is never mutated by concurrent requests.
func (s *SubService) PrepareForRequest(host string) {
	if !isRoutableHost(host) {
		if d := s.configuredPublicHost(); d != "" {
			host = d
		} else if isLoopbackHost(host) {
			host = "localhost"
		}
	}
	s.address = host
	s.usageShown = map[string]bool{}
	s.statsByEmail = map[string]xray.ClientTraffic{}
	s.loadNodes()
	s.loadRemarkSettings()
}

// loadRemarkSettings populates the per-request remark formatting state so
// every subscription format — raw, JSON, Clash — renders remarks the same way
// (the date formatter reads datepicker). Loading it only in getSubs left
// JSON/Clash with the zero value.
func (s *SubService) loadRemarkSettings() {
	var err error
	s.datepicker, err = s.settingService.GetDatepicker()
	if err != nil {
		s.datepicker = "gregorian"
	}
}

func (s *SubService) configuredPublicHost() string {
	if d, err := s.settingService.GetSubDomain(); err == nil && d != "" {
		return d
	}
	if d, err := s.settingService.GetWebDomain(); err == nil && d != "" {
		return d
	}
	return ""
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

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// listenIsInternalOnly reports whether a bind address is reachable only from
// the same host — a loopback IP or a unix-domain socket. Such an inbound can't
// be dialed directly by a remote client, so when it is the child side of a
// fallback its share link must be projected through the master. A public or
// wildcard listen (""/0.0.0.0/::) is reachable on its own port and advertises
// itself.
func listenIsInternalOnly(listen string) bool {
	if listen == "" {
		return false
	}
	if listen[0] == '@' || listen[0] == '/' {
		return true
	}
	return isLoopbackHost(listen)
}

// matchingClients returns the inbound's clients whose SubID equals subId,
// deduplicated by email. settings.clients can accumulate duplicate entries
// for the same client (multi-node sync/import drift, old DBs): SyncInbound
// dedupes the normalized client_inbounds rows on write but never rewrites
// the legacy JSON, and the subscription builders iterate that JSON — so
// without this guard every duplicate became a duplicate profile in the
// output (#5134). Link generation keys purely on (inbound, email), so
// same-email entries are pure duplicates and dropping them is lossless.
func (s *SubService) matchingClients(inbound *model.Inbound, subId string) []model.Client {
	clients, err := s.inboundService.GetClients(inbound)
	if err != nil {
		logger.Error("SubService - GetClients: Unable to get clients from inbound")
		return nil
	}
	var out []model.Client
	seen := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		if client.SubID != subId {
			continue
		}
		key := strings.ToLower(client.Email)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, client)
	}
	return out
}

// GetSubs retrieves subscription links for a given subscription ID and host.
func (s *SubService) GetSubs(subId string, host string) ([]string, []string, int64, xray.ClientTraffic, error) {
	return s.ForRequest(host).getSubs(subId)
}

func (s *SubService) getSubs(subId string) ([]string, []string, int64, xray.ClientTraffic, error) {
	var result []string
	var emails []string
	var traffic xray.ClientTraffic
	var hasEnabledClient bool
	inbounds, err := s.getInboundsBySubId(subId)
	if err != nil {
		return nil, nil, 0, traffic, err
	}
	externalLinks, err := s.getClientExternalLinksBySubId(subId)
	if err != nil {
		return nil, nil, 0, traffic, err
	}

	if len(inbounds) == 0 && len(externalLinks) == 0 {
		return nil, nil, 0, traffic, nil
	}

	seenEmails := make(map[string]struct{})
	for _, inbound := range inbounds {
		clients := s.matchingClients(inbound, subId)
		if len(clients) == 0 {
			continue
		}
		s.projectThroughFallbackMaster(inbound)
		// Host overrides apply AFTER fallback projection so a host's
		// address/TLS wins over the projected master stream.
		hostEps := s.hostEndpoints(inbound, "raw")
		for _, client := range clients {
			if client.Enable {
				hasEnabledClient = true
			}
			var link string
			if len(hostEps) > 0 {
				link = s.linkFromHosts(inbound, client, hostEps)
			} else {
				link = s.GetLink(inbound, client.Email)
			}
			result = append(result, link)
			emails = append(emails, client.Email)
			seenEmails[client.Email] = struct{}{}
		}
	}
	for _, ext := range externalLinks {
		if ext.Enable {
			hasEnabledClient = true
		}
		for _, el := range expandEntry(ext) {
			if link := applyRemarkToLink(el.Link, el.Name); link != "" {
				result = append(result, link)
				emails = append(emails, ext.Email)
				seenEmails[ext.Email] = struct{}{}
			}
		}
	}

	uniqueEmails := make([]string, 0, len(seenEmails))
	for e := range seenEmails {
		uniqueEmails = append(uniqueEmails, e)
	}
	traffic, lastOnline := s.AggregateTrafficByEmails(uniqueEmails)
	traffic.Enable = hasEnabledClient
	return result, emails, lastOnline, traffic, nil
}

// AggregateTrafficByEmails resolves traffic for every email in one
// query and folds the rows into a single ClientTraffic + lastOnline.
// xray.ClientTraffic.Email is globally unique, so a multi-inbound
// client's single row is attached to exactly one inbound — iterating
// per-inbound ClientStats would miss it on the others. Used by GetSubs,
// SubClashService.GetClash, and SubJsonService.GetJson to keep the
// sub-info header consistent across all three formats.
func (s *SubService) AggregateTrafficByEmails(emails []string) (xray.ClientTraffic, int64) {
	var agg xray.ClientTraffic
	var lastOnline int64
	if len(emails) == 0 {
		return agg, 0
	}
	db := database.GetDB()
	var rows []xray.ClientTraffic
	if err := db.
		Model(&xray.ClientTraffic{}).
		Where("email IN ?", emails).
		Find(&rows).Error; err != nil {
		logger.Warning("SubService - AggregateTrafficByEmails: load by email:", err)
		return agg, 0
	}

	// total/expiry are configured limits owned by the clients table, not the
	// runtime traffic rows. In a multi-node setup the node snapshot can reset
	// client_traffics.total/expiry_time to 0, so fall back to the clients
	// table to keep the Subscription-Userinfo header in sync with the UI (#4645).
	limits := make(map[string][2]int64, len(emails))
	var records []model.ClientRecord
	if err := db.Model(&model.ClientRecord{}).Where("email IN ?", emails).Find(&records).Error; err != nil {
		logger.Warning("SubService - AggregateTrafficByEmails: load client limits:", err)
	} else {
		for _, r := range records {
			limits[r.Email] = [2]int64{r.TotalGB, r.ExpiryTime}
		}
	}

	now := time.Now().UnixMilli()
	first := true
	for _, ct := range rows {
		if ct.LastOnline > lastOnline {
			lastOnline = ct.LastOnline
		}
		total, expiry := ct.Total, ct.ExpiryTime
		if lim, ok := limits[ct.Email]; ok {
			if total == 0 {
				total = lim[0]
			}
			if expiry == 0 {
				expiry = lim[1]
			}
		}
		if first {
			agg.Up = ct.Up
			agg.Down = ct.Down
			agg.Total = total
			agg.ExpiryTime = subscriptionExpiryFromClient(now, expiry)
			first = false
			continue
		}
		agg.Up += ct.Up
		agg.Down += ct.Down
		if agg.Total == 0 || total == 0 {
			agg.Total = 0
		} else {
			agg.Total += total
		}
		normalized := subscriptionExpiryFromClient(now, expiry)
		if normalized != agg.ExpiryTime {
			agg.ExpiryTime = 0
		}
	}
	return agg, lastOnline
}

func subscriptionExpiryFromClient(nowMs, expiryTime int64) int64 {
	if expiryTime > 0 {
		return expiryTime
	}
	if expiryTime < 0 {
		return nowMs + (-expiryTime)
	}
	return 0
}

func (s *SubService) getInboundsBySubId(subId string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Preload("ClientStats").Where(`id in (
		SELECT DISTINCT inbounds.id
		FROM inbounds
		JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id
		JOIN clients ON clients.id = client_inbounds.client_id
		WHERE
			inbounds.protocol in ('vmess','vless','trojan','shadowsocks','hysteria')
			AND clients.sub_id = ? AND inbounds.enable = ?
	)`, subId, true).Order("sub_sort_index ASC").Order("id ASC").Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	s.indexStatsByEmail(inbounds)
	return inbounds, nil
}

// indexStatsByEmail records every loaded inbound's client traffic rows keyed by
// email so statsForClient can resolve a client's usage even on an inbound that
// doesn't own its (globally unique) client_traffics row. See statsByEmail.
func (s *SubService) indexStatsByEmail(inbounds []*model.Inbound) {
	if s.statsByEmail == nil {
		s.statsByEmail = map[string]xray.ClientTraffic{}
	}
	for _, inbound := range inbounds {
		for _, st := range inbound.ClientStats {
			s.statsByEmail[st.Email] = st
		}
	}
}

// projectThroughFallbackMaster mutates the inbound in place so its
// Listen/Port/StreamSettings reflect the externally reachable master
// when applicable. Covers both fallback mechanisms:
//   - panel-tracked: an inbound_fallbacks row where child_id = inbound.Id
//   - legacy unix-socket: inbound.Listen begins with "@" and some VLESS/
//     Trojan inbound's settings.fallbacks references that listen address
//
// Returns true when a projection happened; sub services call this before
// generating links so a child VLESS-WS bound to 127.0.0.1 emits the
// master's :443 + TLS state instead of its own loopback endpoint.
//
// Projection only applies to a child that is not directly reachable on its
// own listen (loopback or a unix-domain socket). An inbound on a public or
// wildcard listen is reachable on its own port, so it advertises its own
// port + security even when a stale fallback rule still names it as a child —
// otherwise its share link would leak the master's port and Reality/TLS
// settings (#4987).
func (s *SubService) projectThroughFallbackMaster(inbound *model.Inbound) bool {
	if inbound == nil {
		return false
	}
	if !listenIsInternalOnly(inbound.Listen) {
		return false
	}
	db := database.GetDB()
	var master *model.Inbound

	var rule model.InboundFallback
	if err := db.Where("child_id = ?", inbound.Id).
		Order("sort_order ASC, id ASC").
		First(&rule).Error; err == nil {
		var m model.Inbound
		if err := db.Where("id = ?", rule.MasterId).First(&m).Error; err == nil {
			master = &m
		}
	}

	if master == nil && len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
		var m model.Inbound
		if err := db.Model(model.Inbound{}).
			Where("JSON_TYPE(settings, '$.fallbacks') = 'array'").
			Where("EXISTS (SELECT * FROM json_each(settings, '$.fallbacks') WHERE json_extract(value, '$.dest') = ?)", inbound.Listen).
			First(&m).Error; err == nil {
			master = &m
		}
	}

	if master == nil {
		return false
	}
	inbound.StreamSettings = mergeStreamFromMaster(inbound.StreamSettings, master.StreamSettings)
	inbound.Listen = master.Listen
	inbound.Port = master.Port
	return true
}

// mergeStreamFromMaster copies the master's security + tlsSettings +
// realitySettings + externalProxy onto the child's stream so the child's
// link advertises the master's TLS / Reality state. Transport (network
// + ws/grpc/etc. settings) stays the child's.
func mergeStreamFromMaster(childStream, masterStream string) string {
	var stream map[string]any
	json.Unmarshal([]byte(childStream), &stream)
	if stream == nil {
		stream = map[string]any{}
	}
	var mst map[string]any
	json.Unmarshal([]byte(masterStream), &mst)
	if mst == nil {
		return childStream
	}
	stream["security"] = mst["security"]
	if v, ok := mst["tlsSettings"]; ok {
		stream["tlsSettings"] = v
	} else {
		delete(stream, "tlsSettings")
	}
	if v, ok := mst["realitySettings"]; ok {
		stream["realitySettings"] = v
	} else {
		delete(stream, "realitySettings")
	}
	if v, ok := mst["externalProxy"]; ok {
		stream["externalProxy"] = v
	}
	out, err := json.MarshalIndent(stream, "", "  ")
	if err != nil {
		return childStream
	}
	return string(out)
}

// GetLink dispatches to the protocol-specific generator for one (inbound, client)
// pair. Returns "" when the inbound's protocol doesn't produce a subscription URL
// (socks, http, mixed, wireguard, dokodemo, tunnel). The returned string may
// contain multiple `\n`-separated URLs when the inbound has externalProxy set.
func (s *SubService) GetLink(inbound *model.Inbound, email string) string {
	switch inbound.Protocol {
	case "vmess":
		return s.genVmessLink(inbound, email)
	case "vless":
		return s.genVlessLink(inbound, email)
	case "trojan":
		return s.genTrojanLink(inbound, email)
	case "shadowsocks":
		return s.genShadowsocksLink(inbound, email)
	case "hysteria":
		return s.genHysteriaLink(inbound, email)
	case "mtproto":
		return s.genMtprotoLink(inbound, email)
	}
	return ""
}

// genMtprotoLink builds a Telegram proxy deep link for an mtproto inbound:
func (s *SubService) genMtprotoLink(inbound *model.Inbound, _ string) string {
	if inbound.Protocol != model.MTProto {
		return ""
	}
	settings := map[string]any{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	secret, _ := settings["secret"].(string)
	if secret == "" {
		if healed, ok := model.HealMtprotoSecret(inbound.Settings); ok {
			_ = json.Unmarshal([]byte(healed), &settings)
			secret, _ = settings["secret"].(string)
		}
	}
	if secret == "" {
		return ""
	}
	params := map[string]string{
		"server": s.resolveInboundAddress(inbound),
		"port":   fmt.Sprintf("%d", inbound.Port),
		"secret": secret,
	}
	return buildLinkWithParams("tg://proxy", params, "")
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

// vlessEncryptionEnabled reports whether the VLESS inbound settings enable
// VLESS-level encryption (vlessenc / ML-KEM). When on, the encryption/decryption
// fields hold a generated dotted string (e.g. "mlkem768x25519plus.native.0rtt.<key>");
// "none" or empty means off. The value is never the literal "vlessenc" — that is
// the `xray vlessenc` CLI subcommand name, not a stored value.
func vlessEncryptionEnabled(settings map[string]any) bool {
	for _, key := range []string{"encryption", "decryption"} {
		if v, ok := settings[key].(string); ok && v != "" && v != "none" {
			return true
		}
	}
	return false
}

// vlessFlowAllowed reports whether a client's XTLS Vision flow belongs in
// generated links/configs. Mirrors inboundCanEnableTlsFlow in
// internal/web/service: Vision runs on TCP with tls/reality (classic), and on
// XHTTP whenever VLESS encryption (vlessenc / ML-KEM) is enabled — there the
// VLESS-level encryption stands in for the transport TLS that Vision relies
// on, regardless of the stream security layer (so XHTTP+REALITY+vlessenc
// keeps its flow too).
func vlessFlowAllowed(network, security string, settings map[string]any) bool {
	switch network {
	case "tcp":
		return security == "tls" || security == "reality"
	case "xhttp":
		return vlessEncryptionEnabled(settings)
	}
	return false
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
	case "reality":
		applyShareRealityParams(stream, params)
	default:
		params["security"] = "none"
	}
	if len(clients[clientIndex].Flow) > 0 && vlessFlowAllowed(streamNetwork, security, settings) {
		params["flow"] = clients[clientIndex].Flow
	}

	externalProxies, _ := stream["externalProxy"].([]any)

	if len(externalProxies) > 0 {
		return s.buildExternalProxyURLLinks(
			externalProxies,
			params,
			security,
			func(dest string, port int) string {
				return fmt.Sprintf("vless://%s@%s", uuid, joinHostPort(dest, port))
			},
			func(ep map[string]any) string {
				return s.endpointRemark(inbound, email, ep)
			},
		)
	}

	link := fmt.Sprintf("vless://%s@%s", uuid, joinHostPort(address, port))
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
	password := encodeUserinfo(clients[clientIndex].Password)
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
				return fmt.Sprintf("trojan://%s@%s", password, joinHostPort(dest, port))
			},
			func(ep map[string]any) string {
				return s.endpointRemark(inbound, email, ep)
			},
		)
	}

	link := fmt.Sprintf("trojan://%s@%s", password, joinHostPort(address, port))
	return buildLinkWithParams(link, params, s.genRemark(inbound, email, ""))
}

// encodeUserinfo percent-encodes a userinfo (password/auth) value so it
// can be safely embedded in a `scheme://<value>@host:port` URL. RFC 3986
// allows `=` in userinfo as a sub-delim, but several Trojan and Hysteria
// clients reject share-links where the password contains literal `/`
// or `=` (notably the common base64-with-padding shape produced by the
// panel). Encode them too — this matches encodeURIComponent() on the
// frontend and round-trips cleanly through net/url's parser.
func encodeUserinfo(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// joinHostPort wraps an IPv6 host in square brackets the way RFC 3986
// requires for URI authorities, while leaving IPv4 addresses and hostnames
// untouched. It also strips any brackets already present on the input so
// callers don't have to normalize upstream.
func joinHostPort(host string, port int) string {
	host = strings.Trim(host, "[]")
	return net.JoinHostPort(host, strconv.Itoa(port))
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

	// SIP002 clients (v2rayN) ignore the xray-native type/headerType/host/path
	// params and only read `plugin`. Re-encode a TCP http header as obfs-local so
	// they build a matching tcp/http outbound (v2rayN forces request path "/").
	if streamNetwork == "tcp" && params["headerType"] == "http" {
		host := params["host"]
		delete(params, "type")
		delete(params, "headerType")
		delete(params, "host")
		delete(params, "path")
		params["plugin"] = "obfs-local;obfs=http;obfs-host=" + host
	}

	// SIP002 userinfo is base64(method:password). For SIP022 (2022-blake3-*) the
	// userinfo MUST NOT be base64-encoded; method and password are percent-encoded.
	var userInfo string
	if strings.HasPrefix(method, "2022") {
		userInfo = fmt.Sprintf("%s:%s:%s",
			url.QueryEscape(method),
			url.QueryEscape(inboundPassword),
			url.QueryEscape(clients[clientIndex].Password))
	} else {
		userInfo = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", method, clients[clientIndex].Password)))
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
				return fmt.Sprintf("ss://%s@%s", userInfo, joinHostPort(dest, port))
			},
			func(ep map[string]any) string {
				return s.endpointRemark(inbound, email, ep)
			},
		)
	}

	link := fmt.Sprintf("ss://%s@%s", userInfo, joinHostPort(address, inbound.Port))
	return buildLinkWithParams(link, params, s.genRemark(inbound, email, ""))
}

func (s *SubService) genHysteriaLink(inbound *model.Inbound, email string) string {
	if inbound.Protocol != model.Hysteria {
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
	auth := encodeUserinfo(clients[clientIndex].Auth)
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
		if echValue, ok := searchKey(tlsSettings, "echConfigList"); ok {
			if ech, _ := echValue.(string); ech != "" {
				params["ech"] = ech
			}
		}
		if pins, ok := pinnedSha256List(tlsSettings); ok {
			for i, p := range pins {
				pins[i] = hysteriaPinHex(p)
			}
			params["pinSHA256"] = strings.Join(pins, ",")
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
			epParams := cloneStringMap(params)
			applyExternalProxyHysteriaParams(ep, epParams)

			link := fmt.Sprintf("%s://%s@%s", protocol, auth, joinHostPort(dest, int(portF)))
			links = append(links, buildLinkWithParams(link, epParams, s.endpointRemark(inbound, email, ep)))
		}
		return strings.Join(links, "\n")
	}

	// No external proxy configured — use the inbound's resolved address so
	// node-managed inbounds get the node's host instead of the central panel's.
	if hopPorts := hysteriaHopPorts(stream); hopPorts != "" {
		params["mport"] = hopPorts
	}
	link := fmt.Sprintf("%s://%s@%s", protocol, auth, joinHostPort(s.resolveInboundAddress(inbound), inbound.Port))
	return buildLinkWithParams(link, params, s.genRemark(inbound, email, ""))
}

// hysteriaHopPorts returns the configured Hysteria2 UDP port-hopping range
// (finalmask.quicParams.udpHop.ports), or "" when port hopping is off. The
// range is emitted as the v2rayN-compatible `mport` query param; the URL port
// field stays numeric so .NET-Uri-based importers (v2rayN) can parse the link.
func hysteriaHopPorts(stream map[string]any) string {
	finalmask, _ := stream["finalmask"].(map[string]any)
	quicParams, _ := finalmask["quicParams"].(map[string]any)
	udpHop, _ := quicParams["udpHop"].(map[string]any)
	ports, _ := udpHop["ports"].(string)
	return strings.TrimSpace(ports)
}

// loadNodes refreshes nodesByID from the DB. Called once per request so
// the per-inbound resolveInboundAddress lookups are pure map reads.
// We filter to address != ” so a half-configured node row doesn't
// accidentally produce a useless host like "https://:2053".
func (s *SubService) loadNodes() {
	db := database.GetDB()
	var nodes []*model.Node
	if err := db.Model(&model.Node{}).Where("address != ''").Find(&nodes).Error; err != nil {
		logger.Warning("subscription: load nodes failed:", err)
		s.nodesByID = nil
		return
	}
	m := make(map[int]*model.Node, len(nodes))
	for _, n := range nodes {
		m[n.Id] = n
	}
	s.nodesByID = m
}

// resolveInboundAddress picks the host an external client should connect to,
// honoring the inbound's share address strategy the same way the panel's
// share/QR link builder does (#5208):
//   - "listen": an explicit, client-reachable bind Listen wins, backed by the
//     node's address for node-managed inbounds;
//   - "custom": the inbound's ShareAddr wins, then node, then listen;
//   - "node" (default, and any unknown value): the node's address for
//     node-managed inbounds, then a routable Listen — the pre-strategy order.
//
// Every chain ends at the subscriber's request host (s.address). A
// loopback/wildcard bind or a unix-domain-socket listen is a server-side
// detail and is never advertised; External Proxy still overrides everything
// upstream of this call.
func (s *SubService) resolveInboundAddress(inbound *model.Inbound) string {
	var nodeAddr string
	if inbound.NodeID != nil && s.nodesByID != nil {
		if n, ok := s.nodesByID[*inbound.NodeID]; ok {
			nodeAddr = n.Address
		}
	}
	var listenAddr string
	if listen := inbound.Listen; listen != "" && listen[0] != '@' && listen[0] != '/' && isRoutableHost(listen) {
		listenAddr = listen
	}

	candidates := []string{nodeAddr, listenAddr}
	switch inbound.ShareAddrStrategy {
	case "listen":
		candidates = []string{listenAddr, nodeAddr}
	case "custom":
		candidates = []string{strings.TrimSpace(inbound.ShareAddr), nodeAddr, listenAddr}
	}
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	return s.address
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
			host := ""
			if response, ok := header["response"].(map[string]any); ok {
				if respHeaders, ok := response["headers"].(map[string]any); ok {
					host = searchHost(respHeaders)
				}
			}
			if host == "" {
				headers, _ := request["headers"].(map[string]any)
				host = searchHost(headers)
			}
			params["host"] = host
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
		applyXhttpExtraParams(xhttp, params)
	}
}

// applyXhttpExtraObj copies the bidirectional xhttp settings into the
// VMess base64 JSON link object. VMess supports arbitrary keys, so we
// flatten the SplitHTTPConfig "extra" fields directly onto obj.
func applyXhttpExtraObj(xhttp map[string]any, obj map[string]any) {
	if xpb, ok := xhttp["xPaddingBytes"].(string); ok && len(xpb) > 0 {
		obj["x_padding_bytes"] = xpb
	}
	maps.Copy(obj, buildXhttpExtra(xhttp))
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
			host := ""
			if response, ok := header["response"].(map[string]any); ok {
				if respHeaders, ok := response["headers"].(map[string]any); ok {
					host = searchHost(respHeaders)
				}
			}
			if host == "" {
				headers, _ := request["headers"].(map[string]any)
				host = searchHost(headers)
			}
			obj["host"] = host
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
		if mode, ok := xhttp["mode"].(string); ok {
			obj["mode"] = mode
		}
		applyXhttpExtraObj(xhttp, obj)
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
		if echValue, ok := searchKey(tlsSettings, "echConfigList"); ok {
			if ech, _ := echValue.(string); ech != "" {
				params["ech"] = ech
			}
		}
		if pins, ok := pinnedSha256List(tlsSettings); ok {
			params["pcs"] = strings.Join(pins, ",")
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
		if echValue, ok := searchKey(tlsSettings, "echConfigList"); ok {
			if ech, _ := echValue.(string); ech != "" {
				obj["ech"] = ech
			}
		}
		if pins, ok := pinnedSha256List(tlsSettings); ok {
			obj["pcs"] = strings.Join(pins, ",")
		}
	}
}

// pinnedSha256List extracts tlsSettings.settings.pinnedPeerCertSha256 as a
// []string. The field is panel-only (stripped before the run-config reaches
// xray-core via internal/web/service/xray.go) but flows into share links so clients
// can pin the server's certificate hash.
func pinnedSha256List(tlsClientSettings any) ([]string, bool) {
	raw, ok := searchKey(tlsClientSettings, "pinnedPeerCertSha256")
	if !ok {
		return nil, false
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// hysteriaPinHex normalises a pinnedPeerCertSha256 entry into the 64-character
// lowercase hex form that Xray-core's Hysteria2 pinSHA256 parser requires.
//
// The panel stores pins in several shapes: base64 (xray-core's native TLS
// format, used by the generate button and the JSON subscription) and hex —
// either bare or colon-separated as `openssl x509 -fingerprint -sha256` emits
// it. Hysteria2 clients hex-decode pinSHA256 and crash on a base64 value, so
// each entry is coerced to bare hex here. Anything that is neither a 32-byte
// hex nor a 32-byte base64 SHA-256 is returned unchanged so unexpected data is
// not silently dropped. Mirrors decodeCertPin in internal/web/service/node.go.
func hysteriaPinHex(pin string) string {
	pin = strings.TrimSpace(pin)
	if h := strings.ReplaceAll(pin, ":", ""); len(h) == hex.EncodedLen(sha256.Size) {
		if _, err := hex.DecodeString(h); err == nil {
			return strings.ToLower(h)
		}
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(pin); err == nil && len(b) == sha256.Size {
			return hex.EncodeToString(b)
		}
	}
	return pin
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
		if !(newSecurity == "none" && (key == "alpn" || key == "sni" || key == "fp" || key == "pcs")) {
			newObj[key] = value
		}
	}
	return newObj
}

func applyExternalProxyTLSObj(ep map[string]any, obj map[string]any, security string) {
	if security != "tls" {
		return
	}
	if sni, ok := externalProxySNI(ep); ok {
		obj["sni"] = sni
	}
	if fp, ok := ep["fingerprint"].(string); ok && fp != "" {
		obj["fp"] = fp
	}
	if alpn, ok := externalProxyALPN(ep["alpn"]); ok {
		obj["alpn"] = alpn
	}
	if pins, ok := externalProxyPins(ep["pinnedPeerCertSha256"]); ok {
		obj["pcs"] = joinAnyStrings(pins)
	}
	if ech, ok := ep["echConfigList"].(string); ok && ech != "" {
		obj["ech"] = ech
	}
}

func applyExternalProxyTLSParams(ep map[string]any, params map[string]string, security string) {
	if security != "tls" {
		return
	}
	if sni, ok := externalProxySNI(ep); ok {
		params["sni"] = sni
	}
	if fp, ok := ep["fingerprint"].(string); ok && fp != "" {
		params["fp"] = fp
	}
	if alpn, ok := externalProxyALPN(ep["alpn"]); ok {
		params["alpn"] = alpn
	}
	if pins, ok := externalProxyPins(ep["pinnedPeerCertSha256"]); ok {
		params["pcs"] = joinAnyStrings(pins)
	}
	if ech, ok := ep["echConfigList"].(string); ok && ech != "" {
		params["ech"] = ech
	}
}

// applyExternalProxyHysteriaParams overrides the cert pin for a single
// external-proxy entry on a Hysteria link. Hysteria carries the pin as a hex
// `pinSHA256` (not the `pcs` the URL-param protocols use), so each entry is
// coerced through hysteriaPinHex like the main pin. sni/fp/alpn are left as
// the inbound's own — Hysteria external proxies are typically alternate
// endpoints (port-hop / CDN) fronting the same certificate.
func applyExternalProxyHysteriaParams(ep map[string]any, params map[string]string) {
	pins, ok := externalProxyPins(ep["pinnedPeerCertSha256"])
	if !ok {
		return
	}
	hexPins := make([]string, 0, len(pins))
	for _, p := range pins {
		if s, ok := p.(string); ok {
			hexPins = append(hexPins, hysteriaPinHex(s))
		}
	}
	params["pinSHA256"] = strings.Join(hexPins, ",")
}

// cloneStreamForExternalProxy returns a shallow clone of stream with
// tlsSettings (and its nested settings map) deep-copied. The external
// proxy loop mutates tlsSettings per iteration, so without isolating
// those maps each proxy's SNI/fingerprint/ALPN would leak into the next.
func cloneStreamForExternalProxy(stream map[string]any) map[string]any {
	out := cloneMap(stream)
	ts, ok := out["tlsSettings"].(map[string]any)
	if !ok || ts == nil {
		return out
	}
	clonedTs := cloneMap(ts)
	if inner, ok := clonedTs["settings"].(map[string]any); ok && inner != nil {
		clonedTs["settings"] = cloneMap(inner)
	}
	out["tlsSettings"] = clonedTs
	return out
}

func applyExternalProxyTLSToStream(ep map[string]any, stream map[string]any, security string) {
	if security != "tls" {
		return
	}
	tlsSettings, _ := stream["tlsSettings"].(map[string]any)
	if tlsSettings == nil {
		tlsSettings = map[string]any{}
		stream["tlsSettings"] = tlsSettings
	}
	if sni, ok := externalProxySNI(ep); ok {
		tlsSettings["serverName"] = sni
	}
	if fp, ok := ep["fingerprint"].(string); ok && fp != "" {
		tlsSettings["fingerprint"] = fp
		settings, _ := tlsSettings["settings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
			tlsSettings["settings"] = settings
		}
		settings["fingerprint"] = fp
	}
	if alpn, ok := externalProxyALPNList(ep["alpn"]); ok {
		tlsSettings["alpn"] = alpn
	}
	if pins, ok := externalProxyPins(ep["pinnedPeerCertSha256"]); ok {
		settings, _ := tlsSettings["settings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
			tlsSettings["settings"] = settings
		}
		settings["pinnedPeerCertSha256"] = pins
	}
	if ech, ok := ep["echConfigList"].(string); ok && ech != "" {
		settings, _ := tlsSettings["settings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
			tlsSettings["settings"] = settings
		}
		settings["echConfigList"] = ech
	}
	if ai, ok := ep["allowInsecure"].(bool); ok && ai {
		settings, _ := tlsSettings["settings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
			tlsSettings["settings"] = settings
		}
		settings["allowInsecure"] = true
	}
}

func externalProxySNI(ep map[string]any) (string, bool) {
	if sni, ok := ep["sni"].(string); ok && sni != "" {
		return sni, true
	}
	return "", false
}

func externalProxyALPN(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, v != ""
	case []string:
		if len(v) == 0 {
			return "", false
		}
		return strings.Join(v, ","), true
	case []any:
		alpn := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				alpn = append(alpn, s)
			}
		}
		if len(alpn) == 0 {
			return "", false
		}
		return strings.Join(alpn, ","), true
	default:
		return "", false
	}
}

func externalProxyALPNList(value any) ([]any, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil, false
		}
		parts := strings.Split(v, ",")
		out := make([]any, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out, len(out) > 0
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if item != "" {
				out = append(out, item)
			}
		}
		return out, len(out) > 0
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

// externalProxyPins extracts an external-proxy entry's pinnedPeerCertSha256
// as a []any of non-empty strings. The []any element type matches what the
// JSON/Clash sub builders expect when reading the value back off the cloned
// stream's tlsSettings.settings.
func externalProxyPins(value any) ([]any, bool) {
	switch v := value.(type) {
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if item != "" {
				out = append(out, item)
			}
		}
		return out, len(out) > 0
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func joinAnyStrings(items []any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ",")
}

// buildVmessExternalProxyLinks is a thin adapter: it maps the legacy
// externalProxy entries to []ShareEndpoint and renders them through the unified
// endpoint path. Kept so genVmessLink's call site is unchanged.
func (s *SubService) buildVmessExternalProxyLinks(externalProxies []any, baseObj map[string]any, inbound *model.Inbound, email string) string {
	eps := make([]ShareEndpoint, 0, len(externalProxies))
	for _, externalProxy := range externalProxies {
		ep, _ := externalProxy.(map[string]any)
		eps = append(eps, externalProxyToEndpoint(ep))
	}
	return s.buildEndpointVmessLinks(eps, baseObj, inbound, email)
}

// buildLinkWithParams appends ?query and #fragment to a pre-built
// scheme://userinfo@host:port string without re-parsing it. The caller
// has already escaped userinfo via encodeUserinfo (or chosen a base64
// alphabet with no reserved chars); a url.Parse + .String() round-trip
// would silently decode that escaping because Go's userinfo emitter
// leaves sub-delims (=, +, ;) literal, which breaks Trojan/Hysteria/SS
// clients that reject those chars in the password.
func buildLinkWithParams(link string, params map[string]string, fragment string) string {
	return appendQueryAndFragment(link, params, fragment, "", false)
}

// buildLinkWithParamsAndSecurity is buildLinkWithParams plus an
// external-proxy override: the `security` key in params is replaced with
// the supplied value, and TLS hint fields (alpn/sni/fp/pcs) are stripped
// when the override is `none`.
func buildLinkWithParamsAndSecurity(link string, params map[string]string, fragment, security string, omitTLSFields bool) string {
	return appendQueryAndFragment(link, params, fragment, security, omitTLSFields)
}

func appendQueryAndFragment(link string, params map[string]string, fragment, securityOverride string, omitTLSFields bool) string {
	var sb strings.Builder
	sb.WriteString(link)

	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			if securityOverride != "" && k == "security" {
				v = securityOverride
			}
			if omitTLSFields && (k == "alpn" || k == "sni" || k == "fp" || k == "pcs") {
				continue
			}
			q.Set(k, v)
		}
		encoded := q.Encode()
		if encoded != "" {
			if strings.Contains(link, "?") {
				sb.WriteByte('&')
			} else {
				sb.WriteByte('?')
			}
			sb.WriteString(encoded)
		}
	}

	if fragment != "" {
		sb.WriteByte('#')
		// Match the frontend's encodeURIComponent(remark): spaces become
		// %20 (not + as in query strings).
		sb.WriteString(strings.ReplaceAll(url.QueryEscape(fragment), "+", "%20"))
	}
	return sb.String()
}

// buildExternalProxyURLLinks is a thin adapter: it maps the legacy externalProxy
// entries to []ShareEndpoint and renders them through the unified endpoint path.
// Kept so the genVless/genTrojan/genShadowsocks call sites are unchanged.
func (s *SubService) buildExternalProxyURLLinks(
	externalProxies []any,
	params map[string]string,
	baseSecurity string,
	makeLink func(dest string, port int) string,
	makeRemark func(ep map[string]any) string,
) string {
	eps := make([]ShareEndpoint, 0, len(externalProxies))
	for _, externalProxy := range externalProxies {
		ep, _ := externalProxy.(map[string]any)
		eps = append(eps, externalProxyToEndpoint(ep))
	}
	return s.buildEndpointLinks(eps, params, baseSecurity, makeLink, func(e ShareEndpoint) string {
		return makeRemark(e.ep)
	})
}

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}

// genRemark builds the remark for a non-host link (raw default / legacy
// externalProxy / synthetic JSON-Clash entry). In the subscription body a set
// remark template takes over; otherwise (and in every display context) the
// remark is just the config name (inbound remark, then extra).
func (s *SubService) genRemark(inbound *model.Inbound, email string, extra string) string {
	if s.remarkTemplate != "" && s.subscriptionBody {
		return s.genTemplatedRemark(inbound, s.lookupClient(inbound, email), extra)
	}
	// Sub info page + panel link/QR displays: just the config name (no template,
	// so no per-client email/usage leaks into the shown remark).
	return fallbackRemark(inbound.Remark, extra)
}

// fallbackRemark is the minimal remark used only when no template is configured
// (an operator explicitly cleared it): the inbound remark and the host/extra
// remark joined by "-", skipping empties. The configurable remark model was
// removed in favour of the template, whose default already includes the email.
func fallbackRemark(inboundRemark, extra string) string {
	switch {
	case inboundRemark == "":
		return extra
	case extra == "":
		return inboundRemark
	default:
		return inboundRemark + "-" + extra
	}
}

// findClientStats returns the inbound's traffic record for email, if present.
func (s *SubService) findClientStats(inbound *model.Inbound, email string) (xray.ClientTraffic, bool) {
	for _, clientStat := range inbound.ClientStats {
		if clientStat.Email == email {
			return clientStat, true
		}
	}
	return xray.ClientTraffic{}, false
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

// buildXhttpExtra walks an xhttpSettings map and returns the JSON blob
// that goes into the URL's `extra` param (or, for VMess, the link
// object). Carries ONLY the bidirectional fields from xray-core's
// SplitHTTPConfig — i.e. the ones the server enforces and the client
// must match. Strictly one-sided fields are excluded:
//
//   - server-only (noSSEHeader, scMaxBufferedPosts, scStreamUpServerSecs,
//     serverMaxHeaderBytes) — client wouldn't read them, so emitting
//     them just bloats the URL.
//   - client-only values are included only when present in the inbound
//     JSON. Some deployments/imported configs carry them there, and the
//     subscription link is the only place clients can receive them.
//
// Truthy-only guards keep default inbounds emitting the same compact URL
// they did before this helper grew.
func buildXhttpExtra(xhttp map[string]any) map[string]any {
	if xhttp == nil {
		return nil
	}
	extra := map[string]any{}

	if mode, ok := xhttp["mode"].(string); ok && len(mode) > 0 {
		extra["mode"] = mode
	}

	if xpb, ok := xhttp["xPaddingBytes"].(string); ok && len(xpb) > 0 {
		extra["xPaddingBytes"] = xpb
	}
	if obfs, ok := xhttp["xPaddingObfsMode"].(bool); ok && obfs {
		extra["xPaddingObfsMode"] = true
		for _, field := range []string{"xPaddingKey", "xPaddingHeader", "xPaddingPlacement", "xPaddingMethod"} {
			if v, ok := xhttp[field].(string); ok && len(v) > 0 {
				extra[field] = v
			}
		}
	}

	stringFields := []string{
		"uplinkHTTPMethod",
		"sessionPlacement", "sessionKey",
		"seqPlacement", "seqKey",
		"uplinkDataPlacement", "uplinkDataKey",
		"scMaxEachPostBytes", "scMinPostsIntervalMs",
	}
	// Values matching xray-core's own defaults are redundant on the wire and
	// the literal scMinPostsIntervalMs=30 is a known DPI fingerprint (#5141).
	// Old panels seeded these defaults into every xhttp inbound, so filter
	// them here instead of requiring every stored config to be re-saved.
	coreDefaults := map[string]string{
		"scMaxEachPostBytes":   "1000000",
		"scMinPostsIntervalMs": "30",
	}
	for _, field := range stringFields {
		if v, ok := xhttp[field].(string); ok && len(v) > 0 && v != coreDefaults[field] {
			extra[field] = v
		}
	}

	for _, field := range []string{"uplinkChunkSize"} {
		if v, ok := nonZeroShareValue(xhttp[field]); ok {
			extra[field] = v
		}
	}

	for _, field := range []string{"noGRPCHeader"} {
		if v, ok := xhttp[field].(bool); ok && v {
			extra[field] = v
		}
	}

	for _, field := range []string{"xmux", "downloadSettings"} {
		if v, ok := nonEmptyShareObject(xhttp[field]); ok {
			extra[field] = v
		}
	}

	// Headers — emitted as the {name: value} map upstream's struct
	// expects. The server runtime ignores this field, but the client
	// (consuming the share link) honors it. Drop any "host" entry —
	// host already wins as a top-level URL param.
	if rawHeaders, ok := xhttp["headers"].(map[string]any); ok && len(rawHeaders) > 0 {
		out := map[string]any{}
		for k, v := range rawHeaders {
			if strings.EqualFold(k, "host") {
				continue
			}
			out[k] = v
		}
		if len(out) > 0 {
			extra["headers"] = out
		}
	}

	if len(extra) == 0 {
		return nil
	}
	return extra
}

func nonZeroShareValue(v any) (any, bool) {
	switch value := v.(type) {
	case string:
		return value, value != ""
	case int:
		return value, value != 0
	case int32:
		return value, value != 0
	case int64:
		return value, value != 0
	case float32:
		return value, value != 0
	case float64:
		return value, value != 0
	default:
		return nil, false
	}
}

func nonEmptyShareObject(v any) (any, bool) {
	switch value := v.(type) {
	case map[string]any:
		return value, len(value) > 0
	case map[string]string:
		return value, len(value) > 0
	case []any:
		return value, len(value) > 0
	default:
		return nil, false
	}
}

// applyXhttpExtraParams emits the full xhttp config into the URL query
// params of a vless:// / trojan:// / ss:// link. Sets path/host/mode at
// top level (xray's Build() always lets these win over `extra`) and packs
// everything else into a JSON `extra` param. Also writes the flat
// `x_padding_bytes` param sing-box-family clients understand.
//
// Without this, the admin's custom xPaddingBytes / sessionKey / etc. never
// reach the client and handshakes are silently rejected with
// `invalid padding (...) length: 0` — the client-visible symptom is
// "xhttp doesn't connect" on OpenWRT / sing-box.
//
// Two encodings are written so every popular client can read at least one:
//
//   - x_padding_bytes=<range>  — flat param, understood by sing-box and its
//     derivatives (Podkop, OpenWRT sing-box, Karing, NekoBox, …).
//   - extra=<url-encoded-json> — full xhttp settings blob, which is how
//     xray-core clients (v2rayNG, Happ, Furious, Exclave, …) pick up the
//     bidirectional fields beyond path/host/mode.
func applyXhttpExtraParams(xhttp map[string]any, params map[string]string) {
	if xhttp == nil {
		return
	}
	applyPathAndHostParams(xhttp, params)
	if mode, ok := xhttp["mode"].(string); ok {
		params["mode"] = mode
	}

	if xpb, ok := xhttp["xPaddingBytes"].(string); ok && len(xpb) > 0 {
		params["x_padding_bytes"] = xpb
	}

	extra := buildXhttpExtra(xhttp)
	if extra != nil {
		if b, err := json.Marshal(extra); err == nil {
			params["extra"] = string(b)
		}
	}
}

var kcpMaskToHeaderType = map[string]string{
	"dns":       "dns",
	"dtls":      "dtls",
	"srtp":      "srtp",
	"utp":       "utp",
	"wechat":    "wechat-video",
	"wireguard": "wireguard",
}

var validFinalMaskUDPTypes = map[string]struct{}{
	"salamander":    {},
	"mkcp-legacy":   {},
	"xdns":          {},
	"xicmp":         {},
	"noise":         {},
	"header-custom": {},
	"realm":         {},
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
		if maskType, _ := mask["type"].(string); maskType != "mkcp-legacy" {
			continue
		}

		settings, _ := mask["settings"].(map[string]any)
		header, _ := settings["header"].(string)
		value, _ := settings["value"].(string)
		if header == "" {
			fields.seed = value
			continue
		}
		if mapped, ok := kcpMaskToHeaderType[header]; ok {
			fields.headerType = mapped
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
	Host          string
	BasePath      string
	SId           string
	Enabled       bool
	Download      string
	Upload        string
	Total         string
	Used          string
	Remained      string
	Expire        int64
	LastOnline    int64
	Datepicker    string
	DownloadByte  int64
	UploadByte    int64
	TotalByte     int64
	SubUrl        string
	SubJsonUrl    string
	SubClashUrl   string
	SubTitle      string
	SubSupportUrl string
	Result        []string
	Emails        []string
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
func (s *SubService) BuildURLs(subPath, subJsonPath, subClashPath, subId string) (subURL, subJsonURL, subClashURL string) {
	if subId == "" {
		return "", "", ""
	}

	configuredSubURI, _ := s.settingService.GetSubURI()
	configuredSubJsonURI, _ := s.settingService.GetSubJsonURI()
	configuredSubClashURI, _ := s.settingService.GetSubClashURI()

	// Same base as the panel's Client Information page; s.address is the
	// subscriber's host already normalized away from any loopback/bind IP.
	base := s.settingService.BuildSubURIBase(s.address)

	subURL = s.buildSingleURL(configuredSubURI, base, subPath, subId)

	// When subURI is explicitly configured (reverse-proxy setup), use its
	// scheme+host as the base for JSON and Clash URLs so they match the
	// reverse-proxy endpoint instead of the raw sub-server port. Fall back
	// to the request-derived base if subURI is empty or can't be parsed
	// into a scheme+host (e.g. a malformed value with no scheme).
	jsonClashBase := base
	if configuredSubURI != "" {
		if derived := s.extractBaseFromURI(configuredSubURI); derived != "" {
			jsonClashBase = derived
		}
	}

	subJsonURL = s.buildSingleURL(configuredSubJsonURI, jsonClashBase, subJsonPath, subId)
	subClashURL = s.buildSingleURL(configuredSubClashURI, jsonClashBase, subClashPath, subId)

	return subURL, subJsonURL, subClashURL
}

// extractBaseFromURI extracts scheme://host from a configured URI.
// e.g., "https://example.com/sub-xxx/" → "https://example.com".
// Returns "" when the URI is empty or lacks a scheme/host, so callers can
// fall back to the request-derived base instead of emitting a broken value.
func (s *SubService) extractBaseFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}

// buildSingleURL constructs a single URL using configured URI or base components
func (s *SubService) buildSingleURL(configuredURI, base, basePath, subId string) string {
	if configuredURI != "" {
		return s.joinPathWithID(configuredURI, subId)
	}
	return s.joinPathWithID(base+basePath, subId)
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
func (s *SubService) BuildPageData(subId string, hostHeader string, traffic xray.ClientTraffic, lastOnline int64, subs []string, emails []string, subURL, subJsonURL, subClashURL string, basePath string, subTitle string, subSupportUrl string) PageData {
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
		Host:          hostHeader,
		BasePath:      basePath,
		SId:           subId,
		Enabled:       traffic.Enable,
		Download:      download,
		Upload:        upload,
		Total:         total,
		Used:          used,
		Remained:      remained,
		Expire:        traffic.ExpiryTime / 1000,
		LastOnline:    lastOnline,
		Datepicker:    datepicker,
		DownloadByte:  traffic.Down,
		UploadByte:    traffic.Up,
		TotalByte:     traffic.Total,
		SubUrl:        subURL,
		SubJsonUrl:    subJsonURL,
		SubClashUrl:   subClashURL,
		SubTitle:      subTitle,
		SubSupportUrl: subSupportUrl,
		Result:        subs,
		Emails:        emails,
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
