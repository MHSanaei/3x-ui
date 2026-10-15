package psiphon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	defaultProbeTimeout = 5 * time.Second
	probeTarget204      = "http://cp.cloudflare.com/generate_204"
	probeTargetFallback = "http://www.google.com/generate_204"
	probeTraceTarget    = "https://www.cloudflare.com/cdn-cgi/trace"
	probeIPFallback     = "https://api.ipify.org"
)

// Probe tests connectivity, measures latency, and identifies the exit IP and country.
func Probe(ctx context.Context, socksPort int) ProbeResult {
	if socksPort <= 0 || socksPort > 65535 {
		return ProbeResult{Error: "invalid socks port"}
	}
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return ProbeResult{Error: fmt.Sprintf("socks5 dialer error: %v", err)}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if cd, ok := dialer.(proxy.ContextDialer); ok {
				return cd.DialContext(ctx, network, addr)
			}
			return dialer.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultProbeTimeout,
	}

	latency, err := measureLatency(ctx, client, probeTarget204)
	if err != nil {
		latency, err = measureLatency(ctx, client, probeTargetFallback)
	}
	if err != nil {
		return ProbeResult{Error: fmt.Sprintf("latency probe failed: %v", err)}
	}

	exitIP, exitCountry := lookupExitIdentity(ctx, client)

	return ProbeResult{
		OK:          true,
		LatencyMs:   latency,
		ExitIP:      exitIP,
		ExitCountry: exitCountry,
	}
}

func measureLatency(ctx context.Context, client *http.Client, target string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "3x-ui-probe")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	elapsed := time.Since(start)
	return int(elapsed.Milliseconds()), nil
}

func lookupExitIdentity(ctx context.Context, client *http.Client) (string, string) {
	traceReq, err := http.NewRequestWithContext(ctx, http.MethodGet, probeTraceTarget, nil)
	if err == nil {
		traceReq.Header.Set("User-Agent", "3x-ui-probe")
		if resp, err := client.Do(traceReq); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				ip, loc := parseTrace(string(body))
				if ip != "" {
					return ip, loc
				}
			}
		}
	}

	ipReq, err := http.NewRequestWithContext(ctx, http.MethodGet, probeIPFallback, nil)
	if err == nil {
		ipReq.Header.Set("User-Agent", "3x-ui-probe")
		if resp, err := client.Do(ipReq); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
				trimmed := strings.TrimSpace(string(body))
				if parsed := net.ParseIP(trimmed); parsed != nil {
					return parsed.String(), ""
				}
			}
		}
	}
	return "", ""
}

func parseTrace(body string) (string, string) {
	var ip, loc string
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "ip":
			if parsed := net.ParseIP(value); parsed != nil {
				ip = parsed.String()
			}
		case "loc":
			loc = strings.ToUpper(strings.TrimSpace(value))
		}
	}
	return ip, loc
}
