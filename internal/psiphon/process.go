package psiphon

import (
	"context"
	"errors"
	"fmt"
	"net"
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
// at a time. Structured notices go to NoticesPath instead; this only ever carries an early startup failure.
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
	logger.Infof("psiphon: %s", trimmed)
}

func (w *procLogWriter) LastLine() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastLine
}

// commandArgs builds the invocation, resolving every path to an absolute one
// since the panel's working directory is not guaranteed to be configDir().
func commandArgs() (string, []string, error) {
	bin, err := filepath.Abs(BinPath())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the Psiphon binary path: %w", err)
	}
	cfg, err := filepath.Abs(ConfigPath())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the Psiphon config path: %w", err)
	}
	data, err := filepath.Abs(dataDir())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the Psiphon data directory: %w", err)
	}
	notices, err := filepath.Abs(NoticesPath())
	if err != nil {
		return "", nil, fmt.Errorf("resolving the Psiphon notices path: %w", err)
	}
	return bin, []string{
		"-config", cfg,
		"-dataRootDirectory", data,
		"-notices", notices,
	}, nil
}

// Process wraps the single Psiphon ConsoleClient invocation.
type Process struct {
	mu              sync.RWMutex
	cmd             *exec.Cmd
	done            chan struct{}
	logWriter       *procLogWriter
	exitErr         error
	intentionalStop atomic.Bool
}

func newProcess() *Process { return &Process{logWriter: &procLogWriter{}} }

// IsRunning reports whether the Psiphon process is currently running.
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

// Start launches Psiphon against the admin-supplied config.
func (p *Process) Start() error {
	if p.IsRunning() {
		return errors.New("Psiphon is already running")
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
	logger.Errorf("psiphon: process exited: %v", err)
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
}

// Stop terminates the process gracefully, falling back to a kill. SIGTERM's
// own failure is tolerated, which also degrades correctly on Windows.
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

	logger.Warning("psiphon: did not stop after SIGTERM, killing process")
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
		return errors.New("timed out waiting for the Psiphon process to stop")
	}
}

// waitForListener blocks until the address accepts a connection, giving up
// early if the process died. Mirrors adguard.waitForListener.
func waitForListener(addr string, proc *Process) error {
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()
	dialer := &net.Dialer{Timeout: time.Second}
	for {
		if !proc.IsRunning() {
			return fmt.Errorf("Psiphon exited during startup: %s", proc.GetResult())
		}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Psiphon did not start listening on %s: %s", addr, proc.GetResult())
		case <-time.After(250 * time.Millisecond):
		}
	}
}
