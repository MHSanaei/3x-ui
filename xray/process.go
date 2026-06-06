package xray

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/common"
)

// GetBinaryName returns the Xray binary filename for the current OS and architecture.
func GetBinaryName() string {
	return fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// GetBinaryPath returns the full path to the Xray binary executable.
func GetBinaryPath() string {
	return config.GetBinFolderPath() + "/" + GetBinaryName()
}

// GetConfigPath returns the path to the Xray configuration file in the binary folder.
func GetConfigPath() string {
	return config.GetBinFolderPath() + "/config.json"
}

// GetGeositePath returns the path to the geosite data file used by Xray.
func GetGeositePath() string {
	return config.GetBinFolderPath() + "/geosite.dat"
}

// GetGeoipPath returns the path to the geoip data file used by Xray.
func GetGeoipPath() string {
	return config.GetBinFolderPath() + "/geoip.dat"
}

// GetIPLimitLogPath returns the path to the IP limit log file.
func GetIPLimitLogPath() string {
	return config.GetLogFolder() + "/3xipl.log"
}

// GetIPLimitBannedLogPath returns the path to the banned IP log file.
func GetIPLimitBannedLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.log"
}

// GetIPLimitBannedPrevLogPath returns the path to the previous banned IP log file.
func GetIPLimitBannedPrevLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.prev.log"
}

// GetAccessPersistentLogPath returns the path to the persistent access log file.
func GetAccessPersistentLogPath() string {
	return config.GetLogFolder() + "/3xipl-ap.log"
}

// GetAccessPersistentPrevLogPath returns the path to the previous persistent access log file.
func GetAccessPersistentPrevLogPath() string {
	return config.GetLogFolder() + "/3xipl-ap.prev.log"
}

// GetAccessLogPath reads the Xray config and returns the access log file path.
func GetAccessLogPath() (string, error) {
	config, err := os.ReadFile(GetConfigPath())
	if err != nil {
		logger.Warningf("Failed to read configuration file: %s", err)
		return "", err
	}

	jsonConfig := map[string]any{}
	err = json.Unmarshal([]byte(config), &jsonConfig)
	if err != nil {
		logger.Warningf("Failed to parse JSON configuration: %s", err)
		return "", err
	}

	if jsonConfig["log"] != nil {
		jsonLog := jsonConfig["log"].(map[string]any)
		if jsonLog["access"] != nil {
			accessLogPath := jsonLog["access"].(string)
			return accessLogPath, nil
		}
	}
	return "", err
}

// stopProcess calls Stop on the given Process instance.
func stopProcess(p *Process) {
	p.Stop()
}

// Process wraps an Xray process instance and provides management methods.
type Process struct {
	*process
}

// NewProcess creates a new Xray process and sets up cleanup on garbage collection.
func NewProcess(xrayConfig *Config) *Process {
	p := &Process{newProcess(xrayConfig)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

// NewTestProcess creates a new Xray process that uses a specific config file path.
// Used for test runs (e.g. outbound test) so the main config.json is not overwritten.
// The config file at configPath is removed when the process is stopped.
func NewTestProcess(xrayConfig *Config, configPath string) *Process {
	p := &Process{newTestProcess(xrayConfig, configPath)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

type process struct {
	cmd  *exec.Cmd
	done chan struct{}

	version string
	apiPort int

	// onlineClients is the set of emails active on THIS panel's own xray
	// within the online grace window. It is derived only from local xray
	// traffic polls (see RefreshLocalOnline) — never from remote-node
	// snapshots — so a client connected solely to a remote node is not
	// reported online on local inbounds.
	onlineClients []string
	// localActiveInbounds is the set of THIS panel's inbound tags that
	// carried traffic within the same grace window. Xray's user>>>email
	// stat aggregates across every inbound a client is attached to, so an
	// online email alone can't say which inbound it actually used. Pairing
	// it with the inbound>>>tag stat lets the per-inbound view drop a
	// multi-inbound client from inbounds that saw no traffic this window.
	localActiveInbounds []string
	// localLastOnline records, per email, the last time this panel's own
	// xray reported traffic for it. RefreshLocalOnline rebuilds
	// onlineClients from this map each tick, keeping the local online set
	// independent of the shared client_traffics.last_online column — that
	// column is bumped by remote-node syncs too and would otherwise leak
	// remote-only clients into the local set.
	localLastOnline map[string]int64
	// localInboundLastActive mirrors localLastOnline for inbound tags: the
	// last tick this panel's xray reported traffic through each tag.
	// Rebuilt into localActiveInbounds under the same grace window so the
	// two signals stay aligned — an email within grace always has the
	// inbound it used within grace too.
	localInboundLastActive map[string]int64
	// nodeOnlineTrees holds, per direct remote node (keyed by that node's
	// panel-local id), the GUID-keyed online-emails subtree that node
	// reported — its own clients under its panelGuid plus every descendant
	// under theirs. Keying the stored value by GUID (not node id) lets the
	// master attribute a deeply nested client to the node that physically
	// hosts it across a chain (#4983); the outer node-id key is only so a
	// failed probe can drop that whole branch's contribution. NodeTrafficSyncJob
	// populates entries per cron tick and clears them when a probe fails. The
	// mutex guards this map, onlineClients, and localLastOnline above so the
	// online getters never see a torn read.
	nodeOnlineTrees map[int]map[string][]string
	onlineMu        sync.RWMutex

	config     *Config
	configPath string // if set, use this path instead of GetConfigPath() and remove on Stop
	logWriter  *LogWriter
	exitErr    error
	startTime  time.Time

	intentionalStop atomic.Bool
}

var (
	xrayGracefulStopTimeout = 5 * time.Second
	xrayForceStopTimeout    = 2 * time.Second
)

// newProcess creates a new internal process struct for Xray.
func newProcess(config *Config) *process {
	return &process{
		version:   "Unknown",
		config:    config,
		logWriter: NewLogWriter(),
		startTime: time.Now(),
	}
}

// newTestProcess creates a process that writes and runs with a specific config path.
func newTestProcess(config *Config, configPath string) *process {
	p := newProcess(config)
	p.configPath = configPath
	return p
}

// IsRunning returns true if the Xray process is currently running.
func (p *process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	if p.done != nil {
		select {
		case <-p.done:
			return false
		default:
		}
	}
	if p.cmd.ProcessState == nil {
		return true
	}
	return false
}

// GetErr returns the last error encountered by the Xray process.
func (p *process) GetErr() error {
	return p.exitErr
}

// GetResult returns the last log line or error from the Xray process.
func (p *process) GetResult() string {
	if len(p.logWriter.lastLine) == 0 && p.exitErr != nil {
		return p.exitErr.Error()
	}
	return p.logWriter.lastLine
}

// GetVersion returns the version string of the Xray process.
func (p *process) GetVersion() string {
	return p.version
}

// GetAPIPort returns the API port used by the Xray process.
func (p *Process) GetAPIPort() int {
	return p.apiPort
}

// GetConfig returns the configuration used by the Xray process.
func (p *Process) GetConfig() *Config {
	return p.config
}

// GetOnlineClients returns the union of locally-online clients and
// node-online clients from every registered remote panel. Dedupes by
// email so a client connected to both a local and a node-managed inbound
// surfaces once. Cheap allocation — typical online sets are small and
// the union is recomputed on demand.
func (p *Process) GetOnlineClients() []string {
	p.onlineMu.RLock()
	defer p.onlineMu.RUnlock()

	if len(p.nodeOnlineTrees) == 0 {
		// Hot path for single-panel deployments: avoid the map+dedupe
		// work entirely and return the local slice as-is.
		return p.onlineClients
	}

	seen := make(map[string]struct{}, len(p.onlineClients))
	out := make([]string, 0, len(p.onlineClients))
	add := func(emails []string) {
		for _, email := range emails {
			if _, dup := seen[email]; dup {
				continue
			}
			seen[email] = struct{}{}
			out = append(out, email)
		}
	}
	add(p.onlineClients)
	for _, tree := range p.nodeOnlineTrees {
		for _, emails := range tree {
			add(emails)
		}
	}
	return out
}

// GetLocalOnlineClients returns a copy of the emails online on THIS panel's own
// xray within the grace window. The service layer keys these under the panel's
// own GUID when assembling the per-node online view.
func (p *Process) GetLocalOnlineClients() []string {
	p.onlineMu.RLock()
	defer p.onlineMu.RUnlock()
	if len(p.onlineClients) == 0 {
		return nil
	}
	out := make([]string, len(p.onlineClients))
	copy(out, p.onlineClients)
	return out
}

// GetMergedNodeTrees returns the union of every direct node's reported subtree,
// keyed by the panelGuid of the node that physically hosts each client set.
// Because each child already reports its descendants under their own GUIDs,
// merging the direct children yields the whole tree at any depth (#4983), so a
// client three hops down is attributed to its real node, not the intermediate
// one. GUIDs are globally unique, but a set reported under the same GUID by more
// than one path is deduped per key; empty sets are omitted.
func (p *Process) GetMergedNodeTrees() map[string][]string {
	p.onlineMu.RLock()
	defer p.onlineMu.RUnlock()
	if len(p.nodeOnlineTrees) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string)
	seen := make(map[string]map[string]struct{})
	for _, tree := range p.nodeOnlineTrees {
		for guid, emails := range tree {
			if guid == "" || len(emails) == 0 {
				continue
			}
			dedup := seen[guid]
			if dedup == nil {
				dedup = make(map[string]struct{}, len(emails))
				seen[guid] = dedup
			}
			for _, email := range emails {
				if _, ok := dedup[email]; ok {
					continue
				}
				dedup[email] = struct{}{}
				out[guid] = append(out[guid], email)
			}
		}
	}
	return out
}

// GetLocalActiveInbounds returns a copy of THIS panel's inbound tags that
// carried traffic within the grace window. Only the local xray reports
// per-inbound activity; remote-node snapshots don't carry it, so the service
// layer keys these under the panel's own GUID and a node missing from the
// active-inbounds map means "don't gate" (fall back to the email-only signal).
func (p *Process) GetLocalActiveInbounds() []string {
	p.onlineMu.RLock()
	defer p.onlineMu.RUnlock()
	if len(p.localActiveInbounds) == 0 {
		return nil
	}
	out := make([]string, len(p.localActiveInbounds))
	copy(out, p.localActiveInbounds)
	return out
}

// RefreshLocalOnline records that each email in activeEmails and each tag in
// activeInboundTags had local xray traffic at now, then rebuilds onlineClients
// and localActiveInbounds from every entry seen within graceMs, pruning older
// ones. Called by the local XrayTrafficJob after each xray gRPC stats poll.
// Pass nil/empty slices to only prune — NodeTrafficSyncJob does this so a
// stopped local xray's clients and inbounds still age out between local polls.
func (p *Process) RefreshLocalOnline(activeEmails, activeInboundTags []string, now, graceMs int64) {
	p.onlineMu.Lock()
	defer p.onlineMu.Unlock()
	if p.localLastOnline == nil {
		p.localLastOnline = make(map[string]int64, len(activeEmails))
	}
	for _, email := range activeEmails {
		p.localLastOnline[email] = now
	}
	online := make([]string, 0, len(p.localLastOnline))
	for email, ts := range p.localLastOnline {
		if now-ts < graceMs {
			online = append(online, email)
		} else {
			delete(p.localLastOnline, email)
		}
	}
	p.onlineClients = online

	if p.localInboundLastActive == nil {
		p.localInboundLastActive = make(map[string]int64, len(activeInboundTags))
	}
	for _, tag := range activeInboundTags {
		p.localInboundLastActive[tag] = now
	}
	activeInbounds := make([]string, 0, len(p.localInboundLastActive))
	for tag, ts := range p.localInboundLastActive {
		if now-ts < graceMs {
			activeInbounds = append(activeInbounds, tag)
		} else {
			delete(p.localInboundLastActive, tag)
		}
	}
	p.localActiveInbounds = activeInbounds
}

// SetNodeOnlineTree records the GUID-keyed online subtree one direct remote
// node reported (its own clients under its panelGuid plus every descendant
// under theirs). Replaces any previous entry for that node — NodeTrafficSyncJob
// always sends the full subtree per tick.
func (p *Process) SetNodeOnlineTree(nodeID int, tree map[string][]string) {
	p.onlineMu.Lock()
	defer p.onlineMu.Unlock()
	if p.nodeOnlineTrees == nil {
		p.nodeOnlineTrees = map[int]map[string][]string{}
	}
	p.nodeOnlineTrees[nodeID] = tree
}

// ClearNodeOnlineClients drops a direct node's whole subtree contribution.
// Called when a probe fails so a downed node — and everything behind it — doesn't
// keep its clients listed as "online" until the next successful probe.
func (p *Process) ClearNodeOnlineClients(nodeID int) {
	p.onlineMu.Lock()
	defer p.onlineMu.Unlock()
	delete(p.nodeOnlineTrees, nodeID)
}

// GetUptime returns the uptime of the Xray process in seconds.
func (p *Process) GetUptime() uint64 {
	return uint64(time.Since(p.startTime).Seconds())
}

// refreshAPIPort updates the API port from the inbound configs.
func (p *process) refreshAPIPort() {
	for _, inbound := range p.config.InboundConfigs {
		if inbound.Tag == "api" {
			p.apiPort = inbound.Port
			break
		}
	}
}

// refreshVersion updates the version string by running the Xray binary with -version.
func (p *process) refreshVersion() {
	cmd := exec.Command(GetBinaryPath(), "-version")
	data, err := cmd.Output()
	if err != nil {
		p.version = "Unknown"
	} else {
		datas := bytes.Split(data, []byte(" "))
		if len(datas) <= 1 {
			p.version = "Unknown"
		} else {
			p.version = string(datas[1])
		}
	}
}

// Start launches the Xray process with the current configuration.
func (p *process) Start() (err error) {
	if p.IsRunning() {
		return errors.New("xray is already running")
	}

	defer func() {
		if err != nil {
			logger.Error("Failure in running xray-core process: ", err)
			p.exitErr = err
		}
	}()

	data, err := json.MarshalIndent(p.config, "", "  ")
	if err != nil {
		return common.NewErrorf("Failed to generate XRAY configuration files: %v", err)
	}

	err = os.MkdirAll(config.GetLogFolder(), 0o770)
	if err != nil {
		logger.Warningf("Failed to create log folder: %s", err)
	}

	configPath := GetConfigPath()
	if p.configPath != "" {
		configPath = p.configPath
	}
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return common.NewErrorf("Failed to write configuration file: %v", err)
	}

	cmd := exec.Command(GetBinaryPath(), "-c", configPath)
	cmd.Stdout = p.logWriter
	cmd.Stderr = p.logWriter

	err = p.startCommand(cmd)
	if err != nil {
		return err
	}

	p.refreshVersion()
	p.refreshAPIPort()

	return nil
}

func (p *process) startCommand(cmd *exec.Cmd) error {
	p.cmd = cmd
	p.done = make(chan struct{})
	p.exitErr = nil
	p.intentionalStop.Store(false)

	if err := cmd.Start(); err != nil {
		close(p.done)
		p.cmd = nil
		return err
	}

	attachChildLifetime(cmd)

	go p.waitForCommand(cmd)
	return nil
}

func (p *process) waitForCommand(cmd *exec.Cmd) {
	defer close(p.done)

	err := cmd.Wait()
	if err == nil || p.intentionalStop.Load() {
		return
	}

	// On Windows, killing the process results in "exit status 1" which isn't an error for us.
	if runtime.GOOS == "windows" {
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "exit status 1") {
			p.exitErr = err
			return
		}
	}

	logger.Error("Failure in running xray-core:", err)
	p.exitErr = err
}

// Stop terminates the running Xray process.
func (p *process) Stop() error {
	if !p.IsRunning() {
		return errors.New("xray is not running")
	}
	p.intentionalStop.Store(true)

	// Remove temporary config file used for test runs so main config is never touched
	if p.configPath != "" {
		if p.configPath != GetConfigPath() {
			// Check if file exists before removing
			if _, err := os.Stat(p.configPath); err == nil {
				_ = os.Remove(p.configPath)
			}
		}
	}

	if runtime.GOOS == "windows" {
		if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return p.waitForExit(xrayForceStopTimeout)
	}

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return p.waitForExit(xrayForceStopTimeout)
		}
		return err
	}

	if err := p.waitForExit(xrayGracefulStopTimeout); err == nil {
		return nil
	}

	logger.Warning("xray-core did not stop after SIGTERM, killing process")
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return p.waitForExit(xrayForceStopTimeout)
}

func (p *process) waitForExit(timeout time.Duration) error {
	if p.done == nil {
		return nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-p.done:
		return nil
	case <-timer.C:
		return common.NewErrorf("timed out waiting for xray-core process to stop after %s", timeout)
	}
}

const (
	crashReportPrefix = "core_crash_"
	crashReportSuffix = ".log"
	maxCrashReports   = 10
)

// writeCrashReport persists a captured xray crash chunk to the log folder
// with nanosecond-precision filename so restart-loop bursts don't overwrite
// each other, and prunes old reports to keep the folder bounded.
func writeCrashReport(m []byte) error {
	dir := config.GetLogFolder()
	if err := os.MkdirAll(dir, 0o770); err != nil {
		return err
	}
	pruneOldCrashReports(dir, maxCrashReports-1)
	name := crashReportPrefix + time.Now().Format("20060102_150405_000000000") + crashReportSuffix
	return os.WriteFile(filepath.Join(dir, name), m, 0o640)
}

func pruneOldCrashReports(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var reports []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, crashReportPrefix) && strings.HasSuffix(n, crashReportSuffix) {
			reports = append(reports, n)
		}
	}
	if len(reports) <= keep {
		return
	}
	sort.Strings(reports)
	for _, old := range reports[:len(reports)-keep] {
		_ = os.Remove(filepath.Join(dir, old))
	}
}
