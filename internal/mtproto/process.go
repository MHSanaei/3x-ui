// Package mtproto manages mtg (github.com/9seconds/mtg) sidecar processes that
// serve MTProto FakeTLS proxies. Xray-core has no mtproto protocol, so mtproto
// inbounds are run as standalone mtg processes — one process per inbound —
// entirely outside the Xray config and lifecycle.
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

// GetResult returns the last log line or the exit error from the mtg process.
func (p *Process) GetResult() string {
	if line := p.logWriter.LastLine(); line != "" {
		return line
	}
	if p.exitErr != nil {
		return p.exitErr.Error()
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
	go p.wait(cmd)
	return nil
}

func (p *Process) wait(cmd *exec.Cmd) {
	defer close(p.done)
	err := cmd.Wait()
	p.logWriter.Flush()
	if err == nil || p.intentionalStop.Load() {
		return
	}
	if runtime.GOOS == "windows" {
		if strings.Contains(strings.ToLower(err.Error()), "exit status 1") {
			p.exitErr = err
			return
		}
	}
	logger.Errorf("mtproto: mtg process exited: %v", err)
	p.exitErr = err
}

// Stop terminates the running mtg process gracefully, falling back to a kill.
func (p *Process) Stop() error {
	if !p.IsRunning() {
		return errors.New("mtg is not running")
	}
	p.intentionalStop.Store(true)

	if runtime.GOOS == "windows" {
		if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return p.waitForExit(forceStopTimeout)
	}

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return p.waitForExit(forceStopTimeout)
		}
		return err
	}

	if err := p.waitForExit(gracefulStopTimeout); err == nil {
		return nil
	}

	logger.Warning("mtproto: mtg did not stop after SIGTERM, killing process")
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return p.waitForExit(forceStopTimeout)
}

func (p *Process) waitForExit(timeout time.Duration) error {
	if p.done == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return nil
	case <-timer.C:
		return fmt.Errorf("timed out waiting for mtg process to stop after %s", timeout)
	}
}
