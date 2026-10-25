package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	cleanIpOk          = "ok"
	cleanIpRestricted  = "restricted"
	cleanIpUnreachable = "unreachable"
	cleanIpVerifyMax   = 200
)

// CleanIpVerdict is what the panel's server saw on one IP: ok, restricted (Cloudflare refuses
// to serve the domain there, error 1xxx such as 1034) or unreachable from the server.
type CleanIpVerdict struct {
	Ip     string `json:"ip" example:"104.16.1.1"`
	Status string `json:"status" example:"ok"`
}

type CleanIpVerifyRequest struct {
	Host string   `json:"host" example:"cdn.example.com"`
	Ips  []string `json:"ips" example:"[\"104.16.1.1\"]"`
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
	if !cleanIpHostRe.MatchString(host) {
		return nil, errCleanIpBadRequest
	}
	var ips []string
	for _, raw := range req.Ips {
		if ip, ok := validIPv4(raw); ok && len(ips) < cleanIpVerifyMax {
			ips = append(ips, ip)
		}
	}
	out := make([]CleanIpVerdict, len(ips))
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	for i, ip := range ips {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = CleanIpVerdict{Ip: ip, Status: probeCleanIp(ctx, host, ip)}
		}()
	}
	wg.Wait()
	return out, nil
}
