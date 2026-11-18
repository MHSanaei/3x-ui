package ha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// PeerPath is the peer ping, guarded by the cluster secret.
const PeerPath = "/ha/peer"

// Ping is the peer ping body: identity, role and the public URLs that clients
// are redirected to while this peer is master.
type Ping struct {
	Code     string `json:"code"`
	Role     Role   `json:"role"`
	PanelURL string `json:"panel_url,omitempty"`
	SubURL   string `json:"sub_url,omitempty"`
}

// PeerStatus is one peer's probe result; OK means it counts towards quorum.
type PeerStatus struct {
	Code     string
	OK       bool
	Role     Role
	PanelURL string
	SubURL   string
}

// ParsePeers parses "nl=https://a:2097,pl=https://b:2097".
func ParsePeers(s string) (map[string]string, error) {
	out := map[string]string{}
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		code, url, ok := strings.Cut(part, "=")
		code, url = strings.TrimSpace(code), strings.TrimRight(strings.TrimSpace(url), "/")
		if !ok || code == "" || url == "" {
			return nil, fmt.Errorf("peer %q: want code=url", part)
		}
		if _, dup := out[code]; dup {
			return nil, fmt.Errorf("peer %q listed twice", code)
		}
		out[code] = url
	}
	return out, nil
}

// PeerCodes returns the peer codes sorted.
func PeerCodes(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for c := range m {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}

// Prober pings the peer panels in parallel (the HTTP quorum channel).
type Prober struct {
	Peers  map[string]string
	Secret string
	HTTP   *http.Client // nil means a 1.5 s timeout
}

// Probe returns one result per peer, ordered by code.
func (p *Prober) Probe(ctx context.Context) []PeerStatus {
	hc := p.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 1500 * time.Millisecond}
	}
	codes := PeerCodes(p.Peers)
	out := make([]PeerStatus, len(codes))
	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Go(func() { out[i] = p.probeOne(ctx, hc, code, p.Peers[code]) })
	}
	wg.Wait()
	return out
}

func (p *Prober) probeOne(ctx context.Context, hc *http.Client, code, base string) PeerStatus {
	res := PeerStatus{Code: code}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+PeerPath, nil)
	if err != nil {
		return res
	}
	req.Header.Set("Authorization", "Bearer "+p.Secret)
	resp, err := hc.Do(req)
	if err != nil {
		return res
	}
	defer func() {
		// Drain so the keep-alive connection is reused across rounds.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return res
	}
	var ping Ping
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&ping); err != nil {
		return res
	}
	// A mixed-up address or a balancer answering for another machine is no vote.
	if ping.Code != code {
		return res
	}
	return PeerStatus{Code: code, OK: true, Role: ping.Role, PanelURL: ping.PanelURL, SubURL: ping.SubURL}
}
