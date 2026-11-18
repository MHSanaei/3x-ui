package mtproto

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func telemtDirForID(id int) string {
	return fmt.Sprintf("%s/telemt-%d", configDir(), id)
}

// configVerdict is what telemt says about the config on disk.
type configVerdict int

const (
	// verdictUnavailable: the API did not answer (still starting, or down).
	verdictUnavailable configVerdict = iota
	// verdictPending: valid on disk, not yet published to the running proxy.
	verdictPending
	verdictAccepted
	verdictRejected
	// verdictUnsupported: telemt older than 3.5.17 has no /v1/config/state.
	verdictUnsupported
)

// configState is the part of GET /v1/config/state the panel reads.
type configState struct {
	OK   bool `json:"ok"`
	Data struct {
		DesiredStatus string `json:"desired_status"`
		DesiredError  *struct {
			Path    string `json:"path"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"desired_error"`
		Comparison *struct {
			Status string `json:"status"`
		} `json:"comparison"`
		Watcher *struct {
			Phase     string `json:"phase"`
			ErrorCode string `json:"error_code"`
		} `json:"watcher"`
		WatcherMatchesDesired *bool `json:"watcher_matches_desired"`
	} `json:"data"`
}

// classifyConfigState: accepted needs the file valid, seen by the watcher and published,
// since a rejected reload leaves telemt silently on the old config.
func classifyConfigState(body []byte) (configVerdict, string) {
	var st configState
	if err := json.Unmarshal(body, &st); err != nil || !st.OK {
		return verdictUnavailable, "unreadable config state"
	}
	d := st.Data
	switch d.DesiredStatus {
	case "invalid", "unreadable":
		reason := d.DesiredStatus
		if e := d.DesiredError; e != nil {
			reason = strings.TrimSpace(strings.Join([]string{e.Path, e.Code, e.Message}, " "))
		}
		return verdictRejected, "config on disk is invalid: " + reason
	case "valid":
	default:
		return verdictPending, "desired " + d.DesiredStatus
	}
	matches := d.WatcherMatchesDesired != nil && *d.WatcherMatchesDesired
	if w := d.Watcher; w != nil && matches && (w.Phase == "rejected" || w.Phase == "publication_failed") {
		return verdictRejected, strings.TrimSpace("reload " + w.Phase + " " + w.ErrorCode)
	}
	if matches && d.Comparison != nil && d.Comparison.Status == "published_match" {
		return verdictAccepted, ""
	}
	status := "unknown"
	if d.Comparison != nil {
		status = d.Comparison.Status
	}
	return verdictPending, status
}

var telemtHTTP = &http.Client{Timeout: 3 * time.Second}

// fetchConfigState asks telemt whether it runs the access section on disk.
func fetchConfigState(port int, token string) (configVerdict, string) {
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/config/state?sections=access", port)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return verdictUnavailable, err.Error()
	}
	authorize(req, token)
	resp, err := telemtHTTP.Do(req)
	if err != nil {
		return verdictUnavailable, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusNotFound {
		return verdictUnsupported, "no /v1/config/state (telemt older than 3.5.17)"
	}
	if resp.StatusCode != http.StatusOK {
		return verdictUnavailable, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return classifyConfigState(body)
}

var (
	telemtPollInterval = 200 * time.Millisecond
	// Both waits hold the manager lock: a published reload takes milliseconds, and
	// an unconfirmed one is retried on the next reconcile tick anyway.
	telemtReloadTimeout = 3 * time.Second
	telemtStartTimeout  = 10 * time.Second
	telemtConfigState   = fetchConfigState
	// telemtRetryAfter is how long a config telemt refused to start with rests.
	telemtRetryAfter = time.Minute
)

// waitTelemtConfig polls the config state until a final verdict, the timeout,
// or (with alive set) the process dying, which reads as rejected.
func waitTelemtConfig(port int, token string, timeout time.Duration, alive func() bool) (configVerdict, string) {
	deadline := time.Now().Add(timeout)
	for {
		if alive != nil && !alive() {
			return verdictRejected, "telemt exited"
		}
		verdict, reason := telemtConfigState(port, token)
		switch verdict {
		case verdictAccepted, verdictRejected, verdictUnsupported:
			return verdict, reason
		}
		if time.Now().After(deadline) {
			return verdict, reason
		}
		time.Sleep(telemtPollInterval)
	}
}

// writeFileAtomic replaces path in one rename, so telemt's watcher never reads
// a half-written users.toml.
func writeFileAtomic(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// failedStart is a config telemt refused to start with, and when.
type failedStart struct {
	config string
	at     time.Time
}

// errTelemtRefused marks a config telemt would not start with; the manager rests
// it for telemtRetryAfter unless the desired config changes.
var errTelemtRefused = errors.New("telemt refused the config")

// telemtState is the telemt-specific part of a managed sidecar.
type telemtState struct {
	dir         string
	metricsPort int
	mainText    string
	usersText   string
	// rejectedUsersFP is the users.toml telemt refused, so the reconcile tick
	// does not rewrite and roll it back every 10s until the clients change.
	rejectedUsersFP string
	warnedNoState   bool
}

func (t *telemtState) mainPath() string  { return filepath.Join(t.dir, "telemt.toml") }
func (t *telemtState) usersPath() string { return filepath.Join(t.dir, telemtUsersFile) }

// ensureTelemtLocked moves inbound inst to a telemt process serving it: a
// users-only change is hot-reloaded, anything else restarts telemt.
func (m *Manager) ensureTelemtLocked(inst Instance) error {
	mainFP := telemtMainFingerprint(inst)
	usersText := renderTelemtUsers(inst)
	cur, ok := m.procs[inst.Id]
	if ok && cur.backend != BackendTelemt {
		m.stopLocked(inst.Id, cur)
		cur, ok = nil, false
	}
	if ok && cur.proc.IsRunning() && cur.structuralFP == mainFP {
		cur.tag = inst.Tag
		if cur.secretsFP == usersText || cur.telemt.rejectedUsersFP == usersText {
			return nil
		}
		return m.reloadTelemtUsersLocked(cur, inst.Id, usersText)
	}
	if f, ok := m.failedStart[inst.Id]; ok && f.config == mainFP+usersText && time.Since(f.at) < telemtRetryAfter {
		return nil
	}
	var prev *managed
	if ok {
		prev = cur
		_ = cur.proc.Stop()
		delete(m.procs, inst.Id)
	}
	started, err := m.startTelemtLocked(inst, mainFP, usersText)
	if err == nil {
		delete(m.failedStart, inst.Id)
		m.procs[inst.Id] = started
		logger.Infof("mtproto: started telemt for inbound %d on %s", inst.Id, inst.bindTo())
		return nil
	}
	if errors.Is(err, errTelemtRefused) {
		m.failedStart[inst.Id] = failedStart{config: mainFP + usersText, at: time.Now()}
	}
	if prev != nil && prev.telemt.mainText != "" {
		if rbErr := m.restartPreviousTelemtLocked(inst.Id, prev); rbErr != nil {
			logger.Errorf("mtproto: telemt for inbound %d did not come back on its previous config: %v", inst.Id, rbErr)
		}
	}
	return err
}

// startTelemtLocked writes both files and starts telemt, returning only once
// telemt confirmed the config or kept running past the start timeout.
func (m *Manager) startTelemtLocked(inst Instance, mainFP, usersText string) (*managed, error) {
	apiPort, err := FreeLocalPort()
	if err != nil {
		return nil, err
	}
	metricsPort, err := FreeLocalPort()
	if err != nil {
		return nil, err
	}
	token, err := newAPIToken()
	if err != nil {
		return nil, err
	}
	st := &telemtState{
		dir:         telemtDirForID(inst.Id),
		metricsPort: metricsPort,
		mainText:    renderTelemtMain(inst, telemtRuntime{apiPort: apiPort, metricsPort: metricsPort, apiToken: token}),
		usersText:   usersText,
	}
	cur := &managed{
		backend:      BackendTelemt,
		tag:          inst.Tag,
		structuralFP: mainFP,
		secretsFP:    usersText,
		apiPort:      apiPort,
		apiToken:     token,
		last:         map[string]clientCounters{},
		telemt:       st,
	}
	if err := m.launchTelemt(inst.Id, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

// launchTelemt writes cur's files, starts the process and waits for telemt to
// confirm the config; a process that exits meanwhile is a rejected config.
func (m *Manager) launchTelemt(id int, cur *managed) error {
	st := cur.telemt
	if err := writeFileAtomic(st.usersPath(), st.usersText); err != nil {
		return err
	}
	if err := writeFileAtomic(st.mainPath(), st.mainText); err != nil {
		return err
	}
	proc := m.newTelemtProc(st.mainPath(), st.dir, fmt.Sprintf("inbound %d", id))
	if err := proc.Start(); err != nil {
		return err
	}
	cur.proc = proc
	verdict, reason := waitTelemtConfig(cur.apiPort, cur.apiToken, telemtStartTimeout, proc.IsRunning)
	switch verdict {
	case verdictAccepted:
		return nil
	case verdictUnsupported:
		logger.Warningf("mtproto: telemt for inbound %d cannot confirm its config (%s); upgrade to 3.5.17+", id, reason)
		return nil
	case verdictRejected:
		_ = proc.Stop()
		if last := proc.GetResult(); last != "" {
			reason += ": " + last
		}
		return fmt.Errorf("%w of inbound %d: %s", errTelemtRefused, id, reason)
	default:
		logger.Warningf("mtproto: telemt for inbound %d has not confirmed its config yet (%s)", id, reason)
		return nil
	}
}

// restartPreviousTelemtLocked brings back the last config telemt ran, after a
// new one made it exit on start.
func (m *Manager) restartPreviousTelemtLocked(id int, prev *managed) error {
	if err := m.launchTelemt(id, prev); err != nil {
		return err
	}
	m.procs[id] = prev
	logger.Warningf("mtproto: telemt for inbound %d is back on its previous config", id)
	return nil
}

// reloadTelemtUsersLocked swaps users.toml in place and SIGHUPs telemt; a
// reload telemt refuses is rolled back so the file on disk is what it runs.
func (m *Manager) reloadTelemtUsersLocked(cur *managed, id int, usersText string) error {
	st := cur.telemt
	if err := writeFileAtomic(st.usersPath(), usersText); err != nil {
		return err
	}
	_ = cur.proc.Reload()
	verdict, reason := waitTelemtConfig(cur.apiPort, cur.apiToken, telemtReloadTimeout, nil)
	switch verdict {
	case verdictAccepted, verdictUnsupported:
		if verdict == verdictUnsupported && !st.warnedNoState {
			st.warnedNoState = true
			logger.Warningf("mtproto: telemt for inbound %d cannot confirm reloads (%s); upgrade to 3.5.17+", id, reason)
		}
		cur.secretsFP = usersText
		st.usersText = usersText
		st.rejectedUsersFP = ""
		logger.Infof("mtproto: applied client update to telemt inbound %d in place", id)
		return nil
	case verdictRejected:
		st.rejectedUsersFP = usersText
		if err := writeFileAtomic(st.usersPath(), st.usersText); err != nil {
			return fmt.Errorf("telemt rejected the clients of inbound %d (%s) and the previous file could not be restored: %w", id, reason, err)
		}
		_ = cur.proc.Reload()
		return fmt.Errorf("telemt rejected the clients of inbound %d (%s); it keeps serving the previous set", id, reason)
	default:
		return fmt.Errorf("telemt has not applied the clients of inbound %d yet (%s); retrying on the next reconcile", id, reason)
	}
}

// telemtMetric names: per-user counters are labelled user="<name>".
const (
	metricFromClient  = "telemt_user_octets_from_client_total"
	metricToClient    = "telemt_user_octets_to_client_total"
	metricConnections = "telemt_user_connections_current"
)

// parseTelemtMetrics reads the per-user traffic and live connections from
// telemt's Prometheus text: bytes from the client are upload, to it download.
func parseTelemtMetrics(r io.Reader) (map[string]statsUser, error) {
	users := map[string]statsUser{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		name, rest, ok := strings.Cut(line, "{")
		if !ok || (name != metricFromClient && name != metricToClient && name != metricConnections) {
			continue
		}
		labels, value, ok := strings.Cut(rest, "} ")
		if !ok {
			continue
		}
		user, ok := strings.CutPrefix(labels, `user="`)
		if !ok || !strings.HasSuffix(user, `"`) {
			continue
		}
		user = strings.TrimSuffix(user, `"`)
		n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			continue
		}
		u := users[user]
		switch name {
		case metricFromClient:
			u.BytesIn = int64(n)
		case metricToClient:
			u.BytesOut = int64(n)
		case metricConnections:
			u.Connections = int64(n)
		}
		users[user] = u
	}
	return users, sc.Err()
}

// scrapeTelemtMetrics reads telemt's loopback /metrics; best-effort like the mtg /stats.
func scrapeTelemtMetrics(port int) (map[string]statsUser, bool) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/metrics", port), nil)
	if err != nil {
		return nil, false
	}
	resp, err := telemtHTTP.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	users, err := parseTelemtMetrics(resp.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false
	}
	return users, true
}
