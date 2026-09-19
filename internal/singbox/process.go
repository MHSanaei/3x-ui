package singbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	corepkg "github.com/mhsanaei/3x-ui/v3/internal/core"
)

const (
	defaultGracefulStopTimeout = 5 * time.Second
	defaultForceStopTimeout   = 2 * time.Second
	defaultCommandTimeout     = 10 * time.Second
)

// GetBinaryName returns the sing-box binary filename for the current platform.
func GetBinaryName() string {
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "arm32"
	}
	return fmt.Sprintf("sing-box-%s-%s", runtime.GOOS, arch)
}

// GetBinaryPath returns the panel-managed sing-box binary path.
func GetBinaryPath() string {
	return filepath.Join(config.GetBinFolderPath(), GetBinaryName())
}

// GetConfigPath returns the panel-managed sing-box configuration path.
func GetConfigPath() string {
	return filepath.Join(config.GetBinFolderPath(), "sing-box.json")
}

// Process manages one panel-owned sing-box process.
type Process struct {
	mu        sync.RWMutex
	lifecycle sync.Mutex
	cmd       *exec.Cmd
	done      chan struct{}
	exitErr   error
	version   string
	startTime time.Time
	config    string
}

func NewProcess(configPath string) *Process {
	return &Process{config: configPath, version: "Unknown"}
}

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

func (p *Process) GetErr() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitErr
}

func (p *Process) GetVersion() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.version
}

func (p *Process) GetStartTime() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.startTime
}

func (p *Process) ConfigPath() string {
	return p.config
}

func (p *Process) Validate(ctx context.Context) error {
	binary := GetBinaryPath()
	if _, err := os.Stat(binary); err != nil {
		return fmt.Errorf("sing-box binary is not installed: %w", err)
	}
	if _, err := os.Stat(p.config); err != nil {
		return fmt.Errorf("sing-box config does not exist: %w", err)
	}
	cmdCtx, cancel := context.WithTimeout(ctx, defaultCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, binary, "check", "-c", p.config)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box config check failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (p *Process) Version(ctx context.Context) (string, error) {
	binary := GetBinaryPath()
	cmdCtx, cancel := context.WithTimeout(ctx, defaultCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, binary, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sing-box version failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	version := strings.TrimSpace(string(output))
	p.mu.Lock()
	p.version = version
	p.mu.Unlock()
	return version, nil
}

func (p *Process) Start(ctx context.Context) error {
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	return p.startLocked(ctx)
}

func (p *Process) startLocked(ctx context.Context) error {
	p.mu.RLock()
	running := p.cmd != nil && p.cmd.Process != nil
	done := p.done
	p.mu.RUnlock()
	if running {
		select {
		case <-done:
		default:
			return nil
		}
	}

	if err := p.Validate(ctx); err != nil {
		return err
	}

	binary := GetBinaryPath()
	cmd := exec.Command(binary, "run", "-c", p.config)
	cmd.Stdout = &processOutput{p: p}
	cmd.Stderr = &processOutput{p: p}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sing-box: %w", err)
	}

	p.mu.Lock()
	p.cmd = cmd
	p.done = make(chan struct{})
	p.exitErr = nil
	p.startTime = time.Now()
	done = p.done
	p.mu.Unlock()

	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.exitErr = err
		close(done)
		p.mu.Unlock()
	}()
	return nil
}

func (p *Process) Stop() error {
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	return p.stopLocked()
}

func (p *Process) stopLocked() error {
	p.mu.RLock()
	cmd, done := p.cmd, p.done
	p.mu.RUnlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		_ = cmd.Process.Kill()
	}

	timer := time.NewTimer(defaultGracefulStopTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
	}

	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	timer.Reset(defaultForceStopTimeout)
	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("timed out stopping sing-box")
	}
}

func (p *Process) Restart(ctx context.Context) error {
	p.lifecycle.Lock()
	defer p.lifecycle.Unlock()
	if err := p.stopLocked(); err != nil {
		return err
	}
	return p.startLocked(ctx)
}

type processOutput struct {
	p *Process
}

func (w *processOutput) Write(b []byte) (int, error) {
	// Keep stdout/stderr drained so the child process cannot block on a full pipe.
	// The lifecycle error is captured by Wait.
	return len(b), nil
}

var _ corepkg.Runtime = (*Runtime)(nil)

// Runtime adapts the process to the common core.Runtime contract.
type Runtime struct {
	process *Process
}

func NewRuntime(configPath string) *Runtime {
	return &Runtime{process: NewProcess(configPath)}
}

func (r *Runtime) Type() corepkg.Type { return corepkg.SingBox }
func (r *Runtime) Version(ctx context.Context) (string, error) {
	return r.process.Version(ctx)
}
func (r *Runtime) Start(ctx context.Context) error { return r.process.Start(ctx) }
func (r *Runtime) Stop(ctx context.Context) error {
	return r.process.Stop()
}
func (r *Runtime) Restart(ctx context.Context) error { return r.process.Restart(ctx) }
func (r *Runtime) IsRunning() bool { return r.process.IsRunning() }
func (r *Runtime) ValidateConfig(ctx context.Context) error {
	return r.process.Validate(ctx)
}
func (r *Runtime) Process() *Process { return r.process }
