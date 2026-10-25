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
	"strconv"
	"strings"
	"sync"
	"time"
)

// The full test (an upload) and the fragment test need TLS a browser can not do, so the
// page hands out a one-day command that runs cleanip_script.py on a device on the ISP.

//go:embed cleanip_script.py
var cleanIpScript string

// https://www.cloudflare.com/ips-v4/
var cleanIpCloudflareRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18",
	"108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17",
	"162.158.0.0/15", "104.16.0.0/13", "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
}

const (
	cleanIpCommandTTL  = 24 * time.Hour
	cleanIpMaxCommands = 50
	cleanIpMaxReports  = 20
)

var (
	errCleanIpBadRequest = errors.New("clean ip: bad request")
	errCleanIpExpired    = errors.New("clean ip: command expired")

	cleanIpHostRe  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	cleanIpPathRe  = regexp.MustCompile(`^/[A-Za-z0-9._~!$&()*+,;=:@%/-]{0,255}$`)
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
}

type CleanIpCommand struct {
	Token     string `json:"token" example:"kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D"`
	ExpiresAt int64  `json:"expiresAt" example:"1793000000"`
	ScriptUrl string `json:"scriptUrl" example:"https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py"`
	Unix      string `json:"unix" example:"curl -fsSL 'https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py' | python3 -"`
	Windows   string `json:"windows" example:"irm 'https://panel.example.com/path/cleanip/run/kP3x9Qw7Lm2Zr8Tv5Yb1Nc4D/script.py' | python -"`
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

type cleanIpJob struct {
	req     CleanIpCommandRequest
	expires time.Time
	reports []CleanIpReport
}

var (
	cleanIpJobsMu sync.Mutex
	cleanIpJobs   = map[string]*cleanIpJob{}
)

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
	if req.Path == "" {
		req.Path = "/"
	}
	if !cleanIpHostRe.MatchString(req.Host) || !cleanIpPathRe.MatchString(req.Path) {
		return nil, errCleanIpBadRequest
	}
	req.Size = clampInt(req.Size, 5000, 1000, 64000)
	req.PerRange = clampInt(req.PerRange, 8, 1, 64)
	ips := make([]string, 0, len(req.Ips))
	for _, raw := range req.Ips {
		if ip, ok := validIPv4(raw); ok && len(ips) < 20 {
			ips = append(ips, ip)
		}
	}
	req.Ips = ips

	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := cleanIpNow()
	expires := now.Add(cleanIpCommandTTL)

	cleanIpJobsMu.Lock()
	for t, job := range cleanIpJobs {
		if now.After(job.expires) {
			delete(cleanIpJobs, t)
		}
	}
	for len(cleanIpJobs) >= cleanIpMaxCommands {
		oldest := ""
		for t, job := range cleanIpJobs {
			if oldest == "" || job.expires.Before(cleanIpJobs[oldest].expires) {
				oldest = t
			}
		}
		delete(cleanIpJobs, oldest)
	}
	cleanIpJobs[token] = &cleanIpJob{req: req, expires: expires}
	cleanIpJobsMu.Unlock()

	scriptURL := baseURL + "cleanip/run/" + token + "/script.py"
	return &CleanIpCommand{
		Token:     token,
		ExpiresAt: expires.Unix(),
		ScriptUrl: scriptURL,
		Unix:      "curl -fsSL '" + scriptURL + "' | python3 -",
		Windows:   "irm '" + scriptURL + "' | python -",
	}, nil
}

// cleanIpJobFor needs cleanIpJobsMu held.
func cleanIpJobFor(token string) (*cleanIpJob, error) {
	job, ok := cleanIpJobs[token]
	if !ok || cleanIpNow().After(job.expires) {
		return nil, errCleanIpExpired
	}
	return job, nil
}

func pyLiteral(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Script is the job's test with its values written in as JSON, which Python reads as literals.
func (s *CleanIpService) Script(token, reportURL string) (string, error) {
	cleanIpJobsMu.Lock()
	job, err := cleanIpJobFor(token)
	var req CleanIpCommandRequest
	if err == nil {
		req = job.req
	}
	cleanIpJobsMu.Unlock()
	if err != nil {
		return "", err
	}
	return strings.NewReplacer(
		"__HOST__", pyLiteral(req.Host),
		"__PATH__", pyLiteral(req.Path),
		"__REPORT__", pyLiteral(reportURL),
		"__SIZE__", strconv.Itoa(req.Size),
		"__PER_RANGE__", strconv.Itoa(req.PerRange),
		"__RANGES__", pyLiteral(cleanIpCloudflareRanges),
		"__IPS__", pyLiteral(req.Ips),
	).Replace(cleanIpScript), nil
}

func (s *CleanIpService) Reports(token string) ([]CleanIpReport, error) {
	cleanIpJobsMu.Lock()
	defer cleanIpJobsMu.Unlock()
	job, err := cleanIpJobFor(token)
	if err != nil {
		return nil, err
	}
	return append([]CleanIpReport{}, job.reports...), nil
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
	cleanIpJobsMu.Lock()
	_, err := cleanIpJobFor(token)
	cleanIpJobsMu.Unlock()
	if err != nil {
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

	cleanIpJobsMu.Lock()
	defer cleanIpJobsMu.Unlock()
	job, err := cleanIpJobFor(token)
	if err != nil {
		return nil, err
	}
	job.reports = append(job.reports, report)
	if len(job.reports) > cleanIpMaxReports {
		job.reports = job.reports[len(job.reports)-cleanIpMaxReports:]
	}
	return &report, nil
}
