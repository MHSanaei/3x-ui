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

// The REALITY watch checks a target from this server and, after realityWatchFailLimit failures
// in a row, moves the inbound to the first spare that passes both checks.

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
	errRealityWatchChanged     = errors.New("reality watch: the target changed during the check")
	errRealityWatchOnNode      = errors.New("reality watch: this server cannot test a node inbound's target")

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
	InboundId   int                 `json:"inboundId" example:"3"`
	Target      string              `json:"target" example:"old.example.com:443"`
	ServerNames []string            `json:"serverNames" example:"[\"old.example.com\"]"`
	Auto        bool                `json:"auto" example:"true"`
	Spares      []string            `json:"spares" example:"[\"www.example.com:443\"]"`
	CheckedAt   int64               `json:"checkedAt" example:"1793000000"`
	Ok          bool                `json:"ok" example:"true"`
	Fails       int                 `json:"fails" example:"0"`
	Reason      string              `json:"reason" example:""`
	LatencyMs   int                 `json:"latencyMs" example:"180"`
	Events      []RealityWatchEvent `json:"events"`
}

type realityWatchState struct {
	checkedAt time.Time
	ok        bool
	fails     int
	reason    string
	latencyMs int
	events    []RealityWatchEvent
}

// realityWatchMu guards config, state and the final save, never a network check: those take
// minutes when targets time out and the API must answer meanwhile.
var (
	realityWatchMu     sync.Mutex
	realityWatchStates = map[int]*realityWatchState{}
	realityWatchTickMu sync.Mutex
)

func resetRealityWatchState() {
	realityWatchMu.Lock()
	defer realityWatchMu.Unlock()
	realityWatchStates = map[int]*realityWatchState{}
}

type RealityWatchService struct {
	inboundService InboundService
	hostService    HostService
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
	st := RealityWatchStatus{
		InboundId: inboundId, Auto: entry.Auto, Spares: entry.Spares,
		ServerNames: []string{}, Events: []RealityWatchEvent{},
	}
	if st.Spares == nil {
		st.Spares = []string{}
	}
	if ib, err := s.inboundService.GetInbound(inboundId); err == nil {
		if rs, ok := realityOf(ib); ok {
			st.Target = rs.Target
			if rs.ServerNames != nil {
				st.ServerNames = rs.ServerNames
			}
		}
	}
	if state := realityWatchStates[inboundId]; state != nil {
		st.Ok, st.Fails, st.Reason, st.LatencyMs = state.ok, state.fails, state.reason, state.latencyMs
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

// Tick checks every watched inbound's target once (the cron job calls it). A cycle still running
// when the next one fires keeps going alone, so one outage is not counted twice.
func (s *RealityWatchService) Tick(ctx context.Context) {
	if !realityWatchTickMu.TryLock() {
		return
	}
	defer realityWatchTickMu.Unlock()
	realityWatchMu.Lock()
	cfg := s.loadConfig()
	realityWatchMu.Unlock()
	for id := range cfg {
		s.checkInbound(ctx, id)
	}
}

// checkInbound skips node inbounds: their REALITY server runs on the node, so this server's
// view says nothing about it.
func (s *RealityWatchService) checkInbound(ctx context.Context, id int) {
	ib, err := s.inboundService.GetInbound(id)
	if err != nil || !ib.Enable || ib.NodeID != nil {
		return
	}
	rs, ok := realityOf(ib)
	if !ok || localRealityTarget(rs.Target) {
		return
	}
	res := realityWatchProbe(&s.serverService, rs.Target, firstString(rs.ServerNames), rs.Xver)
	// a domain that resolves into a private network: the SSRF guard blocked it, nothing failed
	if res.PrivateTarget {
		return
	}
	alive := realityTargetAlive(ctx, rs, res)

	realityWatchMu.Lock()
	entry := s.loadConfig()[id]
	state := realityWatchStates[id]
	if state == nil {
		state = &realityWatchState{}
		realityWatchStates[id] = state
	}
	state.checkedAt, state.ok, state.reason, state.latencyMs = time.Now(), alive, res.Reason, res.LatencyMs
	if alive {
		state.fails = 0
		state.reason = ""
		realityWatchMu.Unlock()
		return
	}
	state.fails++
	due := entry.Auto && state.fails >= realityWatchFailLimit
	realityWatchMu.Unlock()
	if !due {
		return
	}

	_, err = s.switchInbound(ctx, id, rs, entry.Spares, res.Reason)
	switch {
	case err == nil:
	case errors.Is(err, errRealityWatchChanged):
		logger.Infof("reality watch: inbound %d changed during the check, not switching it", id)
	default:
		realityWatchMu.Lock()
		defer realityWatchMu.Unlock()
		// one "nothing works" line per outage, not one per check
		if n := len(state.events); n == 0 || state.events[n-1].To != "" {
			s.recordLocked(id, RealityWatchEvent{At: time.Now().Unix(), From: rs.Target, Reason: res.Reason})
		}
		logger.Warningf("reality watch: inbound %d target %s fails (%s) and no spare works", id, rs.Target, res.Reason)
	}
}

// realityTargetAlive decides whether a target failed. The scanner's bar (h2, SNI on the cert) is
// for choosing a target; one that misses it can still carry REALITY, so the real test decides.
func realityTargetAlive(ctx context.Context, rs realitySettingsView, res *RealityScanResult) bool {
	if res.Feasible {
		return true
	}
	if !res.TLS13 {
		return false
	}
	target, err := normalizeRealityTarget(rs.Target)
	if err != nil {
		return false
	}
	sni := firstString(rs.ServerNames)
	if sni == "" {
		sni, _, _ = net.SplitHostPort(target)
	}
	if err := realityWatchRealTest(ctx, target, sni); err != nil {
		logger.Infof("reality watch: %s fails a real REALITY connection: %v", target, err)
		return false
	}
	return true
}

// localRealityTarget is a target on this machine or the LAN: a bare port, a unix socket, or
// a private / loopback address. The admin fronts REALITY with it on purpose; never replace it.
func localRealityTarget(target string) bool {
	t := strings.TrimSpace(target)
	if t == "" || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "@") {
		return true
	}
	if _, err := strconv.Atoi(t); err == nil {
		return true
	}
	host := t
	if h, _, err := net.SplitHostPort(t); err == nil {
		host = h
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}

func firstString(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// SwitchTo moves an inbound to target now (the admin's button), after the same checks. A node
// inbound is refused: both checks run here and say nothing about the node's network.
func (s *RealityWatchService) SwitchTo(ctx context.Context, inboundId int, target string) (*RealityWatchEvent, error) {
	target, err := normalizeRealityTarget(target)
	if err != nil {
		return nil, err
	}
	ib, err := s.inboundService.GetInbound(inboundId)
	if err != nil {
		return nil, err
	}
	if ib.NodeID != nil {
		return nil, errRealityWatchOnNode
	}
	rs, ok := realityOf(ib)
	if !ok {
		return nil, errRealityWatchNotReality
	}
	ev, err := s.switchInbound(ctx, inboundId, rs, []string{target}, "")
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

// switchInbound tries candidates in order; the first that passes the server check and a real
// REALITY connection becomes the target, with its host as the only SNI.
func (s *RealityWatchService) switchInbound(ctx context.Context, inboundId int, rs realitySettingsView,
	candidates []string, reason string,
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
		return s.commitSwitch(inboundId, rs.Target, cand, host, reason)
	}
	return nil, errRealityWatchNoSpare
}

// commitSwitch saves a fresh copy of the inbound, since the checks took long enough for an admin
// to save an edit. A target changed meanwhile means the admin already chose; nothing is switched.
func (s *RealityWatchService) commitSwitch(inboundId int, from, to, sni, reason string) (*RealityWatchEvent, error) {
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
	if rs.Target != from {
		return nil, errRealityWatchChanged
	}
	if err := s.applyTarget(ib, to, sni); err != nil {
		return nil, err
	}
	// The inbound has moved, so a host failure must not lose the spares and history below.
	if err := s.hostService.MoveInboundSni(inboundId, rs.ServerNames, sni); err != nil {
		logger.Warningf("reality watch: inbound %d switched but its hosts keep the old SNI: %v", inboundId, err)
	}
	InvalidateSubData()

	cfg := s.loadConfig()
	entry := cfg[inboundId]
	spares := []string{}
	for _, sp := range entry.Spares {
		if sp != to {
			spares = append(spares, sp)
		}
	}
	if old, err := normalizeRealityTarget(from); err == nil && !containsString(spares, old) {
		spares = append(spares, old)
	}
	entry.Spares = spares
	cfg[inboundId] = entry
	if err := s.storeConfig(cfg); err != nil {
		return nil, err
	}
	ev := RealityWatchEvent{At: time.Now().Unix(), From: from, To: to, Reason: reason}
	s.recordLocked(inboundId, ev)
	realityWatchStates[inboundId].fails = 0
	logger.Warningf("reality watch: inbound %d switched from %s to %s", inboundId, from, to)
	return &ev, nil
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
