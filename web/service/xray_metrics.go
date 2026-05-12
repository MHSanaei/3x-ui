package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
)

type xrayMetricsState struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Reason  string `json:"reason,omitempty"`
}

type ObsTagSnapshot struct {
	Tag          string `json:"tag"`
	Alive        bool   `json:"alive"`
	Delay        int64  `json:"delay"`
	LastSeenTime int64  `json:"lastSeenTime"`
	LastTryTime  int64  `json:"lastTryTime"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type XrayMetricsService struct {
	settingService SettingService

	mu        sync.RWMutex
	state     xrayMetricsState
	client    *http.Client
	obsByTag  map[string]ObsTagSnapshot
}

var validObsTag = regexp.MustCompile(`^[a-zA-Z0-9._\-]+$`)

func obsHistoryKey(tag string) string {
	return "xrObs." + tag + ".delay"
}

func newXrayMetricsClient() *http.Client {
	return &http.Client{Timeout: 1500 * time.Millisecond}
}

func (s *XrayMetricsService) getClient() *http.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		s.client = newXrayMetricsClient()
	}
	return s.client
}

func (s *XrayMetricsService) State() xrayMetricsState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *XrayMetricsService) AggregateMetric(metric string, bucketSeconds, maxPoints int) []map[string]any {
	return xrayMetrics.aggregate(metric, bucketSeconds, maxPoints)
}

func (s *XrayMetricsService) ObservatorySnapshot() []ObsTagSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ObsTagSnapshot, 0, len(s.obsByTag))
	for _, v := range s.obsByTag {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out
}

func (s *XrayMetricsService) HasObservatoryTag(tag string) bool {
	if !validObsTag.MatchString(tag) {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.obsByTag[tag]
	return ok
}

func (s *XrayMetricsService) AggregateObservatory(tag string, bucketSeconds, maxPoints int) []map[string]any {
	if !validObsTag.MatchString(tag) {
		return []map[string]any{}
	}
	return xrayMetrics.aggregate(obsHistoryKey(tag), bucketSeconds, maxPoints)
}

func (s *XrayMetricsService) discoverListen() (string, error) {
	tmpl, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return "", err
	}
	var parsed struct {
		Metrics *struct {
			Listen string `json:"listen"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(tmpl), &parsed); err != nil {
		return "", err
	}
	if parsed.Metrics == nil || strings.TrimSpace(parsed.Metrics.Listen) == "" {
		return "", nil
	}
	return strings.TrimSpace(parsed.Metrics.Listen), nil
}

type rawObsEntry struct {
	Alive        bool   `json:"alive"`
	Delay        int64  `json:"delay"`
	LastSeenTime int64  `json:"last_seen_time"`
	LastTryTime  int64  `json:"last_try_time"`
	OutboundTag  string `json:"outbound_tag"`
}

func (s *XrayMetricsService) Sample(t time.Time) {
	listen, err := s.discoverListen()
	if err != nil {
		s.setState(xrayMetricsState{Reason: err.Error()})
		return
	}
	if listen == "" {
		s.setState(xrayMetricsState{Reason: "metrics block not configured in xray template"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	url := fmt.Sprintf("http://%s/debug/vars", listen)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		s.setState(xrayMetricsState{Listen: listen, Reason: err.Error()})
		return
	}
	resp, err := s.getClient().Do(req)
	if err != nil {
		s.setState(xrayMetricsState{Listen: listen, Reason: err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.setState(xrayMetricsState{Listen: listen, Reason: fmt.Sprintf("HTTP %d", resp.StatusCode)})
		return
	}

	var payload struct {
		MemStats struct {
			HeapAlloc   uint64      `json:"HeapAlloc"`
			Sys         uint64      `json:"Sys"`
			HeapObjects uint64      `json:"HeapObjects"`
			NumGC       uint32      `json:"NumGC"`
			PauseNs     [256]uint64 `json:"PauseNs"`
		} `json:"memstats"`
		Observatory map[string]rawObsEntry `json:"observatory"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		s.setState(xrayMetricsState{Listen: listen, Reason: err.Error()})
		return
	}

	xrayMetrics.append("xrAlloc", t, float64(payload.MemStats.HeapAlloc))
	xrayMetrics.append("xrSys", t, float64(payload.MemStats.Sys))
	xrayMetrics.append("xrHeapObjects", t, float64(payload.MemStats.HeapObjects))
	xrayMetrics.append("xrNumGC", t, float64(payload.MemStats.NumGC))
	var lastPause uint64
	if payload.MemStats.NumGC > 0 {
		idx := (payload.MemStats.NumGC + 255) % 256
		lastPause = payload.MemStats.PauseNs[idx]
	}
	xrayMetrics.append("xrPauseNs", t, float64(lastPause))

	s.applyObservatory(t, payload.Observatory)
	s.setState(xrayMetricsState{Enabled: true, Listen: listen})
}

func (s *XrayMetricsService) applyObservatory(t time.Time, entries map[string]rawObsEntry) {
	next := make(map[string]ObsTagSnapshot, len(entries))
	for key, e := range entries {
		tag := e.OutboundTag
		if tag == "" {
			tag = key
		}
		if !validObsTag.MatchString(tag) {
			continue
		}
		snap := ObsTagSnapshot{
			Tag:          tag,
			Alive:        e.Alive,
			Delay:        e.Delay,
			LastSeenTime: e.LastSeenTime,
			LastTryTime:  e.LastTryTime,
			UpdatedAt:    t.Unix(),
		}
		next[tag] = snap
		xrayMetrics.append(obsHistoryKey(tag), t, float64(e.Delay))
	}

	s.mu.Lock()
	for tag := range s.obsByTag {
		if _, kept := next[tag]; !kept {
			xrayMetrics.drop(obsHistoryKey(tag))
		}
	}
	s.obsByTag = next
	s.mu.Unlock()
}

func (s *XrayMetricsService) setState(st xrayMetricsState) {
	s.mu.Lock()
	s.state = st
	s.mu.Unlock()
	if !st.Enabled && st.Reason != "" {
		logger.Debugf("xray metrics unavailable: %s", st.Reason)
	}
}
