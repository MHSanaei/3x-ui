package service

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// DNS tunnels (StormDNS for WhiteDNS-style apps, dnstt) run beside the panel, behind the
// server's port 53. The panel keeps their profiles for users and finds resolvers that pass them.

const dnsTunnelProfilesKey = "dnsTunnelProfiles"

//go:embed dnstunnel_script.py
var dnsTunnelScript string

var (
	errDnsTunnelBadProfile = errors.New("dns tunnel: bad profile")

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

type DnsResolverResult struct {
	Ip     string `json:"ip" example:"178.22.122.100"`
	Status string `json:"status" example:"ok"`
	Ms     int    `json:"ms" example:"120"`
	Name   string `json:"name" example:"Shecan"`
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

// ---- resolver scan: a one-day command run on a device on the ISP to test ----

type dnsTunnelJob struct {
	req     DnsTunnelCommandRequest
	expires time.Time
	reports []DnsResolverReport
}

var (
	dnsTunnelJobsMu sync.Mutex
	dnsTunnelJobs   = map[string]*dnsTunnelJob{}
)

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

	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := cleanIpNow()
	expires := now.Add(cleanIpCommandTTL)
	dnsTunnelJobsMu.Lock()
	for t, job := range dnsTunnelJobs {
		if now.After(job.expires) {
			delete(dnsTunnelJobs, t)
		}
	}
	if len(dnsTunnelJobs) >= cleanIpMaxCommands {
		dnsTunnelJobsMu.Unlock()
		return nil, errCleanIpBadRequest
	}
	dnsTunnelJobs[token] = &dnsTunnelJob{req: req, expires: expires}
	dnsTunnelJobsMu.Unlock()

	scriptURL := baseURL + "dnstunnel/run/" + token + "/script.py"
	return &CleanIpCommand{
		Token:     token,
		ExpiresAt: expires.Unix(),
		ScriptUrl: scriptURL,
		Unix:      "curl -fsSL '" + scriptURL + "' | python3 -",
		Windows:   "irm '" + scriptURL + "' | python -",
	}, nil
}

// dnsTunnelJobFor needs dnsTunnelJobsMu held.
func dnsTunnelJobFor(token string) (*dnsTunnelJob, error) {
	job, ok := dnsTunnelJobs[token]
	if !ok || cleanIpNow().After(job.expires) {
		return nil, errCleanIpExpired
	}
	return job, nil
}

func (s *DnsTunnelService) Script(token, reportURL string) (string, error) {
	dnsTunnelJobsMu.Lock()
	job, err := dnsTunnelJobFor(token)
	var req DnsTunnelCommandRequest
	if err == nil {
		req = job.req
	}
	dnsTunnelJobsMu.Unlock()
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
	dnsTunnelJobsMu.Lock()
	defer dnsTunnelJobsMu.Unlock()
	job, err := dnsTunnelJobFor(token)
	if err != nil {
		return nil, err
	}
	return append([]DnsResolverReport{}, job.reports...), nil
}

var dnsResolverStatuses = map[string]bool{"ok": true, "timeout": true, "servfail": true, "refused": true, "error": true}

func (s *DnsTunnelService) AddReport(ctx context.Context, token string, body []byte, reporterIp string) (*DnsResolverReport, error) {
	dnsTunnelJobsMu.Lock()
	job, err := dnsTunnelJobFor(token)
	zone := ""
	if err == nil {
		zone = job.req.Zone
	}
	dnsTunnelJobsMu.Unlock()
	if err != nil {
		return nil, err
	}
	var raw DnsResolverReport
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errCleanIpBadRequest
	}
	report := DnsResolverReport{At: cleanIpNow().Unix(), Zone: zone}
	for _, r := range raw.Results {
		ip, ok := validResolverIp(r.Ip)
		if !ok || !dnsResolverStatuses[r.Status] || len(report.Results) >= 300 {
			continue
		}
		name := r.Name
		if !dnsTunnelResolverNameRe.MatchString(name) {
			name = ""
		}
		report.Results = append(report.Results, DnsResolverResult{Ip: ip, Status: r.Status, Ms: max(0, min(r.Ms, 60000)), Name: name})
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

	dnsTunnelJobsMu.Lock()
	defer dnsTunnelJobsMu.Unlock()
	job, err = dnsTunnelJobFor(token)
	if err != nil {
		return nil, err
	}
	job.reports = append(job.reports, report)
	if len(job.reports) > cleanIpMaxReports {
		job.reports = job.reports[len(job.reports)-cleanIpMaxReports:]
	}
	return &report, nil
}
