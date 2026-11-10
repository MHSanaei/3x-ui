package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/version"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// PanelService provides business logic for panel management operations.
// It handles panel restart, updates, and system-level panel controls.
type PanelService struct{}

// PanelUpdateInfo contains the current and latest available panel versions.
// On the dev channel the version fields carry a "dev+<sha>" label and the commit
// fields hold the short SHAs that drive the update-available decision.
type PanelUpdateInfo struct {
	Channel         string `json:"channel"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	CurrentCommit   string `json:"currentCommit,omitempty"`
	LatestCommit    string `json:"latestCommit,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

const (
	panelRepository      = "jrangus/3x-ui"
	panelRepositoryAPI   = "https://api.github.com/repos/" + panelRepository
	panelRepositoryRaw   = "https://raw.githubusercontent.com/" + panelRepository
	maxPanelUpdaterBytes = 2 << 20
	// devReleaseTag is the fixed-tag rolling pre-release the CI force-moves to the
	// newest main commit; the dev update channel installs from it.
	devReleaseTag = "dev-latest"

	updateStatePending = "pending"
	updateStateSuccess = "success"
	updateStateFailed  = "failed"
)

// PanelUpdateStatus reports the outcome of the most recently launched panel
// self-update. RunID lets the caller confirm this status belongs to the
// update it started rather than a stale result left over from an earlier
// run; State is one of "pending", "success", or "failed". RunID is a decimal
// string, not a JSON number: it's a formatted UnixNano timestamp, and
// JavaScript's number type can't represent that precisely (it exceeds
// Number.MAX_SAFE_INTEGER), which would let two different runs round to the
// same value on the wire and defeat the whole point of this field.
type PanelUpdateStatus struct {
	RunID      string `json:"runId" example:"1735689600123456789"`
	State      string `json:"state" example:"success"`
	ExitCode   int    `json:"exitCode" example:"0"`
	FinishedAt int64  `json:"finishedAt" example:"1735689612"`
}

var (
	releaseCommitRegex = regexp.MustCompile(`(?i)commit=([0-9a-f]{7,40})`)
	datedReleaseRegex  = regexp.MustCompile(`^3x-ui-(\d{8})(?:-r(\d+))?$`)
)

// updateMu guards updateRunning/updateStarted/updateRunID/updatePID, which
// stop a second self-update from launching while one is still in flight (two
// concurrent update.sh runs would race each other extracting the release
// tarball and swapping the service unit). A slot is released as soon as the
// in-flight run's own status file reports success or failure -- checked
// against updateRunID so a stale file from an even earlier run can't be
// mistaken for this one finishing -- so a fast failure doesn't lock out a
// retry.
//
// For a run that never reaches a terminal state at all, staleness is judged
// primarily by whether the process we actually launched is still alive
// (updatePID, via processAlive), not by wall-clock time alone: update.sh
// runs install_base() (a package-manager update+install) before anything
// else, plus several downloads, which can legitimately run past a short
// fixed timeout on a slow or throttled host without anything being wrong.
// updateStaleAfter/updatePID together are only a fallback for the systemd-run
// launch path, where the process we can observe (systemd-run itself) has
// already exited by the time startUpdate returns and the actual update.sh
// unit's PID is never recorded -- for that path this is still a pure
// wall-clock heuristic. updateHardCeiling is an absolute backstop so a
// genuinely wedged run (alive but hung forever) can never lock out retries
// permanently, even on the PID-tracked path.
var (
	updateMu      sync.Mutex
	updateRunning bool
	updateStarted time.Time
	updateRunID   int64
	updatePID     int
	updateUnit    string
)

const (
	updateStaleAfter  = 5 * time.Minute
	updateHardCeiling = 25 * time.Minute
)

func (s *PanelService) RestartPanel(delay time.Duration) error {
	go func() {
		time.Sleep(delay)
		if global.TriggerRestart() {
			return
		}
		if runtime.GOOS == "windows" {
			logger.Error("panel restart: no restart hook registered (SIGHUP unsupported on Windows)")
			return
		}
		p, err := os.FindProcess(syscall.Getpid())
		if err != nil {
			logger.Error("panel restart: FindProcess failed:", err)
			return
		}
		if err := p.Signal(syscall.SIGHUP); err != nil {
			logger.Error("failed to send SIGHUP signal:", err)
		}
	}()
	return nil
}

// GetUpdateInfo checks GitHub for the latest 3x-ui release. When the dev channel
// is enabled on a dev build it compares commits against the rolling dev release;
// otherwise it compares versions against the latest stable tag.
func (s *PanelService) GetUpdateInfo() (*PanelUpdateInfo, error) {
	if devChannelActive() {
		return getDevUpdateInfo()
	}
	latest, err := fetchLatestPanelVersion()
	if err != nil {
		return nil, err
	}
	current := config.GetPanelVersion()
	return &PanelUpdateInfo{
		Channel:         "stable",
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: isNewerVersion(latest, current),
	}, nil
}

// devChannelActive reports whether self-update should track the rolling dev
// release. A binary built from a commit must stay on that channel even when an
// older database still contains devChannelEnable=false; otherwise a customized
// build incorrectly compares itself with an unrelated stable release.
func devChannelActive() bool {
	if config.IsDevBuild() {
		return true
	}
	settings := &service.SettingService{}
	enabled, err := settings.GetDevChannelEnable()
	// dev-latest is a compatibility transport for historical dev installs, but
	// its payload can be an immutable dated stable release. Once that release is
	// running it must leave the dev channel even if the old database still says
	// devChannelEnable=true; stable builds have no buildCommit, so comparing them
	// as dev builds would advertise the same payload forever and create an
	// update/install loop.
	if isDatedStableRelease(config.GetBuildRelease()) {
		if err == nil && enabled {
			if setErr := settings.SetDevChannelEnable(false); setErr != nil {
				logger.Warning("failed to clear legacy dev update channel after stable upgrade:", setErr)
			}
		}
		return false
	}
	return err == nil && enabled
}

func isDatedStableRelease(release string) bool {
	return datedReleaseRegex.MatchString(strings.TrimSpace(release))
}

// getDevUpdateInfo compares the running commit against the commit recorded in the
// rolling dev release.
func getDevUpdateInfo() (*PanelUpdateInfo, error) {
	release, err := fetchPanelRelease(devReleaseTag)
	if err != nil {
		return nil, err
	}
	latestCommit := extractReleaseCommit(release)
	if latestCommit == "" {
		return nil, fmt.Errorf("dev release commit is unknown")
	}
	currentCommit := config.GetBuildCommit()
	return &PanelUpdateInfo{
		Channel:         "dev",
		CurrentVersion:  config.GetPanelVersion(),
		CurrentCommit:   shortCommit(currentCommit),
		LatestCommit:    shortCommit(latestCommit),
		LatestVersion:   "dev+" + shortCommit(latestCommit),
		UpdateAvailable: !commitsEqual(currentCommit, latestCommit),
	}, nil
}

// StartUpdate starts the official updater using this panel's own channel
// setting. Returns the run ID to pass to GetUpdateStatus so the caller can
// tell this run's result apart from a stale one.
func (s *PanelService) StartUpdate() (int64, error) {
	return s.startUpdate(devChannelActive())
}

// StartUpdateChannel runs the updater against an explicitly chosen channel,
// overriding the local dev-channel setting. Used by the master node updater so
// a node can be moved to the dev channel from the central panel.
func (s *PanelService) StartUpdateChannel(dev bool) (int64, error) {
	return s.startUpdate(dev)
}

// GetUpdateStatus reports the outcome of the most recently launched panel
// self-update, as recorded by update.sh's EXIT trap (see the script for why
// that covers every exit path, not just the happy one). This is a best-effort
// side channel: a missing or unreadable status file reads as "pending"
// rather than an error, since the update itself is what matters, not this
// status file.
func (s *PanelService) GetUpdateStatus() *PanelUpdateStatus {
	data, err := os.ReadFile(config.GetUpdateStatusFilePath())
	if err != nil {
		return &PanelUpdateStatus{State: updateStatePending}
	}
	var status PanelUpdateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return &PanelUpdateStatus{State: updateStatePending}
	}
	if status.State != updateStateSuccess && status.State != updateStateFailed {
		status.State = updateStatePending
	}
	return &status
}

func (s *PanelService) startUpdate(useDev bool) (int64, error) {
	runID := time.Now().UnixNano()
	activeRunID, acquired := acquireOrJoinUpdateSlot(runID)
	if !acquired {
		// Treat a repeated click (or a click after refreshing the page) as an
		// idempotent request: return the already-running job so the UI can resume
		// polling it instead of showing a dead-end "already in progress" error.
		return activeRunID, nil
	}
	launched := false
	defer func() {
		if !launched {
			releaseUpdateSlot()
		}
	}()

	if runtime.GOOS != "linux" {
		return 0, fmt.Errorf("panel web update is supported only on Linux installations")
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		return 0, fmt.Errorf("bash is required to run the panel updater: %w", err)
	}

	scriptPath, err := downloadPanelUpdater()
	if err != nil {
		return 0, err
	}
	checkCtx, cancelCheck := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelCheck()
	if output, syntaxErr := exec.CommandContext(checkCtx, bash, "-n", scriptPath).CombinedOutput(); syntaxErr != nil {
		_ = os.Remove(scriptPath)
		return 0, fmt.Errorf("downloaded panel updater failed syntax validation: %w: %s", syntaxErr, strings.TrimSpace(string(output)))
	}

	statusFile := config.GetUpdateStatusFilePath()

	mainFolder, serviceFolder := resolveUpdateFolders()
	updateTag := ""
	if useDev {
		updateTag = devReleaseTag
	}
	updateCommand := fmt.Sprintf("%s %s", shellQuote(bash), shellQuote(scriptPath))
	if timeoutBin, timeoutErr := exec.LookPath("timeout"); timeoutErr == nil {
		updateCommand = fmt.Sprintf("%s --signal=TERM --kill-after=30s 20m %s", shellQuote(timeoutBin), updateCommand)
	}
	updateScript := fmt.Sprintf("set -e; trap 'rm -f %s' EXIT; %s", shellQuote(scriptPath), updateCommand)
	runIDEnv := "XUI_UPDATE_RUN_ID=" + strconv.FormatInt(runID, 10)
	statusFileEnv := "XUI_UPDATE_STATUS_FILE=" + statusFile
	proxyEnv := updateProxyEnvVars()

	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		unitName := fmt.Sprintf("x-ui-web-update-%d", time.Now().Unix())
		args := []string{
			"--unit", unitName,
			"--setenv", "XUI_MAIN_FOLDER=" + mainFolder,
			"--setenv", "XUI_SERVICE=" + serviceFolder,
			"--setenv", "XUI_UPDATE_TAG=" + updateTag,
			"--setenv", runIDEnv,
			"--setenv", statusFileEnv,
		}
		for _, kv := range proxyEnv {
			args = append(args, "--setenv", kv)
		}
		args = append(args, bash, "-lc", updateScript)
		cmd := exec.CommandContext(context.Background(), systemdRun, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			output := strings.TrimSpace(string(out))
			if !strings.Contains(output, "System has not been booted with systemd") &&
				!strings.Contains(output, "Failed to connect to bus") {
				_ = os.Remove(scriptPath)
				return 0, fmt.Errorf("failed to start panel update job: %w: %s", err, output)
			}
			logger.Warning("systemd-run is unavailable, falling back to detached update process:", output)
		} else {
			logger.Infof("started panel update job via systemd-run unit %s", unitName)
			recordUpdateUnit(unitName)
			launched = true
			return runID, nil
		}
	}

	cmd := exec.CommandContext(context.Background(), bash, "-lc", updateScript)
	cmd.Env = append(os.Environ(),
		"XUI_MAIN_FOLDER="+mainFolder,
		"XUI_SERVICE="+serviceFolder,
		"XUI_UPDATE_TAG="+updateTag,
		runIDEnv,
		statusFileEnv,
	)
	setDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return 0, fmt.Errorf("failed to start panel update job: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		logger.Warning("failed to release panel update process:", err)
	}
	logger.Infof("started panel update job with pid %d", cmd.Process.Pid)
	recordUpdatePID(cmd.Process.Pid)
	launched = true
	return runID, nil
}

// updateProxyEnvVars forwards ambient proxy env vars to systemd-run's child,
// which (unlike the bash fallback) inherits nothing but --setenv.
func updateProxyEnvVars() []string {
	var out []string
	for _, key := range []string{"https_proxy", "HTTPS_PROXY", "all_proxy", "ALL_PROXY", "http_proxy", "HTTP_PROXY", "no_proxy", "NO_PROXY"} {
		if v := os.Getenv(key); v != "" {
			out = append(out, key+"="+v)
		}
	}
	return out
}

// acquireUpdateSlot claims the single in-flight-update slot for runID. It
// refuses while another run is genuinely still in flight, but grants the
// slot immediately once that run's own status file reports a terminal
// result (success or failure) -- a fast failure shouldn't force the next
// attempt to wait out updateStaleAfter for no reason. Past updateStaleAfter
// with no terminal status yet, it grants the slot anyway UNLESS the process
// we recorded (updatePID) is confirmed still alive, so a merely-slow run
// isn't mistaken for a crashed one; past updateHardCeiling it grants the
// slot unconditionally regardless of liveness, so a truly wedged run can
// never lock out retries forever.
func acquireUpdateSlot(runID int64) bool {
	_, acquired := acquireOrJoinUpdateSlot(runID)
	return acquired
}

// acquireOrJoinUpdateSlot either claims the update slot or returns the run ID
// already occupying it. Returning the active ID makes repeated web requests
// idempotent and lets a refreshed UI reconnect to the original update job.
func acquireOrJoinUpdateSlot(runID int64) (int64, bool) {
	updateMu.Lock()
	defer updateMu.Unlock()
	if updateRunning && !previousRunIsTerminal() {
		// A failed updater does not always get far enough to write its status
		// file. In that case the process/unit we launched is authoritative: once
		// it has exited, keeping the in-memory slot for the stale timeout only
		// turns a recoverable update error into an unnecessary lockout.
		processEnded := updatePID > 0 && !processAlive(updatePID)
		unitEnded := false
		unitRunning := false
		unitKnown := false
		if updateUnit != "" {
			unitRunning, unitKnown = panelUpdateUnitRunning(updateUnit)
			unitEnded = unitKnown && !unitRunning
		}
		if !processEnded && !unitEnded {
			elapsed := time.Since(updateStarted)
			if elapsed < updateHardCeiling {
				stale := elapsed >= updateStaleAfter
				alive := (updatePID > 0 && processAlive(updatePID)) || (unitKnown && unitRunning)
				if !stale || alive {
					return updateRunID, false
				}
			}
		}
	}
	updateRunning = true
	updateStarted = time.Now()
	updateRunID = runID
	updatePID = 0
	updateUnit = ""
	return runID, true
}

// recordUpdatePID notes the PID of the detached update.sh process the
// current slot is tracking, so a later acquireUpdateSlot call can check
// whether it is actually still running instead of only how long ago it
// started. Only reachable for the detached-fallback launch path -- the
// systemd-run path never learns update.sh's own PID, since the process it
// directly observes (systemd-run) has already exited by the time it returns.
func recordUpdatePID(pid int) {
	updateMu.Lock()
	updatePID = pid
	updateMu.Unlock()
}

func recordUpdateUnit(unit string) {
	updateMu.Lock()
	updateUnit = unit
	updateMu.Unlock()
}

// panelUpdateUnitRunning reports the liveness of a transient systemd update
// unit. The second return value is false when systemd itself cannot be queried;
// an unknown result must keep the concurrency lock rather than risk launching
// two updaters. A known inactive/failed/not-found unit has finished and may be
// retried immediately even if update.sh failed before writing its status file.
var panelUpdateUnitRunning = queryPanelUpdateUnitRunning

func queryPanelUpdateUnitRunning(unit string) (bool, bool) {
	if strings.TrimSpace(unit) == "" {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "show", unit,
		"--property=LoadState", "--property=ActiveState", "--value").CombinedOutput()
	text := strings.ToLower(strings.TrimSpace(string(out)))
	if err != nil {
		if strings.Contains(text, "not found") || strings.Contains(text, "could not be found") {
			return false, true
		}
		return false, false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false, false
	}
	for _, field := range fields {
		if field == "not-found" || field == "inactive" || field == "failed" {
			return false, true
		}
	}
	for _, field := range fields {
		switch field {
		case "active", "activating", "reloading", "deactivating":
			return true, true
		}
	}
	return false, false
}

// previousRunIsTerminal reports whether the run currently recorded in
// updateRunID has reached success or failure per its status file. Must be
// called with updateMu held.
func previousRunIsTerminal() bool {
	status := (&PanelService{}).GetUpdateStatus()
	return status.RunID == strconv.FormatInt(updateRunID, 10) && status.State != updateStatePending
}

func releaseUpdateSlot() {
	updateMu.Lock()
	updateRunning = false
	updatePID = 0
	updateUnit = ""
	updateMu.Unlock()
}

func downloadPanelUpdater() (string, error) {
	client := (&service.SettingService{}).NewProxiedHTTPClient(15 * time.Second)
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, panelUpdaterDownloadURL(), nil)
	if reqErr != nil {
		return "", fmt.Errorf("download panel updater: %w", reqErr)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download panel updater: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download panel updater: unexpected HTTP %d", resp.StatusCode)
	}

	file, err := os.CreateTemp("", "3x-ui-update-*.sh")
	if err != nil {
		return "", err
	}
	path := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()

	n, err := io.Copy(file, io.LimitReader(resp.Body, maxPanelUpdaterBytes+1))
	if err != nil {
		return "", fmt.Errorf("write panel updater: %w", err)
	}
	if n == 0 {
		return "", fmt.Errorf("panel updater download is empty")
	}
	if n > maxPanelUpdaterBytes {
		return "", fmt.Errorf("panel updater exceeds %d bytes", maxPanelUpdaterBytes)
	}
	if err := file.Chmod(0o700); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

// panelUpdaterDownloadURL uses the immutable source revision of the running
// binary. This avoids executing a mutable main-branch script as root from the
// web updater. Stable builds fall back to their own version tag.
func panelUpdaterDownloadURL() string {
	// A rolling development build must also be able to receive updater fixes.
	// Pinning it to its old build commit permanently trapped broken updater
	// scripts. Stable dated releases remain pinned to immutable source.
	if config.IsDevBuild() {
		return panelRepositoryRaw + "/main/update.sh"
	}
	ref := panelUpdaterRef(config.GetBuildCommit(), config.GetBuildRelease(), config.GetBaseVersion())
	return panelRepositoryRaw + "/" + ref + "/update.sh"
}

func panelUpdaterRef(commit, release, baseVersion string) string {
	ref := strings.TrimSpace(commit)
	if !isCommitSHA(ref) {
		ref = strings.TrimSpace(release)
	}
	if ref == "" {
		ref = "v" + strings.TrimPrefix(strings.TrimSpace(baseVersion), "v")
	}
	return ref
}

func fetchLatestPanelVersion() (string, error) {
	release, err := fetchPanelRelease("")
	if err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("latest panel release tag is empty")
	}
	return release.TagName, nil
}

// fetchPanelRelease fetches a release from GitHub. An empty tag resolves the
// latest stable release; a non-empty tag (e.g. dev-latest) resolves that tag.
func fetchPanelRelease(tag string) (*service.Release, error) {
	url := panelRepositoryAPI + "/releases/latest"
	if tag != "" {
		url = panelRepositoryAPI + "/releases/tags/" + tag
	}
	client := (&service.SettingService{}).NewProxiedHTTPClient(10 * time.Second)
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, resp.Status)
	}

	var release service.Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// extractReleaseCommit reads the build commit recorded in the dev release: first
// the `commit=<sha>` marker the CI writes into the body, falling back to the
// tag's target commit.
func extractReleaseCommit(release *service.Release) string {
	if m := releaseCommitRegex.FindStringSubmatch(release.Body); m != nil {
		return strings.ToLower(m[1])
	}
	if isCommitSHA(release.TargetCommitish) {
		return strings.ToLower(release.TargetCommitish)
	}
	return ""
}

func isCommitSHA(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func shortCommit(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// commitsEqual compares a short (injected) commit against a full release commit
// by prefix, so an 8-char build stamp matches the 40-char release SHA.
func commitsEqual(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	return strings.HasPrefix(b, a)
}

func resolveUpdateFolders() (string, string) {
	mainFolder := os.Getenv("XUI_MAIN_FOLDER")
	if mainFolder == "" {
		if exePath, err := os.Executable(); err == nil {
			mainFolder = filepath.Dir(exePath)
		}
	}
	if mainFolder == "" {
		mainFolder = "/usr/local/x-ui"
	}

	serviceFolder := os.Getenv("XUI_SERVICE")
	if serviceFolder == "" {
		serviceFolder = "/etc/systemd/system"
	}
	return mainFolder, serviceFolder
}

func isNewerVersion(latest string, current string) bool {
	cmp, ok := version.Compare(latest, current)
	if !ok {
		if latestDate, latestRevision, latestOK := parseDatedRelease(latest); latestOK {
			if currentDate, currentRevision, currentOK := parseDatedRelease(current); currentOK {
				return latestDate > currentDate ||
					(latestDate == currentDate && latestRevision > currentRevision)
			}
		}
		return version.Normalize(latest) != version.Normalize(current)
	}
	return cmp > 0
}

func parseDatedRelease(value string) (date, revision int, ok bool) {
	match := datedReleaseRegex.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0, 0, false
	}
	date, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, 0, false
	}
	if match[2] != "" {
		revision, err = strconv.Atoi(match[2])
		if err != nil {
			return 0, 0, false
		}
	}
	return date, revision, true
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
