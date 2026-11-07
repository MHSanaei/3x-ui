package service

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// The finder probes from a device on the ISP the users have, never from this server: only
// there can a filtered SNI be seen, and the server sends no probes a host could flag.

//go:embed reality_finder_script.py
var realityFinderScript string

const (
	realityFinderCommandTTL  = 24 * time.Hour
	realityFinderMaxCommands = 50
	realityFinderMaxReports  = 20
	realityFinderMaxTargets  = 100
)

var (
	errRealityFinderBadRequest = errors.New("reality finder: bad request")
	errRealityFinderExpired    = errors.New("reality finder: command expired")
	errRealityFinderNoServer   = errors.New("reality finder: no public IPv4 for this server, enter it")

	realityFinderNameRe    = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	realityFinderCountryRe = regexp.MustCompile(`^[A-Z]{0,2}$`)
	realityFinderScopes    = []string{"24", "22", "20", "prefix", "asn"}
	realityFinderCdns      = []string{"", "cloudflare", "fastly", "gcore", "cloudfront"}

	realityFinderNow = time.Now
)

// RealityFinderCommandRequest is what the device scans: the server's neighbours (scope) and,
// through the inbound's port, whether each name gets past the ISP. Port 0 skips that test.
type RealityFinderCommandRequest struct {
	Server  string `json:"server" example:"203.0.113.10"`
	Port    int    `json:"port" example:"443"`
	Scope   string `json:"scope" example:"24"`
	Current string `json:"current" example:"www.example.com"`
	NodeId  *int   `json:"nodeId" example:"1"`
}

type RealityFinderCommand struct {
	Token     string `json:"token" example:"kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D"`
	ExpiresAt int64  `json:"expiresAt" example:"1793000000"`
	Server    string `json:"server" example:"203.0.113.10"`
	ScriptUrl string `json:"scriptUrl" example:"https://panel.example.com/path/realityfinder/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py"`
	Unix      string `json:"unix" example:"python3 -c \"import urllib.request as u,hashlib as h;s=u.urlopen(u.Request('https://panel.example.com/path/realityfinder/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py',headers={'User-Agent':'Mozilla/5.0 (3x-ui REALITY finder)'})).read();exec(s) if h.sha256(s).hexdigest()=='9f2c...' else print('The script did not match this command: make a new one in the REALITY target scanner.')\""`
	Windows   string `json:"windows" example:"python -c \"import urllib.request as u,hashlib as h;s=u.urlopen(u.Request('https://panel.example.com/path/realityfinder/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py',headers={'User-Agent':'Mozilla/5.0 (3x-ui REALITY finder)'})).read();exec(s) if h.sha256(s).hexdigest()=='9f2c...' else print('The script did not match this command: make a new one in the REALITY target scanner.')\""`
}

type RealityFinderNetwork struct {
	Ip      string `json:"ip" example:"5.120.1.2"`
	Asn     int    `json:"asn" example:"197207"`
	Org     string `json:"org" example:"Mobile Communication Company of Iran PLC"`
	Country string `json:"country" example:"IR"`
}

type RealityFinderServer struct {
	Ip     string `json:"ip" example:"203.0.113.10"`
	Asn    int    `json:"asn" example:"24940"`
	Holder string `json:"holder" example:"HETZNER-AS"`
	Prefix string `json:"prefix" example:"203.0.113.0/24"`
}

// RealityFinderTarget is a name that passed TLS 1.3, X25519, h2 and a valid certificate from
// the device. Filtered is null when the filter test did not run.
type RealityFinderTarget struct {
	Sni      string `json:"sni" example:"www.example.com"`
	Ip       string `json:"ip" example:"203.0.113.20"`
	Score    int    `json:"score" example:"96"`
	Ms       int    `json:"ms" example:"240"`
	Filtered *bool  `json:"filtered" example:"false"`
	Same24   bool   `json:"same24" example:"true"`
	SameAsn  bool   `json:"sameAsn" example:"true"`
	Cdn      string `json:"cdn" example:""`
	DnsMatch bool   `json:"dnsMatch" example:"true"`
	Issuer   string `json:"issuer" example:"Let's Encrypt"`
	Expires  string `json:"expires" example:"Jan 30 12:00:00 2027 GMT"`
}

type RealityFinderReport struct {
	At           int64                 `json:"at" example:"1793000000"`
	Network      RealityFinderNetwork  `json:"network"`
	Warning      string                `json:"warning" example:""`
	Router       bool                  `json:"router" example:"false"`
	Tested       int                   `json:"tested" example:"253"`
	Server       RealityFinderServer   `json:"server"`
	Port         int                   `json:"port" example:"443"`
	FilterTested bool                  `json:"filterTested" example:"true"`
	Targets      []RealityFinderTarget `json:"targets"`
}

type realityFinderJob struct {
	req     RealityFinderCommandRequest
	script  string
	expires time.Time
	reports []RealityFinderReport
}

var (
	realityFinderJobsMu sync.Mutex
	realityFinderJobs   = map[string]*realityFinderJob{}
)

type RealityFinderService struct {
	serverService ServerService
	nodeService   NodeService
}

func realityFinderServer(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if addr, err := netip.ParseAddr(raw); err == nil {
		ok := addr.Is4() && addr.IsGlobalUnicast() && !addr.IsPrivate()
		return addr.String(), ok
	}
	return strings.ToLower(raw), realityFinderNameRe.MatchString(raw)
}

// NewCommand stores a one-day job and returns the commands that run its script; baseURL is
// the panel as the admin's browser reaches it, ending in "/".
func (s *RealityFinderService) NewCommand(req RealityFinderCommandRequest, baseURL string) (*RealityFinderCommand, error) {
	if strings.TrimSpace(req.Server) == "" {
		server, err := s.defaultServer(req.NodeId)
		if err != nil {
			return nil, err
		}
		req.Server = server
	}
	server, ok := realityFinderServer(req.Server)
	if !ok || req.Port < 0 || req.Port > 65535 {
		return nil, errRealityFinderBadRequest
	}
	req.Server = server
	if req.Scope == "" {
		req.Scope = "24"
	}
	if !slices.Contains(realityFinderScopes, req.Scope) {
		return nil, errRealityFinderBadRequest
	}
	req.Current = strings.ToLower(strings.TrimSpace(req.Current))
	if !realityFinderNameRe.MatchString(req.Current) {
		req.Current = ""
	}

	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := realityFinderNow()
	expires := now.Add(realityFinderCommandTTL)

	realityFinderJobsMu.Lock()
	for t, job := range realityFinderJobs {
		if now.After(job.expires) {
			delete(realityFinderJobs, t)
		}
	}
	for len(realityFinderJobs) >= realityFinderMaxCommands {
		oldest := ""
		for t, job := range realityFinderJobs {
			if oldest == "" || job.expires.Before(realityFinderJobs[oldest].expires) {
				oldest = t
			}
		}
		delete(realityFinderJobs, oldest)
	}
	script := realityFinderBuildScript(req, baseURL+"realityfinder/run/"+token+"/report")
	realityFinderJobs[token] = &realityFinderJob{req: req, script: script, expires: expires}
	realityFinderJobsMu.Unlock()

	scriptURL := baseURL + "realityfinder/run/" + token + "/script.py"
	sum := sha256.Sum256([]byte(script))
	// python -c keeps stdin a terminal for the VPN-off prompt; the fetch may cross the users' ISP
	// in plain HTTP, so only the hashed script runs. Cloudflare refuses Python's User-Agent (1010).
	oneLiner := `-c "import urllib.request as u,hashlib as h;s=u.urlopen(u.Request('` + scriptURL +
		`',headers={'User-Agent':'Mozilla/5.0 (3x-ui REALITY finder)'})).read();exec(s) if h.sha256(s).hexdigest()=='` +
		hex.EncodeToString(sum[:]) + `' else print('The script did not match this command: make a new one in the REALITY target scanner.')"`
	return &RealityFinderCommand{
		Token:     token,
		ExpiresAt: expires.Unix(),
		Server:    req.Server,
		ScriptUrl: scriptURL,
		Unix:      "python3 " + oneLiner,
		Windows:   "python " + oneLiner,
	}, nil
}

// realityFinderJobFor needs realityFinderJobsMu held.
func realityFinderJobFor(token string) (*realityFinderJob, error) {
	job, ok := realityFinderJobs[token]
	if !ok || realityFinderNow().After(job.expires) {
		return nil, errRealityFinderExpired
	}
	return job, nil
}

// Script is the job's finder, fixed when the command was made so the command can hash it.
func (s *RealityFinderService) Script(token string) (string, error) {
	realityFinderJobsMu.Lock()
	defer realityFinderJobsMu.Unlock()
	job, err := realityFinderJobFor(token)
	if err != nil {
		return "", err
	}
	return job.script, nil
}

// realityFinderBuildScript writes the job's values in as JSON, which Python reads as literals.
func realityFinderBuildScript(req RealityFinderCommandRequest, reportURL string) string {
	literal := func(v string) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	return strings.NewReplacer(
		"__SERVER__", literal(req.Server),
		"__PORT__", strconv.Itoa(req.Port),
		"__SCOPE__", literal(req.Scope),
		"__KNOWN__", literal(req.Current),
		"__REPORT__", literal(reportURL),
	).Replace(realityFinderScript)
}

// defaultServer is the inbound's own server: its node when it is deployed to one.
func (s *RealityFinderService) defaultServer(nodeID *int) (string, error) {
	if nodeID != nil {
		node, err := s.nodeService.GetById(*nodeID)
		if err != nil {
			return "", err
		}
		if server, ok := realityFinderServer(node.Address); ok {
			return server, nil
		}
		return "", errRealityFinderNoServer
	}
	s.serverService.resolvePublicIPs()
	ipv4, _ := s.serverService.publicIPs()
	if _, ok := realityFinderServer(ipv4); !ok {
		return "", errRealityFinderNoServer
	}
	return ipv4, nil
}

func (s *RealityFinderService) Reports(token string) ([]RealityFinderReport, error) {
	realityFinderJobsMu.Lock()
	defer realityFinderJobsMu.Unlock()
	job, err := realityFinderJobFor(token)
	if err != nil {
		return nil, err
	}
	return append([]RealityFinderReport{}, job.reports...), nil
}

// realityFinderText keeps a short printable label from the device: it is shown in the panel.
func realityFinderText(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if runes := []rune(s); len(runes) > limit {
		s = string(runes[:limit])
	}
	return s
}

func realityFinderIPv4(s string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil || !addr.Is4() {
		return "", false
	}
	return addr.String(), true
}

func sanitizeRealityFinderTarget(t RealityFinderTarget) (RealityFinderTarget, bool) {
	sni := strings.ToLower(strings.TrimSpace(t.Sni))
	ip, ok := realityFinderIPv4(t.Ip)
	if !ok || !realityFinderNameRe.MatchString(sni) {
		return RealityFinderTarget{}, false
	}
	cdn := t.Cdn
	if !slices.Contains(realityFinderCdns, cdn) {
		cdn = ""
	}
	return RealityFinderTarget{
		Sni: sni, Ip: ip,
		Score:    max(0, min(t.Score, 100)),
		Ms:       max(0, min(t.Ms, 60000)),
		Filtered: t.Filtered,
		Same24:   t.Same24, SameAsn: t.SameAsn, Cdn: cdn, DnsMatch: t.DnsMatch,
		Issuer:  realityFinderText(t.Issuer, 80),
		Expires: realityFinderText(t.Expires, 40),
	}, true
}

// AddReport records what a run of the script found; the network's IP is the one the report
// came from, the rest is what the device read about itself.
func (s *RealityFinderService) AddReport(token string, body []byte, reporterIp string) (*RealityFinderReport, error) {
	var raw RealityFinderReport
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errRealityFinderBadRequest
	}
	report := RealityFinderReport{
		At: realityFinderNow().Unix(),
		Network: RealityFinderNetwork{
			Ip:  reporterIp,
			Asn: max(0, raw.Network.Asn), Org: realityFinderText(raw.Network.Org, 80),
		},
		Router:       raw.Router,
		Tested:       max(0, min(raw.Tested, 1000000)),
		Port:         max(0, min(raw.Port, 65535)),
		FilterTested: raw.FilterTested,
		Server: RealityFinderServer{
			Asn: max(0, raw.Server.Asn), Holder: realityFinderText(raw.Server.Holder, 80),
		},
		// empty, not nil: the page reads both as arrays
		Targets: []RealityFinderTarget{},
	}
	if realityFinderCountryRe.MatchString(raw.Network.Country) {
		report.Network.Country = raw.Network.Country
	}
	if raw.Warning == "vpn" || raw.Warning == "hosting" {
		report.Warning = raw.Warning
	}
	if ip, ok := realityFinderIPv4(raw.Server.Ip); ok {
		report.Server.Ip = ip
	}
	if prefix, err := netip.ParsePrefix(strings.TrimSpace(raw.Server.Prefix)); err == nil {
		report.Server.Prefix = prefix.Masked().String()
	}
	seen := map[string]bool{}
	for _, t := range raw.Targets {
		clean, ok := sanitizeRealityFinderTarget(t)
		if !ok || seen[clean.Sni] || len(report.Targets) >= realityFinderMaxTargets {
			continue
		}
		seen[clean.Sni] = true
		report.Targets = append(report.Targets, clean)
	}
	sort.SliceStable(report.Targets, func(i, j int) bool {
		a, b := report.Targets[i], report.Targets[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Ms < b.Ms
	})

	realityFinderJobsMu.Lock()
	defer realityFinderJobsMu.Unlock()
	job, err := realityFinderJobFor(token)
	if err != nil {
		return nil, err
	}
	job.reports = append(job.reports, report)
	if len(job.reports) > realityFinderMaxReports {
		job.reports = job.reports[len(job.reports)-realityFinderMaxReports:]
	}
	return &report, nil
}
