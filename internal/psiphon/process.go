package psiphon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var (
	gracefulStopTimeout = 5 * time.Second
	forceStopTimeout    = 2 * time.Second
)

// GetBinaryName returns the psiphon binary filename for the current OS and architecture.
func GetBinaryName() string {
	name := fmt.Sprintf("psiphon-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// GetBinaryPath returns the absolute path to the psiphon binary.
func GetBinaryPath() string {
	binFolder := config.GetBinFolderPath()
	candidates := []string{
		filepath.Join(binFolder, GetBinaryName()),
		filepath.Join(binFolder, "psiphon-core"),
		filepath.Join(binFolder, "ConsoleClient"),
		filepath.Join("..", "..", "bin", GetBinaryName()),
		filepath.Join("..", "..", "bin", "psiphon-core"),
		filepath.Join("..", "..", "bin", "ConsoleClient"),
		"/tmp/ConsoleClient",
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			filepath.Join(binFolder, "psiphon-core.exe"),
			filepath.Join(binFolder, "ConsoleClient.exe"),
		)
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return candidates[0]
}

// HasBinary reports whether a psiphon binary exists on the system.
func HasBinary() bool {
	p := GetBinaryPath()
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// NoticeHandler defines a callback for notice events.
type NoticeHandler func(notice Notice)

type procNoticeWriter struct {
	mu            sync.Mutex
	label         string
	buf           string
	lastLine      string
	noticeHandler NoticeHandler
}

func (w *procNoticeWriter) Write(p []byte) (int, error) {
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
		w.handleLineLocked(line)
	}
	return len(p), nil
}

func (w *procNoticeWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != "" {
		line := w.buf
		w.buf = ""
		w.handleLineLocked(line)
	}
}

func (w *procNoticeWriter) handleLineLocked(line string) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmed == "" {
		return
	}
	w.lastLine = trimmed
	var n Notice
	if err := json.Unmarshal([]byte(trimmed), &n); err == nil && n.NoticeType != "" {
		if w.noticeHandler != nil {
			w.noticeHandler(n)
		}
		return
	}
	logger.Infof("psiphon: %s | %s", w.label, trimmed)
}

func (w *procNoticeWriter) LastLine() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastLine
}

// Process manages one Psiphon ConsoleClient child process.
type Process struct {
	mu              sync.RWMutex
	cmd             *exec.Cmd
	done            chan struct{}
	configPath      string
	dataDir         string
	label           string
	writer          *procNoticeWriter
	exitErr         error
	intentionalStop atomic.Bool
}

// NewProcess creates a new unstarted Process.
func NewProcess(configPath, dataDir, label string, handler NoticeHandler) *Process {
	return &Process{
		configPath: configPath,
		dataDir:    dataDir,
		label:      label,
		writer: &procNoticeWriter{
			label:         label,
			noticeHandler: handler,
		},
	}
}

// IsRunning reports whether the process is currently active.
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

// GetResult returns the last log line or exit error.
func (p *Process) GetResult() string {
	if line := p.writer.LastLine(); line != "" {
		return line
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.exitErr != nil {
		return p.exitErr.Error()
	}
	return ""
}

// Start executes the Psiphon ConsoleClient process.
func (p *Process) Start() error {
	if p.IsRunning() {
		return errors.New("psiphon is already running")
	}
	binPath := GetBinaryPath()
	if !HasBinary() {
		return fmt.Errorf("psiphon binary not found at %s", binPath)
	}

	cmd := exec.CommandContext(context.Background(), binPath, "-config", p.configPath, "-dataRootDirectory", p.dataDir)
	cmd.Stdout = p.writer
	cmd.Stderr = p.writer
	if runtime.GOOS == "linux" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

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
	p.writer.Flush()
	if err == nil || p.intentionalStop.Load() {
		return
	}
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
	logger.Errorf("psiphon: process %s exited: %v", p.label, err)
}

// Stop gracefully shuts down the process with fallback to SIGKILL.
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

	logger.Warningf("psiphon: instance %s did not stop after SIGTERM, killing", p.label)
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
		return fmt.Errorf("timed out waiting for psiphon process to stop after %s", timeout)
	}
}
