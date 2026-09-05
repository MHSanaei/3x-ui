package unattendedupgrades

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// runTimeout bounds a single unattended-upgrade invocation -- long enough
// for a real batch of security updates, short enough that a genuinely stuck
// run does not tie up this dashboard action forever. Killing the process on
// timeout carries a small residual risk of leaving one package
// mid-configured (the same risk any hard timeout on a package manager has),
// but that is still strictly safer than the no-timeout `apt upgrade -y`
// this feature replaced.
const runTimeout = 15 * time.Minute

// RunState is the terminal (or in-progress) state of the most recent RunNow.
type RunState string

const (
	RunPending RunState = "pending"
	RunRunning RunState = "running"
	RunSuccess RunState = "success"
	RunFailed  RunState = "failed"
)

// RunStatus is RunNow's own durable record of its most recent invocation,
// written to StatusPath so it survives a panel restart -- mirroring
// panel.PanelService's own "read the status file back, treat missing/stale
// as pending" shape for its self-update job.
type RunStatus struct {
	RunID     string   `json:"runId"`
	State     RunState `json:"state"`
	StartedAt int64    `json:"startedAt"`
	EndedAt   int64    `json:"endedAt,omitempty"`
	Tail      string   `json:"tail,omitempty"`
}

var runMu sync.Mutex

// RunNow launches `unattended-upgrade` in the background and returns
// immediately with a run ID -- a real run can take minutes, far too long
// for a single blocking HTTP request. Refuses to start a second run while
// one is already in flight rather than letting two invocations race over
// the same dpkg lock (the second would just fail on the lock anyway, but
// failing fast here gives a clearer error than dpkg's own).
func RunNow() (string, error) {
	runMu.Lock()
	defer runMu.Unlock()

	if current, err := readRunStatus(); err == nil && current.State == RunRunning &&
		!staleRunning(current) {
		return "", fmt.Errorf("an unattended-upgrade run is already in progress (started %s)",
			time.Unix(current.StartedAt, 0).Format(time.RFC3339))
	}

	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	startedAt := time.Now().Unix()
	if err := writeRunStatus(RunStatus{RunID: runID, State: RunRunning, StartedAt: startedAt}); err != nil {
		return "", err
	}

	go execute(runID, startedAt)
	return runID, nil
}

func execute(runID string, startedAt int64) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	// -v: enough output to be useful in the panel's own log view without
	// --debug's much larger volume. Never --dry-run -- RunNow means run it.
	cmd := exec.CommandContext(ctx, "unattended-upgrade", "-v")
	cmd.Env = noninteractiveEnv()
	out, runErr := cmd.CombinedOutput()

	status := RunStatus{
		RunID:     runID,
		State:     RunSuccess,
		StartedAt: startedAt,
		EndedAt:   time.Now().Unix(),
		Tail:      tail(string(out), 4000),
	}
	if runErr != nil {
		status.State = RunFailed
	}
	_ = os.MkdirAll(logDir(), 0o700)
	_ = os.WriteFile(LogPath(), out, 0o600)
	_ = writeRunStatus(status)
}

// staleRunning reports whether a recorded "running" status is actually just
// an abandoned record -- most likely the panel process itself restarted
// mid-run, so nothing will ever write this run's terminal state. Guarded by
// more than runTimeout so a genuinely still-running job is never mistaken
// for a stale one.
func staleRunning(status RunStatus) bool {
	return time.Now().Unix()-status.StartedAt > int64(runTimeout.Seconds())+60
}

// GetRunStatus reports the most recent RunNow invocation, read back from
// disk so it survives a panel restart. A missing file (never run yet) reads
// as RunPending, not an error, mirroring panel.PanelService.GetUpdateStatus.
func GetRunStatus() RunStatus {
	status, err := readRunStatus()
	if err != nil {
		return RunStatus{State: RunPending}
	}
	if status.State == RunRunning && staleRunning(status) {
		status.State = RunFailed
		status.Tail = "run status unknown: the panel process restarted before this run could report its own outcome"
	}
	return status
}

func readRunStatus() (RunStatus, error) {
	data, err := os.ReadFile(StatusPath())
	if err != nil {
		return RunStatus{}, err
	}
	var status RunStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return RunStatus{}, err
	}
	return status, nil
}

func writeRunStatus(status RunStatus) error {
	if err := os.MkdirAll(logDir(), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", logDir(), err)
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StatusPath(), data, 0o600)
}

// tail returns at most the last max bytes of s, so a very verbose run's
// output doesn't bloat the status file or the API response.
func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
