package service

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The full test (an upload) and the fragment test need TLS a browser can not do, so the
// page hands out a one-day command that runs cleanip_script.py on a device on the ISP.

//go:embed cleanip_script.py
var cleanIpScript string

const (
	cleanIpCommandTTL  = 24 * time.Hour
	cleanIpMaxCommands = 50
	cleanIpMaxReports  = 20
)

var (
	errCleanIpBadRequest = errors.New("clean ip: bad request")
	errCleanIpExpired    = errors.New("clean ip: command expired")

	cleanIpHostRe  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	cleanIpPathRe  = regexp.MustCompile(`^/[A-Za-z0-9._~!$&()*+,;=:@%/?-]{0,255}$`)
	cleanIpRangeRe = regexp.MustCompile(`^\d{1,5}-\d{1,5}$`)
	cleanIpColoRe  = regexp.MustCompile(`^[A-Z]{0,8}$`)

	cleanIpNow = time.Now
)

// CleanIpCommandRequest is what the full test runs with: the CDN domain as SNI, the path
// it uploads to, and the IPs the page already found (they seed the fragment test).
type CleanIpCommandRequest struct {
	Host     string   `json:"host" example:"cdn.example.com"`
	Path     string   `json:"path" example:"/"`
	Size     int      `json:"size" example:"5000"`
	PerRange int      `json:"perRange" example:"8"`
	Ips      []string `json:"ips" example:"[\"104.16.1.1\"]"`
	Cdn      string   `json:"cdn" example:"cloudflare"`
	Ranges   []string `json:"ranges" example:"[\"104.16.0.0/13\"]"`
}

type CleanIpCommand struct {
	Token     string `json:"token" example:"kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D"`
	ExpiresAt int64  `json:"expiresAt" example:"1793000000"`
	ScriptUrl string `json:"scriptUrl" example:"https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py"`
	Unix      string `json:"unix" example:"python3 -c \"import urllib.request as u,hashlib as h;s=u.urlopen(u.Request('https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py',headers={'User-Agent':'Mozilla/5.0 (3x-ui clean IP)'})).read();exec(s) if h.sha256(s).hexdigest()=='9f2c...' else print('The script did not match this command: make a new one on the Clean IP scanner page.')\""`
	Windows   string `json:"windows" example:"python -c \"import urllib.request as u,hashlib as h;s=u.urlopen(u.Request('https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py',headers={'User-Agent':'Mozilla/5.0 (3x-ui clean IP)'})).read();exec(s) if h.sha256(s).hexdigest()=='9f2c...' else print('The script did not match this command: make a new one on the Clean IP scanner page.')\""`
	// Via, UnixVia, WindowsVia reach the panel's domain through the fastest page IP, for an
	// ISP that blocks the domain itself; empty without a page IP or for a panel opened by IP.
	Via        string `json:"via" example:"104.16.1.1"`
	UnixVia    string `json:"unixVia" example:"python3 -c \"import urllib.request as u,hashlib as h,socket as o;g=o.getaddrinfo;o.getaddrinfo=lambda a,*r,**k:g('104.16.1.1' if a=='panel.example.com' else a,*r,**k);s=u.urlopen(u.Request('https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py?via=104.16.1.1',headers={'User-Agent':'Mozilla/5.0 (3x-ui clean IP)'})).read();o.getaddrinfo=g;exec(s) if h.sha256(s).hexdigest()=='9f2c...' else print('The script did not match this command: make a new one on the Clean IP scanner page.')\""`
	WindowsVia string `json:"windowsVia" example:"python -c \"import urllib.request as u,hashlib as h,socket as o;g=o.getaddrinfo;o.getaddrinfo=lambda a,*r,**k:g('104.16.1.1' if a=='panel.example.com' else a,*r,**k);s=u.urlopen(u.Request('https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py?via=104.16.1.1',headers={'User-Agent':'Mozilla/5.0 (3x-ui clean IP)'})).read();o.getaddrinfo=g;exec(s) if h.sha256(s).hexdigest()=='9f2c...' else print('The script did not match this command: make a new one on the Clean IP scanner page.')\""`
}

type CleanIpHit struct {
	Ip   string `json:"ip" example:"104.16.1.1"`
	Ms   int    `json:"ms" example:"410"`
	Colo string `json:"colo" example:"FRA"`
}

// CleanIpFragment is one fragment setting the test tried; an empty Packets is the run
// without a fragment, to compare against.
type CleanIpFragment struct {
	Packets string `json:"packets" example:"tlshello"`
	Length  string `json:"length" example:"10-20"`
	Delay   string `json:"delay" example:"1-2"`
	Ok      int    `json:"ok" example:"9"`
	N       int    `json:"n" example:"10"`
	Ms      int    `json:"ms" example:"700"`
}

type CleanIpReport struct {
	At        int64             `json:"at" example:"1793000000"`
	Network   CleanIpNetwork    `json:"network"`
	Tested    int               `json:"tested" example:"120"`
	Size      int               `json:"size" example:"5000"`
	Ips       []CleanIpHit      `json:"ips"`
	Fragments []CleanIpFragment `json:"fragments"`
}

var cleanIpJobs oneDayJobs[CleanIpCommandRequest, CleanIpReport]

func clampInt(v, def, lo, hi int) int {
	if v == 0 {
		return def
	}
	return max(lo, min(v, hi))
}

func validIPv4(s string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(s))
	if ip == nil || ip.To4() == nil {
		return "", false
	}
	return ip.To4().String(), true
}

// NewCommand stores a one-day job and returns the commands that fetch its script;
// baseURL is the panel as the admin's browser reaches it, ending in "/".
func (s *CleanIpService) NewCommand(req CleanIpCommandRequest, baseURL string) (*CleanIpCommand, error) {
	req.Host = strings.TrimSpace(req.Host)
	if strings.ContainsAny(baseURL, "'\"\\") {
		return nil, errCleanIpBadRequest
	}
	if req.Path == "" {
		req.Path = "/"
	}
	if !cleanIpHostRe.MatchString(req.Host) || !cleanIpPathRe.MatchString(req.Path) {
		return nil, errCleanIpBadRequest
	}
	req.Size = clampInt(req.Size, 5000, 1000, 64000)
	req.PerRange = clampInt(req.PerRange, 8, 1, 64)
	if req.Cdn == "" {
		req.Cdn = cleanIpCdns[0].Id
	}
	if cleanIpCdnRanges(req.Cdn) == nil {
		return nil, errCleanIpBadRequest
	}
	if req.Ranges = cleanIpTickedRanges(req.Ranges); len(req.Ranges) == 0 {
		req.Ranges = cleanIpCdnRanges(req.Cdn)
	}
	ips := make([]string, 0, len(req.Ips))
	for _, raw := range req.Ips {
		if ip, ok := validIPv4(raw); ok && len(ips) < 100 {
			ips = append(ips, ip)
		}
	}
	req.Ips = ips

	token, expires, err := cleanIpJobs.add(req)
	if err != nil {
		return nil, err
	}

	scriptURL := baseURL + "cleanip/run/" + token + "/script.py"
	reportURL := baseURL + "cleanip/run/" + token + "/report"
	scripts := map[string]string{"": cleanIpBuildScript(req, reportURL, "")}
	cmd := &CleanIpCommand{Token: token, ExpiresAt: expires.Unix(), ScriptUrl: scriptURL}
	cmd.Unix = "python3 " + hashedRunCommand(scriptURL, scripts[""], "", "", cleanIpPage)
	cmd.Windows = "python " + hashedRunCommand(scriptURL, scripts[""], "", "", cleanIpPage)
	if domain := panelDomain(baseURL); domain != "" && len(req.Ips) > 0 {
		cmd.Via = req.Ips[0]
		scripts[cmd.Via] = cleanIpBuildScript(req, reportURL, cmd.Via)
		run := hashedRunCommand(scriptURL+"?via="+cmd.Via, scripts[cmd.Via], domain, cmd.Via, cleanIpPage)
		cmd.UnixVia, cmd.WindowsVia = "python3 "+run, "python "+run
	}
	if err := cleanIpJobs.setScripts(token, scripts); err != nil {
		return nil, err
	}
	return cmd, nil
}

const cleanIpPage = "Clean IP scanner page"

// hashedRunCommand is the python -c that fetches the script and runs it only if its SHA-256
// matches, since the fetch may cross the filtered ISP in plain HTTP. A pin acts as curl --resolve.
func hashedRunCommand(scriptURL, script, pinDomain, pinIP, page string) string {
	sum := sha256.Sum256([]byte(script))
	code := "import urllib.request as u,hashlib as h"
	if pinDomain != "" {
		code += ",socket as o;g=o.getaddrinfo;o.getaddrinfo=lambda a,*r,**k:g('" + pinIP +
			"' if a=='" + pinDomain + "' else a,*r,**k)"
	}
	// Cloudflare answers Python's default User-Agent with error 1010.
	code += ";s=u.urlopen(u.Request('" + scriptURL + "',headers={'User-Agent':'Mozilla/5.0 (3x-ui clean IP)'})).read()"
	if pinDomain != "" {
		code += ";o.getaddrinfo=g"
	}
	code += ";exec(s) if h.sha256(s).hexdigest()=='" + hex.EncodeToString(sum[:]) +
		"' else print('The script did not match this command: make a new one on the " + page + ".')"
	return `-c "` + code + `"`
}

// panelDomain is the panel's domain for the pinned command; a panel opened by IP has no
// domain an ISP could block, so it is not pinned.
func panelDomain(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" || net.ParseIP(u.Hostname()) != nil {
		return ""
	}
	return u.Hostname()
}

func pyLiteral(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// cleanIpCdnRanges is a CDN's default selection: its groups that are on.
func cleanIpCdnRanges(id string) []string {
	for _, cdn := range cleanIpCdns {
		if cdn.Id != id {
			continue
		}
		var ranges []string
		for _, g := range cdn.Groups {
			if g.On {
				ranges = append(ranges, g.Ranges...)
			}
		}
		return ranges
	}
	return nil
}

// cleanIpTickedRanges keeps the admin's IPv4 ranges the script can sample (/8 to /30).
func cleanIpTickedRanges(raw []string) []string {
	seen := map[string]bool{}
	ranges := make([]string, 0, len(raw))
	for _, r := range raw {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(r))
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 8 || prefix.Bits() > 30 {
			continue
		}
		key := prefix.Masked().String()
		if !seen[key] && len(ranges) < 3000 {
			seen[key] = true
			ranges = append(ranges, key)
		}
	}
	return ranges
}

func (s *CleanIpService) Cdns() []CleanIpCdn {
	return cleanIpCdns
}

// Script is the job's test, fixed when the command was made so the command can hash it; via
// is the IP a pinned command was fetched through (?via=).
func (s *CleanIpService) Script(token, via string) (string, error) {
	return cleanIpJobs.script(token, via)
}

// cleanIpBuildScript writes the job's values in as JSON, which Python reads as literals.
func cleanIpBuildScript(req CleanIpCommandRequest, reportURL, via string) string {
	return strings.NewReplacer(
		"__HOST__", pyLiteral(req.Host),
		"__PATH__", pyLiteral(req.Path),
		"__REPORT__", pyLiteral(reportURL),
		"__VIA__", pyLiteral(via),
		"__SIZE__", strconv.Itoa(req.Size),
		"__PER_RANGE__", strconv.Itoa(req.PerRange),
		"__RANGES__", pyLiteral(req.Ranges),
		"__IPS__", pyLiteral(req.Ips),
	).Replace(cleanIpScript)
}

func (s *CleanIpService) Reports(token string) ([]CleanIpReport, error) {
	return cleanIpJobs.reports(token)
}

func sanitizeFragment(f CleanIpFragment) (CleanIpFragment, bool) {
	out := CleanIpFragment{Ok: max(0, min(f.Ok, 1000)), N: max(0, min(f.N, 1000)), Ms: max(0, min(f.Ms, 60000))}
	switch f.Packets {
	case "":
		return out, f.Length == "" && f.Delay == ""
	case "tlshello", "1-1":
		if !cleanIpRangeRe.MatchString(f.Length) || !cleanIpRangeRe.MatchString(f.Delay) {
			return out, false
		}
		out.Packets, out.Length, out.Delay = f.Packets, f.Length, f.Delay
		return out, true
	}
	return out, false
}

// AddReport records what a run of the script found, under the ISP it reported from.
func (s *CleanIpService) AddReport(ctx context.Context, token string, body []byte, reporterIp string) (*CleanIpReport, error) {
	if _, err := cleanIpJobs.request(token); err != nil {
		return nil, err
	}
	var raw CleanIpReport
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errCleanIpBadRequest
	}
	report := CleanIpReport{
		At:     cleanIpNow().Unix(),
		Tested: max(0, min(raw.Tested, 100000)),
		Size:   max(0, min(raw.Size, 64000)),
		// empty, not nil: the page reads both as arrays
		Ips:       []CleanIpHit{},
		Fragments: []CleanIpFragment{},
	}
	for _, hit := range raw.Ips {
		ip, ok := validIPv4(hit.Ip)
		if !ok || len(report.Ips) >= 200 {
			continue
		}
		colo := hit.Colo
		if !cleanIpColoRe.MatchString(colo) {
			colo = ""
		}
		report.Ips = append(report.Ips, CleanIpHit{Ip: ip, Ms: max(0, min(hit.Ms, 60000)), Colo: colo})
	}
	sort.SliceStable(report.Ips, func(i, j int) bool { return report.Ips[i].Ms < report.Ips[j].Ms })
	for _, f := range raw.Fragments {
		if clean, ok := sanitizeFragment(f); ok && len(report.Fragments) < 40 {
			report.Fragments = append(report.Fragments, clean)
		}
	}
	if network, err := s.LookupNetwork(ctx, reporterIp); err == nil {
		report.Network = *network
	} else {
		report.Network = CleanIpNetwork{Ip: reporterIp}
	}

	if err := cleanIpJobs.addReport(token, report); err != nil {
		return nil, err
	}
	return &report, nil
}
