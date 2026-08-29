package adguard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var (
	gracefulStopTimeout = 5 * time.Second
	forceStopTimeout    = 2 * time.Second
)

// procLogWriter forwards the child's stdout/stderr into the panel log a line
// at a time, so AdGuard Home's own startup errors are visible in the log
// viewer instead of vanishing with the pipe.
type procLogWriter struct {
	mu       sync.Mutex
	buf      string
	lastLine string
}

func (w *procLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf += string(p)
	for {
		i := strings.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		w.emitLocked(line)
	}
	return len(p), nil
}

// Flush emits a buffered partial line, called once the process exits so a
// final un-terminated error line is not lost.
func (w *procLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != "" {
		line := w.buf
		w.buf = ""
		w.emitLocked(line)
	}
}

func (w *procLogWriter) emitLocked(line string) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmed == "" {
		return
	}
	w.lastLine = trimmed
	logger.Infof("adguard: %s", trimmed)
}

func (w *procLogWriter) LastLine() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastLine
}

// commandArgs builds the invocation, resolving every path to an absolute one.
//
// Absolute is not cosmetic here. AdGuard Home resolves a relative --config
// against its --work-dir rather than against the current directory, so a
// relative pair points at <work-dir>/<work-dir>/AdGuardHome.yaml, which does
// not exist -- and a missing config is how AdGuard Home decides it is being
// launched for the first time. The failure is silent and looks like the panel
// never wrote a config: the seeded file is sitting right there, untouched,
// while the admin is shown the setup wizard.
//
// The panel's bin folder is relative ("bin") unless XUI_BIN_FOLDER overrides
// it, so this is the normal case, not an edge one.
func commandArgs() (string, []string, error) {
	bin, err := filepath.Abs(BinPath())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the AdGuard Home binary path: %w", err)
	}
	configPath, err := filepath.Abs(ConfigPath())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the AdGuard Home config path: %w", err)
	}
	workDir, err := filepath.Abs(Dir())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the AdGuard Home work directory: %w", err)
	}
	// --no-check-update: this install is managed by the panel, so the binary
	// nagging about a release it cannot install itself is noise.
	return bin, []string{
		"--config", configPath,
		"--work-dir", workDir,
		"--no-check-update",
	}, nil
}

// Process wraps the single AdGuard Home invocation.
type Process struct {
	mu              sync.RWMutex
	cmd             *exec.Cmd
	done            chan struct{}
	logWriter       *procLogWriter
	exitErr         error
	intentionalStop atomic.Bool
}

func newProcess() *Process { return &Process{logWriter: &procLogWriter{}} }

// IsRunning reports whether the AdGuard Home process is currently running.
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	cmd, done := p.cmd, p.done
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if done != nil {
		select {
		case <-done:
			return false
		default:
		}
	}
	return true
}

// GetResult returns the last log line or the exit error from the process.
func (p *Process) GetResult() string {
	if line := p.logWriter.LastLine(); line != "" {
		return line
	}
	p.mu.RLock()
	exitErr := p.exitErr
	p.mu.RUnlock()
	if exitErr != nil {
		return exitErr.Error()
	}
	return ""
}

// Start launches AdGuard Home against the seeded config.
func (p *Process) Start() error {
	if p.IsRunning() {
		return errors.New("AdGuard Home is already running")
	}
	bin, args, err := commandArgs()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Stdout = p.logWriter
	cmd.Stderr = p.logWriter
	done := make(chan struct{})
	p.mu.Lock()
	p.cmd = cmd
	p.done = done
	p.exitErr = nil
	p.mu.Unlock()
	p.intentionalStop.Store(false)
	if err := cmd.Start(); err != nil {
		close(done)
		p.mu.Lock()
		p.cmd = nil
		p.mu.Unlock()
		return err
	}
	go p.wait(cmd, done)
	return nil
}

func (p *Process) wait(cmd *exec.Cmd, done chan struct{}) {
	defer close(done)
	err := cmd.Wait()
	p.logWriter.Flush()
	if err == nil || p.intentionalStop.Load() {
		return
	}
	logger.Errorf("adguard: process exited: %v", err)
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
}

// Stop terminates the process gracefully, falling back to a kill. SIGTERM is
// attempted first and its failure tolerated, which is also how this ends up
// doing the right thing on Windows, where signals are not supported.
func (p *Process) Stop() error {
	if !p.IsRunning() {
		return nil
	}
	p.intentionalStop.Store(true)
	p.mu.RLock()
	cmd, done := p.cmd, p.done
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return waitForExit(done, forceStopTimeout)
		}
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return waitForExit(done, forceStopTimeout)
	}

	if err := waitForExit(done, gracefulStopTimeout); err == nil {
		return nil
	}

	logger.Warning("adguard: did not stop after SIGTERM, killing process")
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return waitForExit(done, forceStopTimeout)
}

func waitForExit(done <-chan struct{}, timeout time.Duration) error {
	if done == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("timed out waiting for the AdGuard Home process to stop")
	}
}
