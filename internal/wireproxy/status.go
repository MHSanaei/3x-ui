package wireproxy

import (
	"context"
	"encoding/json"
	"fmt"
)

// Status mirrors the fields of `warpwp --status-json` this package's callers
// actually use -- warpwp's own output carries more (cron/timer/logs/cache
// details); json.Unmarshal drops what isn't mapped here rather than this
// package re-deriving a second source of truth for it.
type Status struct {
	ManagerVersion   string `json:"manager_version"`
	NativeVersion    string `json:"native_version"`
	Healthy          bool   `json:"healthy"`
	Scheduler        string `json:"scheduler"`
	Installed        bool   `json:"installed"`
	ManagerInstalled bool   `json:"manager_installed"`
	NativeInstalled  bool   `json:"native_installed"`
	Service          struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Active bool   `json:"active"`
	} `json:"service"`
	Socks5 struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Listening bool   `json:"listening"`
	} `json:"socks5"`
	Warp struct {
		Endpoint     string `json:"endpoint"`
		EndpointPort string `json:"endpoint_port"`
		IP           string `json:"ip"`
		Colo         string `json:"colo"`
		Loc          string `json:"loc"`
		Status       string `json:"status"`
		On           bool   `json:"on"`
	} `json:"warp"`
	Selection struct {
		TimeTotal        string `json:"time_total"`
		Colo             string `json:"colo"`
		Loc              string `json:"loc"`
		CheckedAt        int64  `json:"checked_at"`
		ProbeLossPercent string `json:"probe_loss_percent"`
		Stable           string `json:"stable"`
		Scanner          string `json:"scanner"`
	} `json:"selection"`
	RoutingGuard struct {
		// Danger means warpwp found dangerous system-wide WARP routing (a
		// stray wg-quick@warp, warp-svc, etc.) still active -- see this
		// package's own doc comment for why that specifically matters. The
		// UI surfaces this prominently: it means something outside warpwp's
		// own management could be breaking the VPS's inbound connectivity.
		Danger bool `json:"danger"`
	} `json:"routing_guard"`
}

// GetStatus reports the current state via warpwp's own --status-json. Not
// installed short-circuits to a zero Status rather than trying to run a
// binary that isn't there, mirroring psiphon.IsConfigured-style gating.
func GetStatus(ctx context.Context) (Status, error) {
	if !IsInstalled() {
		return Status{}, nil
	}
	out, err := run(ctx, "--status-json")
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out)
}

// parseStatus is split out from GetStatus so the JSON shape can be tested
// against a captured real warpwp --status-json sample without shelling out
// to anything.
func parseStatus(data []byte) (Status, error) {
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return Status{}, fmt.Errorf("parsing warpwp --status-json: %w", err)
	}
	return status, nil
}
