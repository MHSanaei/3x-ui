package psiphon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/proxy"
)

// notice is one line of Psiphon's JSON-lines notice log; unread fields are dropped on decode.
type notice struct {
	NoticeType string          `json:"noticeType"`
	Data       json.RawMessage `json:"data"`
}

// TunnelStatus summarizes the process's own view of its tunnel, read from its
// notice log -- cheap enough for every status poll, unlike CurrentExit.
type TunnelStatus struct {
	Connected    bool   `json:"connected"`
	TunnelCount  int    `json:"tunnelCount"`
	ServerRegion string `json:"serverRegion,omitempty"`
	ClientRegion string `json:"clientRegion,omitempty"`
}

// CurrentTunnel scans NoticesPath for the latest Tunnels/ConnectedServerRegion/
// ClientRegion notices. A missing log returns a zero TunnelStatus, not an error.
func CurrentTunnel() (TunnelStatus, error) {
	f, err := os.Open(NoticesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return TunnelStatus{}, nil
		}
		return TunnelStatus{}, err
	}
	defer f.Close()

	var status TunnelStatus
	scanner := bufio.NewScanner(f)
	// Notice lines are small JSON objects; 1 MiB comfortably covers even an
	// unusually large one without letting a corrupt log OOM this scan.
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	for scanner.Scan() {
		var n notice
		if err := json.Unmarshal(scanner.Bytes(), &n); err != nil {
			continue // one malformed line shouldn't hide every notice around it
		}
		switch n.NoticeType {
		case "Tunnels":
			var d struct {
				Count int `json:"count"`
			}
			if json.Unmarshal(n.Data, &d) == nil {
				status.TunnelCount = d.Count
				status.Connected = d.Count > 0
			}
		case "ConnectedServerRegion":
			var d struct {
				ServerRegion string `json:"serverRegion"`
			}
			if json.Unmarshal(n.Data, &d) == nil {
				status.ServerRegion = d.ServerRegion
			}
		case "ClientRegion":
			var d struct {
				Region string `json:"region"`
			}
			if json.Unmarshal(n.Data, &d) == nil {
				status.ClientRegion = d.Region
			}
		}
	}
	return status, scanner.Err()
}

// ExitInfo is a live, network-verified view of what answers behind the SOCKS
// proxy, distinct from TunnelStatus, which only reports Psiphon's own claim.
type ExitInfo struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
}

// socksHTTPClient builds an http.Client that dials out through the managed
// SOCKS proxy. Mirrors tor.socksHTTPClient.
func socksHTTPClient(timeout time.Duration) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", SocksPort), nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	ctxDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer does not support context")
	}
	return &http.Client{
		Transport: &http.Transport{DialContext: ctxDialer.DialContext},
		Timeout:   timeout,
	}, nil
}

// CurrentExit dials out through the managed SOCKS proxy to confirm what is
// really reachable -- a real network round-trip, the same split internal/tor draws between IsRunning and CurrentIP.
func CurrentExit(ctx context.Context) (ExitInfo, error) {
	client, err := socksHTTPClient(30 * time.Second)
	if err != nil {
		return ExitInfo{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipinfo.io/json", nil)
	if err != nil {
		return ExitInfo{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return ExitInfo{}, err
	}
	defer resp.Body.Close()
	var parsed struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&parsed); err != nil {
		return ExitInfo{}, fmt.Errorf("decoding ipinfo.io response: %w", err)
	}
	if parsed.IP == "" {
		return ExitInfo{}, fmt.Errorf("ipinfo.io response had no IP")
	}
	return ExitInfo{IP: parsed.IP, Country: parsed.Country}, nil
}
