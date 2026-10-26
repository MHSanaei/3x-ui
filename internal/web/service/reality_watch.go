package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// The REALITY watch keeps an inbound's target alive: it checks the target from this server
// and, after realityWatchFailLimit failed checks in a row, moves the inbound to the first
// spare that passes both the server check and a real REALITY connection.

const (
	realityWatchSetting   = "realityWatch"
	realityWatchFailLimit = 3
	realityWatchMaxSpares = 10
	realityWatchMaxEvents = 20
)

var (
	errRealityWatchBadSpare    = errors.New("reality watch: invalid spare target")
	errRealityWatchNotReality  = errors.New("reality watch: not a REALITY inbound")
	errRealityWatchTargetFails = errors.New("reality watch: target fails the check")
	errRealityWatchNoSpare     = errors.New("reality watch: no spare target works")

	realityWatchProbe = func(s *ServerService, target, sni string, xver int) *RealityScanResult {
		res, err := s.ScanRealityTarget(target, sni, xver, false)
		if err != nil {
			return &RealityScanResult{Target: target, Reason: err.Error()}
		}
		return res
	}
	realityWatchRealTest = xrayRealityRealTest
)

// RealityWatchEntry is what the admin sets per inbound: spare targets (host:port) in order
// of preference, and whether a dead target is replaced without asking.
type RealityWatchEntry struct {
	Auto   bool     `json:"auto" example:"true"`
	Spares []string `json:"spares" example:"[\"www.example.com:443\"]"`
}

// RealityWatchEvent is one switch (To set) or a failed attempt to find a working spare.
type RealityWatchEvent struct {
	At     int64  `json:"at" example:"1793000000"`
	From   string `json:"from" example:"old.example.com:443"`
	To     string `json:"to" example:"www.example.com:443"`
	Reason string `json:"reason" example:"TLS handshake failed: EOF"`
}

type RealityWatchStatus struct {
	InboundId int                 `json:"inboundId" example:"3"`
	Target    string              `json:"target" example:"old.example.com:443"`
	Auto      bool                `json:"auto" example:"true"`
	Spares    []string            `json:"spares" example:"[\"www.example.com:443\"]"`
	CheckedAt int64               `json:"checkedAt" example:"1793000000"`
	Ok        bool                `json:"ok" example:"true"`
	Fails     int                 `json:"fails" example:"0"`
	Reason    string              `json:"reason" example:""`
	Events    []RealityWatchEvent `json:"events"`
}

type realityWatchState struct {
	checkedAt time.Time
	ok        bool
	fails     int
	reason    string
	events    []RealityWatchEvent
}

var (
	realityWatchMu     sync.Mutex
	realityWatchStates = map[int]*realityWatchState{}
)

func resetRealityWatchState() {
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	realityWatchStates = map[int]*realityWatchState{}
}

type RealityWatchService struct {
	inboundService InboundService
	settingService SettingService
	serverService  ServerService
	xrayService    XrayService
}

type realitySettingsView struct {
	Target      string   `json:"target"`
	Dest        string   `json:"dest"`
	ServerNames []string `json:"serverNames"`
	Xver        int      `json:"xver"`
}

// realityOf reads an inbound's REALITY target (older configs call it dest) and SNIs.
func realityOf(ib *model.Inbound) (realitySettingsView, bool) {
	var stream struct {
		Security        string              `json:"security"`
		RealitySettings realitySettingsView `json:"realitySettings"`
	}
	if json.Unmarshal([]byte(ib.StreamSettings), &stream) != nil || stream.Security != "reality" {
		return realitySettingsView{}, false
	}
	rs := stream.RealitySettings
	if rs.Target == "" {
		rs.Target = rs.Dest
	}
	return rs, rs.Target != ""
}

// normalizeRealityTarget is host:port in lower case, :443 when no port is given.
func normalizeRealityTarget(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	host, port, err := splitRealityTarget(raw)
	if err != nil || strings.ContainsAny(raw, " \"'`;") {
		return "", errRealityWatchBadSpare
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func (s *RealityWatchService) loadConfig() map[int]RealityWatchEntry {
	out := map[int]RealityWatchEntry{}
	raw, err := s.settingService.getString(realityWatchSetting)
	if err != nil || raw == "" {
		return out
	}
	var byKey map[string]RealityWatchEntry
	if json.Unmarshal([]byte(raw), &byKey) != nil {
		return out
	}
	for k, v := range byKey {
		if id, err := strconv.Atoi(k); err == nil {
			out[id] = v
		}
	}
	return out
}

func (s *RealityWatchService) storeConfig(cfg map[int]RealityWatchEntry) error {
	byKey := make(map[string]RealityWatchEntry, len(cfg))
	for id, v := range cfg {
		byKey[strconv.Itoa(id)] = v
	}
	b, err := json.Marshal(byKey)
	if err != nil {
		return err
	}
	return s.settingService.setString(realityWatchSetting, string(b))
}

// SaveConfig sets an inbound's spares and auto switch; the current target is never a spare.
func (s *RealityWatchService) SaveConfig(inboundId int, entry RealityWatchEntry) error {
	ib, err := s.inboundService.GetInbound(inboundId)
	if err != nil {
		return err
	}
	rs, ok := realityOf(ib)
	if !ok {
		return errRealityWatchNotReality
	}
	current, _ := normalizeRealityTarget(rs.Target)
	spares := []string{}
	for _, raw := range entry.Spares {
		target, err := normalizeRealityTarget(raw)
		if err != nil {
			return err
		}
		if target != current && !containsString(spares, target) && len(spares) < realityWatchMaxSpares {
			spares = append(spares, target)
		}
	}
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	cfg := s.loadConfig()
	cfg[inboundId] = RealityWatchEntry{Auto: entry.Auto, Spares: spares}
	return s.storeConfig(cfg)
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *RealityWatchService) Status(inboundId int) RealityWatchStatus {
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	return s.statusLocked(inboundId, s.loadConfig()[inboundId])
}

func (s *RealityWatchService) statusLocked(inboundId int, entry RealityWatchEntry) RealityWatchStatus {
	st := RealityWatchStatus{InboundId: inboundId, Auto: entry.Auto, Spares: entry.Spares, Events: []RealityWatchEvent{}}
	if st.Spares == nil {
		st.Spares = []string{}
	}
	if ib, err := s.inboundService.GetInbound(inboundId); err == nil {
		if rs, ok := realityOf(ib); ok {
			st.Target = rs.Target
		}
	}
	if state := realityWatchStates[inboundId]; state != nil {
		st.Ok, st.Fails, st.Reason = state.ok, state.fails, state.reason
		if !state.checkedAt.IsZero() {
			st.CheckedAt = state.checkedAt.Unix()
		}
		st.Events = append(st.Events, state.events...)
	}
	return st
}

// List is the status of every watched inbound.
func (s *RealityWatchService) List() []RealityWatchStatus {
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	out := []RealityWatchStatus{}
	for id, entry := range s.loadConfig() {
		out = append(out, s.statusLocked(id, entry))
	}
	return out
}

// Tick checks every watched inbound's target once (the cron job calls it). Node inbounds are
// skipped: their REALITY server runs on the node, so this server's view says nothing about it.
func (s *RealityWatchService) Tick(ctx context.Context) {
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	cfg := s.loadConfig()
	for id, entry := range cfg {
		ib, err := s.inboundService.GetInbound(id)
		if err != nil || !ib.Enable || ib.NodeID != nil {
			continue
		}
		rs, ok := realityOf(ib)
		if !ok {
			continue
		}
		state := realityWatchStates[id]
		if state == nil {
			state = &realityWatchState{}
			realityWatchStates[id] = state
		}
		res := realityWatchProbe(&s.serverService, rs.Target, firstString(rs.ServerNames), rs.Xver)
		state.checkedAt, state.ok, state.reason = time.Now(), res.Feasible, res.Reason
		if res.Feasible {
			state.fails = 0
			continue
		}
		state.fails++
		if !entry.Auto || state.fails < realityWatchFailLimit {
			continue
		}
		ev, err := s.switchLocked(ctx, ib, rs, entry, entry.Spares, cfg)
		if err != nil {
			// one "nothing works" line per outage, not one per check
			if n := len(state.events); n == 0 || state.events[n-1].To != "" {
				s.recordLocked(id, RealityWatchEvent{At: time.Now().Unix(), From: rs.Target, Reason: res.Reason})
			}
			logger.Warningf("reality watch: inbound %d target %s fails (%s) and no spare works", id, rs.Target, res.Reason)
			continue
		}
		ev.Reason = res.Reason
		state.events[len(state.events)-1].Reason = res.Reason
		state.fails = 0
	}
}

func firstString(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// SwitchTo moves an inbound to target now (the admin's button), after the same checks.
func (s *RealityWatchService) SwitchTo(ctx context.Context, inboundId int, target string) (*RealityWatchEvent, error) {
	target, err := normalizeRealityTarget(target)
	if err != nil {
		return nil, err
	}
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	ib, err := s.inboundService.GetInbound(inboundId)
	if err != nil {
		return nil, err
	}
	rs, ok := realityOf(ib)
	if !ok {
		return nil, errRealityWatchNotReality
	}
	cfg := s.loadConfig()
	ev, err := s.switchLocked(ctx, ib, rs, cfg[inboundId], []string{target}, cfg)
	if errors.Is(err, errRealityWatchNoSpare) {
		return nil, errRealityWatchTargetFails
	}
	return ev, err
}

// TestTarget runs the server check and the real REALITY connection on one target.
func (s *RealityWatchService) TestTarget(ctx context.Context, target string) (*RealityScanResult, error) {
	target, err := normalizeRealityTarget(target)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(target)
	res := realityWatchProbe(&s.serverService, target, host, 0)
	if res.Feasible {
		if err := realityWatchRealTest(ctx, target, host); err != nil {
			res.Feasible, res.Reason = false, "REALITY test failed: "+err.Error()
		}
	}
	return res, nil
}

// switchLocked tries candidates in order; the first that passes the server check and a real
// REALITY connection becomes the target, with its host as the only SNI. Needs realityWatchMu.
func (s *RealityWatchService) switchLocked(ctx context.Context, ib *model.Inbound, rs realitySettingsView,
	entry RealityWatchEntry, candidates []string, cfg map[int]RealityWatchEntry,
) (*RealityWatchEvent, error) {
	for _, cand := range candidates {
		if cand == rs.Target {
			continue
		}
		host, _, err := net.SplitHostPort(cand)
		if err != nil {
			continue
		}
		if res := realityWatchProbe(&s.serverService, cand, host, rs.Xver); !res.Feasible {
			continue
		}
		if err := realityWatchRealTest(ctx, cand, host); err != nil {
			logger.Infof("reality watch: %s passes the TLS check but not a real REALITY connection: %v", cand, err)
			continue
		}
		if err := s.applyTarget(ib, cand, host); err != nil {
			return nil, err
		}
		spares := []string{}
		for _, sp := range entry.Spares {
			if sp != cand {
				spares = append(spares, sp)
			}
		}
		if old, err := normalizeRealityTarget(rs.Target); err == nil && !containsString(spares, old) {
			spares = append(spares, old)
		}
		entry.Spares = spares
		cfg[ib.Id] = entry
		if err := s.storeConfig(cfg); err != nil {
			return nil, err
		}
		ev := RealityWatchEvent{At: time.Now().Unix(), From: rs.Target, To: cand}
		s.recordLocked(ib.Id, ev)
		logger.Warningf("reality watch: inbound %d switched from %s to %s", ib.Id, rs.Target, cand)
		return &ev, nil
	}
	return nil, errRealityWatchNoSpare
}

func (s *RealityWatchService) recordLocked(inboundId int, ev RealityWatchEvent) {
	state := realityWatchStates[inboundId]
	if state == nil {
		state = &realityWatchState{}
		realityWatchStates[inboundId] = state
	}
	state.events = append(state.events, ev)
	if len(state.events) > realityWatchMaxEvents {
		state.events = state.events[len(state.events)-realityWatchMaxEvents:]
	}
}

// applyTarget rewrites only target and serverNames, keeping every other REALITY setting, and
// saves through UpdateInbound so the change reaches Xray the same way an admin's edit does.
func (s *RealityWatchService) applyTarget(ib *model.Inbound, target, sni string) error {
	var stream map[string]any
	if err := json.Unmarshal([]byte(ib.StreamSettings), &stream); err != nil {
		return err
	}
	rs, ok := stream["realitySettings"].(map[string]any)
	if !ok {
		return errRealityWatchNotReality
	}
	delete(rs, "dest")
	rs["target"] = target
	rs["serverNames"] = []string{sni}
	b, err := json.Marshal(stream)
	if err != nil {
		return err
	}
	ib.StreamSettings = string(b)
	_, needRestart, err := s.inboundService.UpdateInbound(ib)
	if err != nil {
		return err
	}
	if needRestart {
		s.xrayService.SetToNeedRestart()
	}
	return nil
}
