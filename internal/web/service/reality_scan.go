package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

const (
	realityScanTimeout     = 10 * time.Second
	realityDiscoverTimeout = 4 * time.Second
	realityScanConcurrency = 32
	realityDiscoverMaxIPs  = 256
	realityScanMaxTotal    = 512
)

var defaultRealityScanCandidates = []string{
	"www.cloudflare.com:443",
	"www.microsoft.com:443",
	"www.amazon.com:443",
	"aws.amazon.com:443",
	"www.samsung.com:443",
	"www.nvidia.com:443",
	"www.amd.com:443",
	"www.intel.com:443",
	"www.sony.com:443",
	"dl.google.com:443",
}

type RealityScanResult struct {
	Target      string   `json:"target" example:"www.cloudflare.com:443"`
	Host        string   `json:"host" example:"www.cloudflare.com"`
	IP          string   `json:"ip" example:"104.16.124.96"`
	Port        int      `json:"port" example:"443"`
	Feasible    bool     `json:"feasible" example:"true"`
	TLS13       bool     `json:"tls13" example:"true"`
	TLSVersion  string   `json:"tlsVersion" example:"1.3"`
	H2          bool     `json:"h2" example:"true"`
	ALPN        string   `json:"alpn" example:"h2"`
	X25519      bool     `json:"x25519" example:"true"`
	CurveID     string   `json:"curveID" example:"X25519"`
	CertValid   bool     `json:"certValid" example:"true"`
	CertSubject string   `json:"certSubject" example:"cloudflare.com"`
	CertIssuer  string   `json:"certIssuer" example:"Google Trust Services"`
	NotAfter    string   `json:"notAfter" example:"2026-08-01T00:00:00Z"`
	ServerNames []string `json:"serverNames"`
	LatencyMs   int      `json:"latencyMs" example:"180"`
	Reason      string   `json:"reason" example:""`
}

type realityProbeTask struct {
	dialHost string
	port     int
	sni      string
	timeout  time.Duration
	bulk     bool
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return "unknown"
	}
}

func realityCurveName(id tls.CurveID) string {
	switch id {
	case tls.X25519:
		return "X25519"
	case tls.X25519MLKEM768:
		return "X25519MLKEM768"
	case tls.CurveP256:
		return "P-256"
	case tls.CurveP384:
		return "P-384"
	case tls.CurveP521:
		return "P-521"
	case 0:
		return ""
	default:
		return fmt.Sprintf("0x%04x", uint16(id))
	}
}

func filterUsableSANs(dnsNames []string) []string {
	out := make([]string, 0, len(dnsNames))
	for _, n := range dnsNames {
		n = strings.TrimSpace(n)
		if n == "" || strings.HasPrefix(n, "*.") {
			continue
		}
		out = append(out, n)
	}
	return out
}

func firstUsableName(leaf *x509.Certificate) string {
	cn := strings.TrimSpace(leaf.Subject.CommonName)
	if cn != "" && !strings.HasPrefix(cn, "*.") {
		return cn
	}
	for _, n := range leaf.DNSNames {
		n = strings.TrimSpace(n)
		if n != "" && !strings.HasPrefix(n, "*.") {
			return n
		}
	}
	return ""
}

func splitRealityTarget(target string) (string, int, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", 0, common.NewError("target is required")
	}
	host, portStr := target, "443"
	if h, p, err := net.SplitHostPort(target); err == nil {
		host, portStr = h, p
	}
	host, err := netsafe.NormalizeHost(host)
	if err != nil {
		return "", 0, common.NewError("invalid target host: ", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, common.NewError("invalid target port")
	}
	return host, port, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func enumerateCIDR(cidr string, max int) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return nil, err
	}
	ips := make([]string, 0, max)
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
		if len(ips) >= max {
			break
		}
	}
	return ips, nil
}

func (s *ServerService) probeRealityAddr(dialHost string, port int, sni string, timeout time.Duration) *RealityScanResult {
	addr := net.JoinHostPort(dialHost, strconv.Itoa(port))
	res := &RealityScanResult{Port: port}
	if net.ParseIP(dialHost) != nil {
		res.IP = dialHost
	}
	if sni != "" {
		res.Host = sni
		res.Target = net.JoinHostPort(sni, strconv.Itoa(port))
	} else {
		res.Host = dialHost
		res.Target = addr
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	conn, err := netsafe.SSRFGuardedDialContext(ctx, "tcp", addr)
	if err != nil {
		res.Reason = "connection failed: " + err.Error()
		return res
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	cfg := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2", "http/1.1"},
		CurvePreferences:   []tls.CurveID{tls.X25519, tls.X25519MLKEM768},
		MinVersion:         tls.VersionTLS12,
	}
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		res.Reason = "TLS handshake failed: " + err.Error()
		return res
	}
	res.LatencyMs = int(time.Since(start).Milliseconds())

	st := tlsConn.ConnectionState()
	res.TLS13 = st.Version == tls.VersionTLS13
	res.TLSVersion = tlsVersionName(st.Version)
	res.ALPN = st.NegotiatedProtocol
	res.H2 = st.NegotiatedProtocol == "h2"
	res.CurveID = realityCurveName(st.CurveID)
	res.X25519 = st.CurveID == tls.X25519 || st.CurveID == tls.X25519MLKEM768

	verifyHost := sni
	if len(st.PeerCertificates) > 0 {
		leaf := st.PeerCertificates[0]
		res.CertSubject = leaf.Subject.CommonName
		if res.CertSubject == "" && len(leaf.DNSNames) > 0 {
			res.CertSubject = leaf.DNSNames[0]
		}
		if len(leaf.Issuer.Organization) > 0 {
			res.CertIssuer = leaf.Issuer.Organization[0]
		} else {
			res.CertIssuer = leaf.Issuer.CommonName
		}
		res.NotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
		res.ServerNames = filterUsableSANs(leaf.DNSNames)

		if sni == "" {
			if discovered := firstUsableName(leaf); discovered != "" {
				res.Host = discovered
				res.Target = net.JoinHostPort(discovered, strconv.Itoa(port))
				verifyHost = discovered
			}
		}

		if verifyHost != "" {
			opts := x509.VerifyOptions{DNSName: verifyHost, Intermediates: x509.NewCertPool()}
			for _, c := range st.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			if _, verr := leaf.Verify(opts); verr == nil {
				res.CertValid = true
			} else {
				res.Reason = "certificate not trusted: " + verr.Error()
			}
		} else {
			res.Reason = "no usable domain in certificate"
		}
	} else {
		res.Reason = "no certificate presented"
	}

	res.Feasible = res.TLS13 && res.H2 && res.X25519 && res.CertValid
	if !res.Feasible && res.Reason == "" {
		switch {
		case !res.TLS13:
			res.Reason = "server does not negotiate TLS 1.3"
		case !res.H2:
			res.Reason = "server does not negotiate HTTP/2 (h2)"
		case !res.X25519:
			res.Reason = "server did not use X25519 key exchange"
		}
	}
	return res
}

func (s *ServerService) probeRealityTarget(host string, port int) *RealityScanResult {
	return s.probeRealityAddr(host, port, host, realityScanTimeout)
}

func (s *ServerService) ScanRealityTarget(target string) (*RealityScanResult, error) {
	host, port, err := splitRealityTarget(target)
	if err != nil {
		return nil, err
	}
	return s.probeRealityTarget(host, port), nil
}

func (s *ServerService) ScanRealityTargets(targetsCSV string) ([]*RealityScanResult, error) {
	var tokens []string
	for _, raw := range strings.Split(targetsCSV, ",") {
		if t := strings.TrimSpace(raw); t != "" {
			tokens = append(tokens, t)
		}
	}
	if len(tokens) == 0 {
		tokens = append(tokens, defaultRealityScanCandidates...)
	}

	var tasks []realityProbeTask
	var invalid []*RealityScanResult
	for _, token := range tokens {
		if len(tasks) >= realityScanMaxTotal {
			break
		}
		if strings.Contains(token, "/") {
			ips, err := enumerateCIDR(token, realityDiscoverMaxIPs)
			if err != nil {
				invalid = append(invalid, &RealityScanResult{Target: token, Reason: "invalid CIDR: " + err.Error()})
				continue
			}
			for _, ip := range ips {
				if len(tasks) >= realityScanMaxTotal {
					break
				}
				tasks = append(tasks, realityProbeTask{dialHost: ip, port: 443, timeout: realityDiscoverTimeout, bulk: true})
			}
			continue
		}
		host, port, err := splitRealityTarget(token)
		if err != nil {
			invalid = append(invalid, &RealityScanResult{Target: token, Reason: err.Error()})
			continue
		}
		if net.ParseIP(host) != nil {
			tasks = append(tasks, realityProbeTask{dialHost: host, port: port, timeout: realityDiscoverTimeout})
		} else {
			tasks = append(tasks, realityProbeTask{dialHost: host, port: port, sni: host, timeout: realityScanTimeout})
		}
	}

	probed := make([]*RealityScanResult, len(tasks))
	sem := make(chan struct{}, realityScanConcurrency)
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, tk realityProbeTask) {
			defer wg.Done()
			defer func() { <-sem }()
			r := s.probeRealityAddr(tk.dialHost, tk.port, tk.sni, tk.timeout)
			if tk.bulk && r.TLSVersion == "" {
				return
			}
			probed[idx] = r
		}(i, task)
	}
	wg.Wait()

	results := dedupRealityResults(append(probed, invalid...))
	sortRealityResults(results)
	return results, nil
}

func dedupRealityResults(results []*RealityScanResult) []*RealityScanResult {
	best := make(map[string]*RealityScanResult)
	order := make([]string, 0, len(results))
	for _, r := range results {
		if r == nil {
			continue
		}
		if ex, ok := best[r.Target]; !ok {
			best[r.Target] = r
			order = append(order, r.Target)
		} else if betterRealityResult(r, ex) {
			best[r.Target] = r
		}
	}
	out := make([]*RealityScanResult, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

func betterRealityResult(a, b *RealityScanResult) bool {
	if a.Feasible != b.Feasible {
		return a.Feasible
	}
	return a.LatencyMs > 0 && (b.LatencyMs == 0 || a.LatencyMs < b.LatencyMs)
}

func sortRealityResults(results []*RealityScanResult) {
	slices.SortStableFunc(results, func(a, b *RealityScanResult) int {
		if a.Feasible != b.Feasible {
			if a.Feasible {
				return -1
			}
			return 1
		}
		return a.LatencyMs - b.LatencyMs
	})
}
