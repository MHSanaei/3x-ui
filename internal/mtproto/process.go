// Package mtproto manages mtg-multi (github.com/mhsanaei/mtg-multi) sidecar
// processes that serve MTProto FakeTLS proxies. Xray-core has no mtproto
// protocol, so mtproto inbounds are run as standalone mtg processes — one
// process per inbound, each serving every active client's secret through the
// mtg-multi [secrets] section — entirely outside the Xray config and lifecycle.
// A client edit is hot-applied via the fork's POST /reload endpoint so live
// connections survive; the manager falls back to a restart on older binaries.
package mtproto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// GetBinaryName returns the mtg binary filename for the current OS and arch,
// matching the naming scheme used for the Xray binary. On Windows the ".exe"
// extension is appended so a natural "mtg-windows-amd64.exe" is found.
func GetBinaryName() string {
	name := fmt.Sprintf("mtg-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// GetBinaryPath returns the full path to the mtg binary, alongside the Xray binary.
func GetBinaryPath() string {
	return config.GetBinFolderPath() + "/" + GetBinaryName()
}

func configDir() string {
	return config.GetBinFolderPath() + "/mtproto"
}

func configPathForID(id int) string {
	return fmt.Sprintf("%s/mtg-%d.toml", configDir(), id)
}

var (
	gracefulStopTimeout = 5 * time.Second
	forceStopTimeout    = 2 * time.Second
)

// procLogWriter consumes the mtg child process's stdout/stderr. It splits the
// stream into lines, forwards each one to the x-ui log — so mtg's own messages,
// including why it cannot reach Telegram, become visible in the panel log viewer
// and journald — and remembers the most recent line for GetResult.
type procLogWriter struct {
	mu       sync.Mutex
	label    string
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

// Flush emits any buffered partial line; called once the process exits so a
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
	logger.Infof("mtproto: mtg %s | %s", w.label, trimmed)
}

func (w *procLogWriter) LastLine() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastLine
}

// Process wraps a single mtg process invocation for one mtproto inbound.
type Process struct {
	mu              sync.RWMutex
	cmd             *exec.Cmd
	done            chan struct{}
	configPath      string
	logWriter       *procLogWriter
	exitErr         error
	intentionalStop atomic.Bool
}

func newProcess(configPath, label string) *Process {
	return &Process{
		configPath: configPath,
		logWriter:  &procLogWriter{label: label},
	}
}

// IsRunning reports whether the mtg process is currently running.
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

// GetResult returns the last log line or the exit error from the mtg process.
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

// Start launches the mtg process against its generated config file.
func (p *Process) Start() error {
	if p.IsRunning() {
		return errors.New("mtg is already running")
	}
	cmd := exec.CommandContext(context.Background(), GetBinaryPath(), "run", p.configPath)
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
	attachChildLifetime(cmd)
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
	if runtime.GOOS == "windows" {
		if strings.Contains(strings.ToLower(err.Error()), "exit status 1") {
			p.setExitErr(err)
			return
		}
	}
	logger.Errorf("mtproto: mtg process exited: %v", err)
	p.setExitErr(err)
}

func (p *Process) setExitErr(err error) {
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
}

// Stop terminates the running mtg process gracefully, falling back to a kill.
func (p *Process) Stop() error {
	if !p.IsRunning() {
		return errors.New("mtg is not running")
	}
	p.intentionalStop.Store(true)
	p.mu.RLock()
	cmd, done := p.cmd, p.done
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return errors.New("mtg is not running")
	}

	if runtime.GOOS == "windows" {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return waitForExit(done, forceStopTimeout)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return waitForExit(done, forceStopTimeout)
		}
		return err
	}

	if err := waitForExit(done, gracefulStopTimeout); err == nil {
		return nil
	}

	logger.Warning("mtproto: mtg did not stop after SIGTERM, killing process")
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
		return fmt.Errorf("timed out waiting for mtg process to stop after %s", timeout)
	}
}
