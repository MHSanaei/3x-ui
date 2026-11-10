package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	exitPoolDir            = "/var/lib/xnvgatebox/fanout"
	exitPoolClientPrefix   = "xnv3-"
	exitPoolClientDomain   = "@exit.local"
	exitPoolOutboundPrefix = "xnv3-exit-"
	exitPoolGroupPrefix    = "Exit Pool / "
	exitPoolSourceStable   = "xnvgatebox_stable"
	exitPoolSourceHome     = "new_residential"
	exitPoolSourceNativeDC = "new_native_datacenter"
)

var exitPoolSubIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,40}$`)

const exitPoolMaxProxyBody = 8 << 20

type ExitPoolService struct {
	InboundService     InboundService
	ClientService      ClientService
	SettingService     SettingService
	XraySettingService XraySettingService
	XrayService        XrayService
}

type ExitPoolExit struct {
	Slot      int     `json:"slot"`
	Port      int     `json:"port"`
	Host      string  `json:"host"`
	Region    string  `json:"region"`
	Country   string  `json:"country"`
	ExitIP    string  `json:"exitIp"`
	Status    string  `json:"status"`
	LatencyMS float64 `json:"latencyMs,omitempty"`
	// SourceCategory is an isolation boundary, not a presentation-only label.
	// Existing xnvgatebox slots default to the stable category. Additional
	// providers must explicitly identify themselves and pass the category's
	// evidence checks before they are exposed to the panel.
	SourceCategory string `json:"sourceCategory"`
	SourceName     string `json:"sourceName,omitempty"`
	IPType         string `json:"ipType,omitempty"`
	Native         bool   `json:"native,omitempty"`
	Stable         bool   `json:"stable"`
	// ProxyAddress/ProxyPort are present only for verified supplemental exits.
	// Stable xnvgatebox exits always use their loopback listener in Port.
	ProxyAddress string `json:"-"`
	ProxyPort    int    `json:"-"`
	SocksUser    string `json:"-"`
	SocksPass    string `json:"-"`
}

type ExitPoolInbound struct {
	ID       int            `json:"id"`
	Tag      string         `json:"tag"`
	Remark   string         `json:"remark"`
	Port     int            `json:"port"`
	Protocol model.Protocol `json:"protocol"`
}

type ExitPoolMember struct {
	Slot           int    `json:"slot"`
	Region         string `json:"region"`
	Country        string `json:"country"`
	ExitIP         string `json:"exitIp"`
	SourceCategory string `json:"sourceCategory"`
	Email          string `json:"email"`
}

type ExitPoolGroup struct {
	SubID       string           `json:"subId"`
	Name        string           `json:"name"`
	SubURL      string           `json:"subUrl"`
	InboundID   int              `json:"inboundId"`
	InboundTag  string           `json:"inboundTag"`
	Members     []ExitPoolMember `json:"members"`
	MemberCount int              `json:"memberCount"`
}

type ExitPoolState struct {
	Installed bool              `json:"installed"`
	Error     string            `json:"error,omitempty"`
	Exits     []ExitPoolExit    `json:"exits"`
	Inbounds  []ExitPoolInbound `json:"inbounds"`
	Groups    []ExitPoolGroup   `json:"groups"`
}

type ExitPoolSyncRequest struct {
	InboundID int    `json:"inboundId"`
	SubID     string `json:"subId"`
	Name      string `json:"name"`
	Slots     []int  `json:"slots"`
}

func newExitPoolSubID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pool-" + hex.EncodeToString(b), nil
}

func exitPoolEmail(subID string, exit ExitPoolExit) string {
	region := strings.ToLower(exit.Region)
	region = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(region, "-")
	region = strings.Trim(region, "-")
	if region == "" {
		region = "region"
	}
	return exitPoolClientPrefix + strings.ToLower(subID) + "-" + region + "-s" + strconv.Itoa(exit.Slot) + exitPoolClientDomain
}

func exitPoolOutboundTag(slot int) string {
	return exitPoolOutboundPrefix + strconv.Itoa(slot)
}

func exitPoolComment(e ExitPoolExit) string {
	return fmt.Sprintf("xnv3:%d:%s:%s:%s:%s", e.Slot, e.Region, e.Country, e.ExitIP, e.SourceCategory)
}

func parseExitPoolComment(value string) (ExitPoolMember, bool) {
	p := strings.SplitN(value, ":", 6)
	if (len(p) != 5 && len(p) != 6) || p[0] != "xnv3" {
		return ExitPoolMember{}, false
	}
	slot, err := strconv.Atoi(p[1])
	if err != nil || slot < 1 {
		return ExitPoolMember{}, false
	}
	source := exitPoolSourceStable
	if len(p) == 6 {
		source = p[5]
	}
	if !validExitPoolSource(source) {
		return ExitPoolMember{}, false
	}
	return ExitPoolMember{Slot: slot, Region: p[2], Country: p[3], ExitIP: p[4], SourceCategory: source}, true
}

func validExitPoolSource(source string) bool {
	switch source {
	case exitPoolSourceStable, exitPoolSourceHome, exitPoolSourceNativeDC:
		return true
	default:
		return false
	}
}

func normalizeExitPoolExit(e ExitPoolExit) (ExitPoolExit, error) {
	if e.SourceCategory == "" {
		e.SourceCategory = exitPoolSourceStable
	}
	if !validExitPoolSource(e.SourceCategory) {
		return ExitPoolExit{}, fmt.Errorf("unrecognized exit-pool source category %q", e.SourceCategory)
	}
	e.Stable = e.SourceCategory == exitPoolSourceStable
	switch e.SourceCategory {
	case exitPoolSourceStable:
		if e.ProxyAddress != "" || e.ProxyPort != 0 || e.Port < 1 || e.Port > 65535 {
			return ExitPoolExit{}, fmt.Errorf("stable exit slot %d has an invalid listener", e.Slot)
		}
	case exitPoolSourceHome:
		if e.IPType != "strict_residential" && e.IPType != "likely_residential" {
			return ExitPoolExit{}, fmt.Errorf("new residential exit slot %d lacks residential evidence", e.Slot)
		}
	case exitPoolSourceNativeDC:
		if !e.Native || (e.IPType != "datacenter" && e.IPType != "business_isp") {
			return ExitPoolExit{}, fmt.Errorf("new native datacenter exit slot %d lacks native datacenter evidence", e.Slot)
		}
	}
	if e.SourceCategory != exitPoolSourceStable {
		if e.Slot < 100000 || e.Slot > 999999 || e.Port != 0 || e.ProxyPort < 1 || e.ProxyPort > 65535 {
			return ExitPoolExit{}, fmt.Errorf("supplemental exit slot %d has an invalid reserved slot or listener", e.Slot)
		}
		ip := net.ParseIP(strings.TrimSpace(e.ProxyAddress))
		if ip == nil || ip.To4() == nil || !exitPoolPublicIPv4(ip.To4()) {
			return ExitPoolExit{}, fmt.Errorf("supplemental exit slot %d requires a public literal IPv4 proxy", e.Slot)
		}
		e.ProxyAddress = ip.To4().String()
	}
	if (e.SocksUser == "") != (e.SocksPass == "") {
		return ExitPoolExit{}, fmt.Errorf("exit slot %d has incomplete SOCKS credentials", e.Slot)
	}
	return e, nil
}

func exitPoolPublicIPv4(ip net.IP) bool {
	if len(ip) != net.IPv4len || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	switch {
	case ip[0] == 0, ip[0] == 127, ip[0] >= 224:
		return false
	case ip[0] == 100 && ip[1]&0xc0 == 64:
		return false
	case ip[0] == 169 && ip[1] == 254:
		return false
	case ip[0] == 192 && (ip[1] == 0 || ip[1] == 2):
		return false
	case ip[0] == 198 && (ip[1] == 18 || ip[1] == 19 || ip[1] == 51):
		return false
	case ip[0] == 203 && ip[1] == 0:
		return false
	default:
		return true
	}
}

func exitPoolSOCKSServer(e ExitPoolExit) map[string]any {
	address, port := "127.0.0.1", e.Port
	if e.SourceCategory != exitPoolSourceStable {
		address, port = e.ProxyAddress, e.ProxyPort
	}
	server := map[string]any{"address": address, "port": port}
	if e.SocksUser != "" {
		server["users"] = []any{map[string]any{"user": e.SocksUser, "pass": e.SocksPass}}
	}
	return server
}

func exitPoolRoutingRule(inboundTag, email, outboundTag string) map[string]any {
	// Both address families and UDP (including client DNS packets) are forced
	// through the selected SOCKS outbound. Xray has no alternate outbound in
	// this rule, so an unavailable/UDP-incapable proxy fails closed.
	return map[string]any{"type": "field", "network": "tcp,udp", "inboundTag": []any{inboundTag}, "user": []any{email}, "outboundTag": outboundTag}
}

// ExitPoolClientDisplayName returns the stable, region-aware name used by
// subscription renderers. It intentionally overrides a global remark template:
// otherwise every identity on the same mother inbound can receive one identical
// node name in clients such as v2rayN.
func ExitPoolClientDisplayName(client model.Client) (string, bool) {
	if !strings.HasPrefix(client.Group, exitPoolGroupPrefix) {
		return "", false
	}
	member, ok := parseExitPoolComment(client.Comment)
	if !ok {
		return "", false
	}
	name := strings.TrimSpace(strings.TrimPrefix(client.Group, exitPoolGroupPrefix))
	region := strings.TrimSpace(member.Region)
	if region == "" {
		region = strings.TrimSpace(member.Country)
	}
	if name == "" || region == "" {
		return "", false
	}
	return fmt.Sprintf("%s-%s-S%d", name, region, member.Slot), true
}

func readExitPoolFile(name string, limit int64) ([]byte, error) {
	path := filepath.Join(exitPoolDir, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid exit-pool %s", name)
	}
	return os.ReadFile(path)
}

func validateExitPoolListenAddr(value string) error {
	if strings.TrimSpace(value) != "127.0.0.1" {
		return errors.New("xnvgatebox management listener is not loopback-only; run x-ui exit-pool secure")
	}
	return nil
}

func exitPoolConsoleOperationAllowed(method, requestPath string) bool {
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return true
	}
	cleanPath := pathpkg.Clean("/" + strings.TrimLeft(requestPath, "/"))
	return cleanPath != "/api/settings" && cleanPath != "/api/update/apply"
}

func newExitPoolHTTPClient(ctx context.Context, timeout time.Duration) (*http.Client, string, error) {
	settingsRaw, err := readExitPoolFile("settings.json", 64<<10)
	if err != nil {
		return nil, "", err
	}
	var settings struct {
		Port       int    `json:"port"`
		ListenAddr string `json:"listen_addr"`
	}
	if json.Unmarshal(settingsRaw, &settings) != nil || settings.Port < 1 || settings.Port > 65535 {
		return nil, "", errors.New("invalid xnvgatebox settings")
	}
	if err := validateExitPoolListenAddr(settings.ListenAddr); err != nil {
		return nil, "", err
	}
	baseRaw, err := readExitPoolFile("basepath", 4096)
	if err != nil {
		return nil, "", err
	}
	passwordRaw, err := readExitPoolFile("password", 4096)
	if err != nil {
		return nil, "", err
	}
	base := strings.Trim(strings.TrimSpace(string(baseRaw)), "/")
	password := strings.TrimSpace(string(passwordRaw))
	if base == "" || strings.Contains(base, "/") || password == "" {
		return nil, "", errors.New("invalid xnvgatebox access files")
	}

	root := fmt.Sprintf("http://127.0.0.1:%d/%s", settings.Port, base)
	expectedHost := fmt.Sprintf("127.0.0.1:%d", settings.Port)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: timeout,
		Jar:     jar,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "http" || req.URL.Host != expectedHost || len(via) > 5 {
				return errors.New("refused xnvgatebox redirect outside loopback service")
			}
			return nil
		},
	}
	form := url.Values{"password": {password}}
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", err
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(loginReq)
	if err != nil {
		return nil, "", fmt.Errorf("connect to xnvgatebox: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		return nil, "", fmt.Errorf("xnvgatebox login returned HTTP %d", resp.StatusCode)
	}
	return client, root, nil
}

func (s *ExitPoolService) loadExits() ([]ExitPoolExit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client, root, err := newExitPoolHTTPClient(ctx, 8*time.Second)
	if err != nil {
		return nil, err
	}
	exitsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/exits", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(exitsReq)
	if err != nil {
		return nil, fmt.Errorf("read xnvgatebox exits: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xnvgatebox exits returned HTTP %d", resp.StatusCode)
	}
	var view struct {
		Exits []struct {
			Slot           int     `json:"slot"`
			Port           int     `json:"port"`
			Host           string  `json:"host"`
			Region         string  `json:"region"`
			Country        string  `json:"country"`
			ExitIP         string  `json:"exit_ip"`
			Status         string  `json:"status"`
			LatencyMS      float64 `json:"latency_ms"`
			SourceCategory string  `json:"source_category"`
			SourceName     string  `json:"source_name"`
			Native         bool    `json:"native"`
			ProxyAddress   string  `json:"proxy_address"`
			ProxyPort      int     `json:"proxy_port"`
			Egress         *struct {
				IPType string `json:"ip_type"`
			} `json:"egress"`
			SocksUser string `json:"socks_user"`
			SocksPass string `json:"socks_pass"`
		} `json:"exits"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&view); err != nil {
		return nil, fmt.Errorf("decode xnvgatebox exits: %w", err)
	}
	exits := make([]ExitPoolExit, 0, len(view.Exits))
	for _, e := range view.Exits {
		ipType := ""
		if e.Egress != nil {
			ipType = e.Egress.IPType
		}
		exit, err := normalizeExitPoolExit(ExitPoolExit{
			Slot: e.Slot, Port: e.Port, Host: e.Host, Region: e.Region,
			Country: e.Country, ExitIP: e.ExitIP, Status: e.Status,
			LatencyMS: e.LatencyMS, SourceCategory: e.SourceCategory,
			SourceName: e.SourceName, IPType: ipType, Native: e.Native,
			ProxyAddress: e.ProxyAddress, ProxyPort: e.ProxyPort,
			SocksUser: e.SocksUser, SocksPass: e.SocksPass,
		})
		if err != nil {
			return nil, err
		}
		exits = append(exits, exit)
	}
	return exits, nil
}

// ProxyConsole exposes xnvgatebox only through the authenticated 3x-ui panel.
// The upstream listener is expected to be loopback-only; its password and
// session cookie are never sent to the browser.
func (s *ExitPoolService) ProxyConsole(w http.ResponseWriter, r *http.Request, subPath, csrfToken string) error {
	cleanPath := pathpkg.Clean("/" + strings.TrimLeft(subPath, "/"))
	if !exitPoolConsoleOperationAllowed(r.Method, cleanPath) {
		http.Error(w, "This operation is locked by 3x-ui security policy", http.StatusForbidden)
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	client, root, err := newExitPoolHTTPClient(ctx, 2*time.Minute)
	if err != nil {
		return err
	}
	upstreamURL, err := url.Parse(root + cleanPath)
	if err != nil {
		return err
	}
	upstreamURL.RawQuery = r.URL.RawQuery
	requestBody := http.MaxBytesReader(w, r.Body, exitPoolMaxProxyBody)
	request, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL.String(), requestBody)
	if err != nil {
		return err
	}
	for _, name := range []string{"Accept", "Accept-Language", "Content-Type", "Range"} {
		if value := r.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("xnvgatebox console request failed: %w", err)
	}
	defer response.Body.Close()
	for _, name := range []string{"Cache-Control", "Content-Disposition", "Content-Length", "Content-Range", "Content-Type"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	contentType := response.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		blob, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if readErr != nil {
			return readErr
		}
		tokenJSON, _ := json.Marshal(csrfToken)
		bridge := `<script>(()=>{const t=` + string(tokenJSON) + `,f=window.fetch.bind(window);window.fetch=(i,o={})=>{const m=(o.method||"GET").toUpperCase();if(!["GET","HEAD","OPTIONS","TRACE"].includes(m)){const h=new Headers(o.headers||{});h.set("X-CSRF-Token",t);o={...o,headers:h}}return f(i,o)}})();</script>`
		blob = []byte(strings.Replace(string(blob), "<head>", "<head>"+bridge, 1))
		w.Header().Del("Content-Length")
		w.WriteHeader(response.StatusCode)
		_, err = w.Write(blob)
		return err
	}
	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, io.LimitReader(response.Body, 64<<20))
	return err
}

func (s *ExitPoolService) compatibleInbounds(userID int) ([]ExitPoolInbound, error) {
	rows, err := s.InboundService.GetInbounds(userID)
	if err != nil {
		return nil, err
	}
	out := make([]ExitPoolInbound, 0, len(rows))
	for _, ib := range rows {
		if !ib.Enable || ib.NodeID != nil {
			continue
		}
		if ib.Protocol != model.VLESS && ib.Protocol != model.VMESS && ib.Protocol != model.Trojan {
			continue
		}
		out = append(out, ExitPoolInbound{ID: ib.Id, Tag: ib.Tag, Remark: ib.Remark, Port: ib.Port, Protocol: ib.Protocol})
	}
	return out, nil
}

func (s *ExitPoolService) groups(host string) ([]ExitPoolGroup, error) {
	rows, err := s.ClientService.List()
	if err != nil {
		return nil, err
	}
	bySub := map[string]*ExitPoolGroup{}
	for _, row := range rows {
		if !strings.HasPrefix(row.Email, exitPoolClientPrefix) || !strings.HasSuffix(row.Email, exitPoolClientDomain) {
			continue
		}
		member, ok := parseExitPoolComment(row.Comment)
		if !ok || row.SubID == "" || len(row.InboundIds) != 1 {
			continue
		}
		g := bySub[row.SubID]
		if g == nil {
			g = &ExitPoolGroup{SubID: row.SubID, Name: strings.TrimPrefix(row.Group, "Exit Pool / "), InboundID: row.InboundIds[0]}
			bySub[row.SubID] = g
		}
		member.Email = row.Email
		g.Members = append(g.Members, member)
	}
	result := make([]ExitPoolGroup, 0, len(bySub))
	subEnabled, _ := s.SettingService.GetSubEnable()
	subBase, _ := s.SettingService.GetSubURI()
	if subBase == "" {
		subBase = s.SettingService.BuildSubURIBase(host)
		subPath, _ := s.SettingService.GetSubPath()
		subBase += subPath
	}
	for _, g := range bySub {
		if ib, err := s.InboundService.GetInbound(g.InboundID); err == nil {
			g.InboundTag = ib.Tag
		}
		slices.SortFunc(g.Members, func(a, b ExitPoolMember) int { return a.Slot - b.Slot })
		g.MemberCount = len(g.Members)
		if subEnabled && subBase != "" {
			g.SubURL = strings.TrimRight(subBase, "/") + "/" + g.SubID
		}
		result = append(result, *g)
	}
	slices.SortFunc(result, func(a, b ExitPoolGroup) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

func (s *ExitPoolService) State(userID int, host string) (ExitPoolState, error) {
	state := ExitPoolState{Installed: true, Exits: []ExitPoolExit{}, Inbounds: []ExitPoolInbound{}, Groups: []ExitPoolGroup{}}
	if _, err := os.Stat(exitPoolDir); err != nil {
		if os.IsNotExist(err) {
			state.Installed = false
			return state, nil
		}
		return state, err
	}
	var err error
	state.Inbounds, err = s.compatibleInbounds(userID)
	if err != nil {
		return state, err
	}
	state.Groups, err = s.groups(host)
	if err != nil {
		return state, err
	}
	state.Exits, err = s.loadExits()
	if err != nil {
		state.Error = err.Error()
		return state, nil
	}
	for i := range state.Exits {
		state.Exits[i].SocksUser = ""
		state.Exits[i].SocksPass = ""
	}
	return state, nil
}

func stringSlice(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if value, ok := item.(string); ok {
				out = append(out, value)
			}
		}
		return out
	case []string:
		return x
	default:
		return nil
	}
}

func isManagedExitPoolRule(rule map[string]any) bool {
	tag, _ := rule["outboundTag"].(string)
	if !strings.HasPrefix(tag, exitPoolOutboundPrefix) {
		return false
	}
	for _, user := range stringSlice(rule["user"]) {
		if strings.HasPrefix(user, exitPoolClientPrefix) && strings.HasSuffix(user, exitPoolClientDomain) {
			return true
		}
	}
	return false
}

func (s *ExitPoolService) Sync(userID int, host string, req ExitPoolSyncRequest) (ExitPoolGroup, error) {
	if req.InboundID < 1 || len(req.Slots) == 0 {
		return ExitPoolGroup{}, errors.New("choose one inbound and at least one exit")
	}
	if req.SubID == "" {
		var err error
		req.SubID, err = newExitPoolSubID()
		if err != nil {
			return ExitPoolGroup{}, err
		}
	}
	if !exitPoolSubIDPattern.MatchString(req.SubID) {
		return ExitPoolGroup{}, errors.New("subId must be 6-40 letters, numbers, underscore or dash")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len([]rune(req.Name)) > 64 {
		return ExitPoolGroup{}, errors.New("group name is required and must not exceed 64 characters")
	}
	inbound, err := s.InboundService.GetInbound(req.InboundID)
	if err != nil {
		return ExitPoolGroup{}, err
	}
	if inbound.UserId != userID || !inbound.Enable || inbound.NodeID != nil || (inbound.Protocol != model.VLESS && inbound.Protocol != model.VMESS && inbound.Protocol != model.Trojan) {
		return ExitPoolGroup{}, errors.New("only enabled local VLESS, VMess or Trojan inbounds are supported")
	}
	exits, err := s.loadExits()
	if err != nil {
		return ExitPoolGroup{}, err
	}
	requested := map[int]bool{}
	for _, slot := range req.Slots {
		requested[slot] = true
	}
	desired := make([]ExitPoolExit, 0, len(requested))
	for _, e := range exits {
		if requested[e.Slot] {
			if e.Status != "up" {
				return ExitPoolGroup{}, fmt.Errorf("exit slot %d is not ready", e.Slot)
			}
			desired = append(desired, e)
			delete(requested, e.Slot)
		}
	}
	if len(requested) != 0 {
		return ExitPoolGroup{}, errors.New("one or more selected exits no longer exist")
	}
	slices.SortFunc(desired, func(a, b ExitPoolExit) int { return a.Slot - b.Slot })

	clients, err := s.ClientService.List()
	if err != nil {
		return ExitPoolGroup{}, err
	}
	existing := map[int]ClientWithAttachments{}
	managedEmails := map[string]bool{}
	for _, row := range clients {
		if row.SubID != req.SubID {
			continue
		}
		if !strings.HasPrefix(row.Email, exitPoolClientPrefix) || !strings.HasSuffix(row.Email, exitPoolClientDomain) {
			return ExitPoolGroup{}, errors.New("subId already belongs to a regular client")
		}
		member, ok := parseExitPoolComment(row.Comment)
		if !ok {
			return ExitPoolGroup{}, errors.New("subId already belongs to an invalid exit-pool client")
		}
		if len(row.InboundIds) != 1 || row.InboundIds[0] != req.InboundID {
			return ExitPoolGroup{}, errors.New("this exit-pool group belongs to a different inbound")
		}
		existing[member.Slot] = row
		managedEmails[row.Email] = true
	}
	createdEmails := make([]string, 0, len(desired))
	rollbackCreated := func() {
		for i := len(createdEmails) - 1; i >= 0; i-- {
			_, _ = s.ClientService.DeleteByEmail(&s.InboundService, createdEmails[i], false)
		}
	}
	for _, e := range desired {
		if _, ok := existing[e.Slot]; ok {
			continue
		}
		client := model.Client{Email: exitPoolEmail(req.SubID, e), SubID: req.SubID, Group: exitPoolGroupPrefix + req.Name, Comment: exitPoolComment(e), Enable: true, TrafficReset: "never", Security: "auto"}
		needRestart, createErr := s.ClientService.Create(&s.InboundService, &ClientCreatePayload{Client: client, InboundIds: []int{req.InboundID}, AllowSharedSubID: true})
		if needRestart {
			s.XrayService.SetToNeedRestart()
		}
		if createErr != nil {
			rollbackCreated()
			return ExitPoolGroup{}, createErr
		}
		createdEmails = append(createdEmails, client.Email)
		managedEmails[client.Email] = true
	}
	for slot, row := range existing {
		if slices.ContainsFunc(desired, func(e ExitPoolExit) bool { return e.Slot == slot }) {
			continue
		}
		needRestart, deleteErr := s.ClientService.DeleteByEmail(&s.InboundService, row.Email, false)
		if needRestart {
			s.XrayService.SetToNeedRestart()
		}
		if deleteErr != nil {
			return ExitPoolGroup{}, deleteErr
		}
	}
	if err := s.writeRoutingForSub(inbound, req.SubID, desired, managedEmails); err != nil {
		rollbackCreated()
		return ExitPoolGroup{}, err
	}
	if s.XrayService.IsXrayRunning() {
		if err := s.XrayService.RestartXray(false); err != nil {
			return ExitPoolGroup{}, err
		}
	}
	groups, err := s.groups(host)
	if err != nil {
		return ExitPoolGroup{}, err
	}
	for _, group := range groups {
		if group.SubID == req.SubID {
			return group, nil
		}
	}
	return ExitPoolGroup{}, errors.New("exit-pool group was not found after synchronization")
}

func (s *ExitPoolService) writeRoutingForSub(inbound *model.Inbound, subID string, desired []ExitPoolExit, managedEmails map[string]bool) error {
	raw, err := s.SettingService.GetXrayConfigTemplate()
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return err
	}
	outbounds, _ := cfg["outbounds"].([]any)
	byTag := map[string]map[string]any{}
	kept := make([]any, 0, len(outbounds)+len(desired))
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		tag, _ := m["tag"].(string)
		if strings.HasPrefix(tag, exitPoolOutboundPrefix) {
			byTag[tag] = m
		} else {
			kept = append(kept, item)
		}
	}
	for _, e := range desired {
		server := exitPoolSOCKSServer(e)
		byTag[exitPoolOutboundTag(e.Slot)] = map[string]any{"tag": exitPoolOutboundTag(e.Slot), "protocol": "socks", "settings": map[string]any{"servers": []any{server}}}
	}
	tags := make([]string, 0, len(byTag))
	for tag := range byTag {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	for _, tag := range tags {
		kept = append(kept, byTag[tag])
	}
	cfg["outbounds"] = kept
	routing, _ := cfg["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)
	cleaned := make([]any, 0, len(rules)+len(desired))
	for _, item := range rules {
		m, ok := item.(map[string]any)
		if !ok || !isManagedExitPoolRule(m) {
			cleaned = append(cleaned, item)
			continue
		}
		remove := false
		for _, user := range stringSlice(m["user"]) {
			if managedEmails[user] {
				remove = true
				break
			}
		}
		if !remove {
			cleaned = append(cleaned, item)
		}
	}
	for i := len(desired) - 1; i >= 0; i-- {
		e := desired[i]
		email := exitPoolEmail(subID, e)
		cleaned = append([]any{exitPoolRoutingRule(inbound.Tag, email, exitPoolOutboundTag(e.Slot))}, cleaned...)
	}
	routing["rules"] = cleaned
	cfg["routing"] = routing
	blob, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.XraySettingService.SaveXraySetting(string(blob))
}

func (s *ExitPoolService) Delete(subID string) error {
	if !exitPoolSubIDPattern.MatchString(subID) {
		return errors.New("invalid subId")
	}
	clients, err := s.ClientService.List()
	if err != nil {
		return err
	}
	emails := map[string]bool{}
	for _, row := range clients {
		if row.SubID == subID && strings.HasPrefix(row.Email, exitPoolClientPrefix) && strings.HasSuffix(row.Email, exitPoolClientDomain) {
			emails[row.Email] = true
			needRestart, deleteErr := s.ClientService.DeleteByEmail(&s.InboundService, row.Email, false)
			if needRestart {
				s.XrayService.SetToNeedRestart()
			}
			if deleteErr != nil {
				return deleteErr
			}
		}
	}
	if len(emails) == 0 {
		return errors.New("exit-pool group not found")
	}
	raw, err := s.SettingService.GetXrayConfigTemplate()
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return err
	}
	routing, _ := cfg["routing"].(map[string]any)
	if routing == nil {
		routing = map[string]any{}
	}
	rules, _ := routing["rules"].([]any)
	cleaned := make([]any, 0, len(rules))
	usedTags := map[string]bool{}
	for _, item := range rules {
		m, ok := item.(map[string]any)
		remove := false
		if ok && isManagedExitPoolRule(m) {
			for _, user := range stringSlice(m["user"]) {
				if emails[user] {
					remove = true
					break
				}
			}
		}
		if !remove {
			cleaned = append(cleaned, item)
			if ok {
				if tag, _ := m["outboundTag"].(string); strings.HasPrefix(tag, exitPoolOutboundPrefix) {
					usedTags[tag] = true
				}
			}
		}
	}
	routing["rules"] = cleaned
	cfg["routing"] = routing
	outbounds, _ := cfg["outbounds"].([]any)
	kept := make([]any, 0, len(outbounds))
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if ok {
			tag, _ := m["tag"].(string)
			if strings.HasPrefix(tag, exitPoolOutboundPrefix) && !usedTags[tag] {
				continue
			}
		}
		kept = append(kept, item)
	}
	cfg["outbounds"] = kept
	blob, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.XraySettingService.SaveXraySetting(string(blob)); err != nil {
		return err
	}
	if s.XrayService.IsXrayRunning() {
		return s.XrayService.RestartXray(false)
	}
	return nil
}
