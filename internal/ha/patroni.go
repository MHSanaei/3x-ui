package ha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PatroniClient reads the local Patroni REST API. The panel never drives
// PostgreSQL itself: election, promotion, demotion and rewind are Patroni's.
type PatroniClient struct {
	base string
	http *http.Client
}

// NewPatroniClient builds a client; a nil hc gets a 3 s timeout.
func NewPatroniClient(base string, hc *http.Client) *PatroniClient {
	if hc == nil {
		hc = &http.Client{Timeout: 3 * time.Second}
	}
	return &PatroniClient{base: strings.TrimRight(base, "/"), http: hc}
}

type patroniStatus struct {
	State string `json:"state"`
	Role  string `json:"role"`
}

// IsLeader reports a running primary. Patroni before 3.x said "master";
// a standby_leader leads a standby cluster and does not write.
func (c *PatroniClient) IsLeader(ctx context.Context) (bool, error) {
	var st patroniStatus
	// Patroni answers 503 with the same body on a non-leader, so parse any code.
	if err := c.getJSON(ctx, "/patroni", &st); err != nil {
		return false, err
	}
	if st.Role == "" {
		return false, fmt.Errorf("patroni: no role in the response")
	}
	return (st.Role == "primary" || st.Role == "master") && st.State == "running", nil
}

// PatroniTimings are the fencing-relevant keys of the dynamic configuration.
type PatroniTimings struct {
	TTL          int `json:"ttl"`
	LoopWait     int `json:"loop_wait"`
	RetryTimeout int `json:"retry_timeout"`
}

// Config reads GET /config.
func (c *PatroniClient) Config(ctx context.Context) (PatroniTimings, error) {
	var dc PatroniTimings
	if err := c.getJSON(ctx, "/config", &dc); err != nil {
		return PatroniTimings{}, err
	}
	return dc, nil
}

// Check lists broken fencing invariants: Patroni must demote before its key
// expires, and the panel (fenceDeadline) must fence before a standby may promote.
func (dc PatroniTimings) Check(fenceDeadline time.Duration) []string {
	if dc.TTL <= 0 {
		return []string{"patroni: ttl missing from the dynamic configuration"}
	}
	var out []string
	if dc.LoopWait+2*dc.RetryTimeout > dc.TTL {
		out = append(out, fmt.Sprintf("patroni: loop_wait(%d) + 2*retry_timeout(%d) > ttl(%d), the leader may not demote in time",
			dc.LoopWait, dc.RetryTimeout, dc.TTL))
	}
	if ttl := time.Duration(dc.TTL) * time.Second; fenceDeadline >= ttl {
		out = append(out, fmt.Sprintf("panel: fence_after + interval + probe timeout = %s >= Patroni ttl %s, the panel may fence after a promotion",
			fenceDeadline, ttl))
	}
	return out
}

func (c *PatroniClient) getJSON(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("patroni %s: %w", path, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("patroni %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("patroni %s: read: %w", path, err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("patroni %s: status %d, not JSON: %w", path, resp.StatusCode, err)
	}
	return nil
}
