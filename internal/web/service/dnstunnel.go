package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DNS tunnels (StormDNS for WhiteDNS-style apps, dnstt) run beside the panel, behind the
// server's port 53. The panel keeps their profiles for users and finds resolvers that pass them.

const dnsTunnelProfilesKey = "dnsTunnelProfiles"

//go:embed dnstunnel_script.py
var dnsTunnelScript string

var (
	errDnsTunnelBadProfile = errors.New("dns tunnel: bad profile")
	errDnsTunnelBadDir     = errors.New("dns tunnel: the StormDNS folder must be an absolute path")

	dnsTunnelKeyRe  = regexp.MustCompile(`^[A-Za-z0-9+/=_-]{16,128}$`)
	dnsTunnelNameRe = regexp.MustCompile(`^[\p{L}\p{N} ._()·-]{1,40}$`)

	dnsTunnelResolverNameRe = regexp.MustCompile(`^[A-Za-z0-9 ._()-]{1,32}$`)
)

// DnsTunnelProfile is one tunnel server users connect to: engine stormdns (WhiteDNS,
// MasterDNS-compatible apps) or dnstt, its delegated domain and its key.
type DnsTunnelProfile struct {
	Name   string `json:"name" example:"Iran"`
	Engine string `json:"engine" example:"stormdns"`
	Domain string `json:"domain" example:"st.t.example.com"`
	Key    string `json:"key" example:"93d3404c6701dc9dea2d1ff3282ab88b"`
	Method int    `json:"method" example:"1"`
}

type DnsTunnelCommandRequest struct {
	Zone      string   `json:"zone" example:"t.example.com"`
	Resolvers []string `json:"resolvers" example:"[\"178.22.122.100\"]"`
}

// DnsResolverResult is one resolver as the test saw it; MaxName is the longest query name it
// carried to the server (StormDNS packs its upload there), 0 when not measured.
type DnsResolverResult struct {
	Ip      string `json:"ip" example:"178.22.122.100"`
	Status  string `json:"status" example:"ok"`
	Ms      int    `json:"ms" example:"120"`
	Name    string `json:"name" example:"Shecan"`
	MaxName int    `json:"maxName" example:"253"`
}

type DnsResolverReport struct {
	At      int64               `json:"at" example:"1793000000"`
	Network CleanIpNetwork      `json:"network"`
	Zone    string              `json:"zone" example:"t.example.com"`
	Results []DnsResolverResult `json:"results"`
}

type DnsTunnelService struct {
	settingService SettingService
	cleanIpService CleanIpService
}

func (s *DnsTunnelService) GetProfiles() ([]DnsTunnelProfile, error) {
	raw, err := s.settingService.getString(dnsTunnelProfilesKey)
	if err != nil {
		return nil, err
	}
	profiles := []DnsTunnelProfile{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &profiles); err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

func validDnsTunnelProfile(p DnsTunnelProfile) bool {
	if p.Engine != "stormdns" && p.Engine != "dnstt" {
		return false
	}
	return dnsTunnelNameRe.MatchString(p.Name) && cleanIpHostRe.MatchString(p.Domain) &&
		dnsTunnelKeyRe.MatchString(p.Key) && p.Method >= 0 && p.Method <= 5
}

func (s *DnsTunnelService) SaveProfiles(profiles []DnsTunnelProfile) error {
	if len(profiles) > 20 {
		return errDnsTunnelBadProfile
	}
	for i := range profiles {
		profiles[i].Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(profiles[i].Domain), "."))
		profiles[i].Key = strings.TrimSpace(profiles[i].Key)
		profiles[i].Name = strings.TrimSpace(profiles[i].Name)
		if !validDnsTunnelProfile(profiles[i]) {
			return errDnsTunnelBadProfile
		}
	}
	raw, err := json.Marshal(profiles)
	if err != nil {
		return err
	}
	return s.settingService.setString(dnsTunnelProfilesKey, string(raw))
}

// DetectStormDns reads the StormDNS server installed in dir (server_config.toml and its key
// file) so the admin never copies the key by hand; one profile per served domain. An empty
// list means no StormDNS server is installed there, which the page explains.
func (s *DnsTunnelService) DetectStormDns(dir string) ([]DnsTunnelProfile, error) {
	if !filepath.IsAbs(dir) {
		return nil, errDnsTunnelBadDir
	}
	dir = filepath.Clean(dir)
	raw, err := os.ReadFile(filepath.Join(dir, "server_config.toml"))
	if errors.Is(err, fs.ErrNotExist) {
		return []DnsTunnelProfile{}, nil
	}
	if err != nil {
		return nil, err
	}
	conf := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(strings.TrimSpace(key), "#") {
			continue
		}
		if i := strings.Index(value, "#"); i >= 0 {
			value = value[:i]
		}
		conf[strings.TrimSpace(key)] = strings.ReplaceAll(strings.TrimSpace(value), "'", `"`)
	}
	var domains []string
	if err := json.Unmarshal([]byte(conf["DOMAIN"]), &domains); err != nil || len(domains) == 0 {
		return nil, errDnsTunnelBadProfile
	}
	method, err := strconv.Atoi(conf["DATA_ENCRYPTION_METHOD"])
	if err != nil {
		return nil, errDnsTunnelBadProfile
	}
	keyFile := filepath.Join(dir, "encrypt_key.txt")
	if v := conf["ENCRYPTION_KEY_FILE"]; v != "" {
		if err := json.Unmarshal([]byte(v), &keyFile); err != nil {
			return nil, errDnsTunnelBadProfile
		}
	}
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, errDnsTunnelBadProfile
	}
	profiles := make([]DnsTunnelProfile, 0, len(domains))
	for i, domain := range domains {
		p := DnsTunnelProfile{
			Name:   "StormDNS",
			Engine: "stormdns",
			Domain: strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), ".")),
			Key:    strings.TrimSpace(string(key)),
			Method: method,
		}
		if i > 0 {
			p.Name += " " + strconv.Itoa(i+1)
		}
		if !validDnsTunnelProfile(p) {
			return nil, errDnsTunnelBadProfile
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// ---- resolver scan: a one-day command run on a device on the ISP to test ----

var dnsTunnelJobs oneDayJobs[DnsTunnelCommandRequest, DnsResolverReport]

func validResolverIp(s string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil {
		return "", false
	}
	return ip.String(), true
}

func (s *DnsTunnelService) NewCommand(req DnsTunnelCommandRequest, baseURL string) (*CleanIpCommand, error) {
	req.Zone = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(req.Zone), "."))
	if !cleanIpHostRe.MatchString(req.Zone) {
		return nil, errCleanIpBadRequest
	}
	resolvers := make([]string, 0, len(req.Resolvers))
	for _, raw := range req.Resolvers {
		if ip, ok := validResolverIp(raw); ok && len(resolvers) < 200 {
			resolvers = append(resolvers, ip)
		}
	}
	req.Resolvers = resolvers

	token, expires, err := dnsTunnelJobs.add(req)
	if err != nil {
		return nil, err
	}

	scriptURL := baseURL + "dnstunnel/run/" + token + "/script.py"
	return &CleanIpCommand{
		Token:     token,
		ExpiresAt: expires.Unix(),
		ScriptUrl: scriptURL,
		Unix:      "curl -fsSL '" + scriptURL + "' | python3 -",
		Windows:   "irm '" + scriptURL + "' | python -",
	}, nil
}

func (s *DnsTunnelService) Script(token, reportURL string) (string, error) {
	req, err := dnsTunnelJobs.request(token)
	if err != nil {
		return "", err
	}
	return strings.NewReplacer(
		"__ZONE__", pyLiteral(req.Zone),
		"__RESOLVERS__", pyLiteral(req.Resolvers),
		"__REPORT__", pyLiteral(reportURL),
	).Replace(dnsTunnelScript), nil
}

func (s *DnsTunnelService) Reports(token string) ([]DnsResolverReport, error) {
	return dnsTunnelJobs.reports(token)
}

var dnsResolverStatuses = map[string]bool{"ok": true, "liar": true, "timeout": true, "servfail": true, "refused": true, "error": true}

func (s *DnsTunnelService) AddReport(ctx context.Context, token string, body []byte, reporterIp string) (*DnsResolverReport, error) {
	job, err := dnsTunnelJobs.request(token)
	if err != nil {
		return nil, err
	}
	zone := job.Zone
	var raw DnsResolverReport
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errCleanIpBadRequest
	}
	report := DnsResolverReport{At: cleanIpNow().Unix(), Zone: zone, Results: []DnsResolverResult{}}
	for _, r := range raw.Results {
		ip, ok := validResolverIp(r.Ip)
		if !ok || !dnsResolverStatuses[r.Status] || len(report.Results) >= 300 {
			continue
		}
		name := r.Name
		if !dnsTunnelResolverNameRe.MatchString(name) {
			name = ""
		}
		report.Results = append(report.Results, DnsResolverResult{Ip: ip, Status: r.Status, Ms: max(0, min(r.Ms, 60000)), Name: name, MaxName: max(0, min(r.MaxName, 253))})
	}
	// working resolvers first, fastest first
	sort.SliceStable(report.Results, func(i, j int) bool {
		a, b := report.Results[i], report.Results[j]
		if (a.Status == "ok") != (b.Status == "ok") {
			return a.Status == "ok"
		}
		return a.Ms < b.Ms
	})
	if network, err := s.cleanIpService.LookupNetwork(ctx, reporterIp); err == nil {
		report.Network = *network
	} else {
		report.Network = CleanIpNetwork{Ip: reporterIp}
	}

	if err := dnsTunnelJobs.addReport(token, report); err != nil {
		return nil, err
	}
	return &report, nil
}
