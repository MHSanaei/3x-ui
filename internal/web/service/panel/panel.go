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
	panelUpdaterURL      = "https://raw.githubusercontent.com/MHSanaei/3x-ui/main/update.sh"
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

var releaseCommitRegex = regexp.MustCompile(`(?i)commit=([0-9a-f]{7,40})`)

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
)

const (
	updateStaleAfter  = 20 * time.Minute
	updateHardCeiling = 2 * time.Hour
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
	current := config.GetBaseVersion()
	return &PanelUpdateInfo{
		Channel:         "stable",
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: isNewerVersion(latest, current),
	}, nil
}

// devChannelActive reports whether self-update should track the rolling dev
// release. It is driven solely by the opt-in setting so the panel can
// cross-grade a stable build onto the dev channel once the user enables it;
// nothing updates without an explicit user action, so an unattended stable
// binary with the toggle off stays on the stable channel.
func devChannelActive() bool {
	enabled, err := (&service.SettingService{}).GetDevChannelEnable()
	return err == nil && enabled
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
	if !acquireUpdateSlot(runID) {
		return 0, fmt.Errorf("a panel update is already in progress")
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

	statusFile := config.GetUpdateStatusFilePath()

	mainFolder, serviceFolder := resolveUpdateFolders()
	updateTag := ""
	if useDev {
		updateTag = devReleaseTag
	}
	updateScript := fmt.Sprintf("set -e; trap 'rm -f %s' EXIT; %s %s", shellQuote(scriptPath), shellQuote(bash), shellQuote(scriptPath))
	runIDEnv := "XUI_UPDATE_RUN_ID=" + strconv.FormatInt(runID, 10)
	statusFileEnv := "XUI_UPDATE_STATUS_FILE=" + statusFile

	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		unitName := fmt.Sprintf("x-ui-web-update-%d", time.Now().Unix())
		cmd := exec.CommandContext(context.Background(), systemdRun,
			"--unit", unitName,
			"--setenv", "XUI_MAIN_FOLDER="+mainFolder,
			"--setenv", "XUI_SERVICE="+serviceFolder,
			"--setenv", "XUI_UPDATE_TAG="+updateTag,
			"--setenv", runIDEnv,
			"--setenv", statusFileEnv,
			bash, "-lc", updateScript,
		)
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
	updateMu.Lock()
	defer updateMu.Unlock()
	if updateRunning && !previousRunIsTerminal() {
		elapsed := time.Since(updateStarted)
		if elapsed < updateHardCeiling {
			stale := elapsed >= updateStaleAfter
			alive := updatePID > 0 && processAlive(updatePID)
			if !stale || alive {
				return false
			}
		}
	}
	updateRunning = true
	updateStarted = time.Now()
	updateRunID = runID
	updatePID = 0
	return true
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
	updateMu.Unlock()
}

func downloadPanelUpdater() (string, error) {
	client := (&service.SettingService{}).NewProxiedHTTPClient(15 * time.Second)
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, panelUpdaterURL, nil)
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
	url := "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest"
	if tag != "" {
		url = "https://api.github.com/repos/MHSanaei/3x-ui/releases/tags/" + tag
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
	cmp, ok := compareVersionStrings(latest, current)
	if !ok {
		return normalizeVersionTag(latest) != normalizeVersionTag(current)
	}
	return cmp > 0
}

func compareVersionStrings(a string, b string) (int, bool) {
	aParts, okA := parseVersionParts(a)
	bParts, okB := parseVersionParts(b)
	if !okA || !okB {
		return 0, false
	}
	for i := range len(aParts) {
		if aParts[i] > bParts[i] {
			return 1, true
		}
		if aParts[i] < bParts[i] {
			return -1, true
		}
	}
	return 0, true
}

func parseVersionParts(version string) ([3]int, bool) {
	var result [3]int
	parts := strings.Split(normalizeVersionTag(version), ".")
	if len(parts) != 3 {
		return result, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return result, false
		}
		result[i] = n
	}
	return result, true
}

func normalizeVersionTag(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
