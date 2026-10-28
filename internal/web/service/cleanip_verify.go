package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// Outgoing connections to many IPs can read as port scanning to a hosting provider (#6950),
// so the server checks a short list, only on the CDN's own edges, within an hourly budget.
const (
	cleanIpOk            = "ok"
	cleanIpRestricted    = "restricted"
	cleanIpUnreachable   = "unreachable"
	cleanIpSkipped       = "skipped"
	cleanIpLimited       = "limited"
	cleanIpVerifyMax     = 50
	cleanIpVerifyWorkers = 4
	cleanIpVerifyHourly  = 300
)

// CleanIpVerdict is what the server saw on one IP: ok, restricted (Cloudflare error 1xxx such as
// 1034), unreachable, skipped (not one of the CDN's edges, never contacted) or limited (budget spent).
type CleanIpVerdict struct {
	Ip     string `json:"ip" example:"104.16.1.1"`
	Status string `json:"status" example:"ok"`
}

type CleanIpVerifyRequest struct {
	Host string   `json:"host" example:"cdn.example.com"`
	Cdn  string   `json:"cdn" example:"cloudflare"`
	Ips  []string `json:"ips" example:"[\"104.16.1.1\"]"`
}

type cleanIpChecked struct {
	status string
	at     time.Time
}

var (
	cleanIpProbe = probeCleanIp

	cleanIpVerifyMu sync.Mutex
	cleanIpVerified = map[string]cleanIpChecked{}
)

func resetCleanIpVerifyBudget() {
	cleanIpVerifyMu.Lock()
	defer cleanIpVerifyMu.Unlock()
	cleanIpVerified = map[string]cleanIpChecked{}
}

// cleanIpCdnPrefixes is every range a CDN publishes, opt-in groups included; nil for an unknown id.
func cleanIpCdnPrefixes(id string) []netip.Prefix {
	for _, cdn := range cleanIpCdns {
		if cdn.Id != id {
			continue
		}
		prefixes := []netip.Prefix{}
		for _, g := range cdn.Groups {
			for _, r := range g.Ranges {
				if p, err := netip.ParsePrefix(r); err == nil {
					prefixes = append(prefixes, p)
				}
			}
		}
		return prefixes
	}
	return nil
}

func inCleanIpPrefixes(ip string, prefixes []netip.Prefix) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// claimCleanIpProbes answers IPs checked this hour from memory and reserves budget for the
// rest; it returns the IPs to probe now. Forgets checks older than an hour.
func claimCleanIpProbes(out []CleanIpVerdict, todo []int) []int {
	cleanIpVerifyMu.Lock()
	defer cleanIpVerifyMu.Unlock()
	now := cleanIpNow()
	for ip, c := range cleanIpVerified {
		if now.Sub(c.at) >= time.Hour {
			delete(cleanIpVerified, ip)
		}
	}
	var probe []int
	for _, i := range todo {
		if c, ok := cleanIpVerified[out[i].Ip]; ok {
			if c.status != "" {
				out[i].Status = c.status
				continue
			}
		} else if len(cleanIpVerified) < cleanIpVerifyHourly {
			cleanIpVerified[out[i].Ip] = cleanIpChecked{at: now}
		} else {
			out[i].Status = cleanIpLimited
			continue
		}
		probe = append(probe, i)
	}
	return probe
}

// settleCleanIpProbe records a verdict; a probe cut short by the caller leaving keeps its slot
// in the budget (the connection was made) but no verdict.
func settleCleanIpProbe(ip, status string, cancelled bool) {
	cleanIpVerifyMu.Lock()
	defer cleanIpVerifyMu.Unlock()
	if c, ok := cleanIpVerified[ip]; ok && !cancelled {
		c.status = status
		cleanIpVerified[ip] = c
	}
}

// classifyCleanIpResponse reads the start of an HTTP reply through one IP. Only Cloudflare's own
// 1xxx error pages mark the IP bad; any reply from the site, even an error, proves it serves it.
func classifyCleanIpResponse(raw []byte) string {
	if !bytes.HasPrefix(raw, []byte("HTTP/")) {
		return cleanIpUnreachable
	}
	head, body, _ := bytes.Cut(raw, []byte("\r\n\r\n"))
	statusLine, _, _ := bytes.Cut(head, []byte("\r\n"))
	fields := bytes.Fields(statusLine)
	if len(fields) < 2 {
		return cleanIpUnreachable
	}
	status, _ := strconv.Atoi(string(fields[1]))
	if status == 403 && bytes.HasPrefix(bytes.TrimSpace(body), []byte("error code: 1")) {
		return cleanIpRestricted
	}
	return cleanIpOk
}

func probeCleanIp(ctx context.Context, host, ip string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
	if err != nil {
		return cleanIpUnreachable
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	conn := tls.Client(raw, &tls.Config{ServerName: host, NextProtos: []string{"http/1.1"}})
	if err := conn.HandshakeContext(ctx); err != nil {
		return cleanIpUnreachable
	}
	req := "GET / HTTP/1.1\r\nHost: " + host + "\r\nUser-Agent: Mozilla/5.0\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		return cleanIpUnreachable
	}
	buf := make([]byte, 4096)
	n, _ := io.ReadAtLeast(conn, buf, 64)
	return classifyCleanIpResponse(buf[:n])
}

// VerifyIps checks from the panel's server that each IP really serves host. The scan in the
// browser only sees that an IP answers TLS, which every Cloudflare edge does.
func (s *CleanIpService) VerifyIps(ctx context.Context, req CleanIpVerifyRequest) ([]CleanIpVerdict, error) {
	host := req.Host
	if req.Cdn == "" {
		req.Cdn = cleanIpCdns[0].Id
	}
	prefixes := cleanIpCdnPrefixes(req.Cdn)
	if !cleanIpHostRe.MatchString(host) || prefixes == nil {
		return nil, errCleanIpBadRequest
	}
	out := []CleanIpVerdict{}
	var todo []int
	seen := map[string]bool{}
	for _, raw := range req.Ips {
		ip, ok := validIPv4(raw)
		if !ok || seen[ip] || len(out) >= cleanIpVerifyMax {
			continue
		}
		seen[ip] = true
		out = append(out, CleanIpVerdict{Ip: ip, Status: cleanIpSkipped})
		if inCleanIpPrefixes(ip, prefixes) {
			todo = append(todo, len(out)-1)
		}
	}

	sem := make(chan struct{}, cleanIpVerifyWorkers)
	var wg sync.WaitGroup
	for _, i := range claimCleanIpProbes(out, todo) {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			status := cleanIpProbe(ctx, host, out[i].Ip)
			settleCleanIpProbe(out[i].Ip, status, ctx.Err() != nil)
			out[i].Status = status
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
