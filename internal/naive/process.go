package naive

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type Process struct {
	tag        string
	configPath string
	cmd        *exec.Cmd
	logWriter  *LogWriter
	startedAt  time.Time
	mu         sync.RWMutex
	done       chan struct{}
	exitErr    error
	stopping   bool
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "naive.exe"
	}
	return "naive"
}

func BinaryPath() string {
	return filepath.Join(config.GetBinFolderPath(), binaryName())
}

func ConfigPath(tag string) string {
	return filepath.Join(os.TempDir(), "naive-"+tag+".json")
}

func hostFromProxy(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func killStaleByTag(tag string) {
	if runtime.GOOS == "windows" {
		return
	}
	cfg := ConfigPath(tag)
	_ = exec.Command("pkill", "-f", cfg).Run()
}

func Start(tag string, cfg Config) (*Process, error) {
	if err := ValidateTag(tag); err != nil {
		return nil, err
	}
	if err := ValidateProxyURL(cfg.Proxy); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.GetLogFolder(), 0o755); err != nil {
		return nil, err
	}
	path := ConfigPath(tag)
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return nil, err
	}
	proc := &Process{
		tag:        tag,
		configPath: path,
		logWriter:  NewLogWriter(tag),
		startedAt:  time.Now(),
		done:       make(chan struct{}),
	}

	killStaleByTag(tag)
	cmd := exec.Command(BinaryPath(), path)
	cmd.Stdout = proc.logWriter
	cmd.Stderr = proc.logWriter
	if err := cmd.Start(); err != nil {
		_ = os.Remove(path)
		logger.Error("[naive/" + tag + "] start failed -> " + hostFromProxy(cfg.Proxy) + ": " + err.Error())
		return nil, err
	}
	proc.cmd = cmd
	logger.Info("[naive/" + tag + "] started -> " + hostFromProxy(cfg.Proxy))
	go proc.wait()
	return proc, nil
}

func (p *Process) wait() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.exitErr = err
	stopping := p.stopping
	p.mu.Unlock()
	if err != nil && !stopping {
		logger.Error("[naive/" + p.tag + "] exited: " + err.Error())
	}
	close(p.done)
}

func waitDone(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func (p *Process) Stop() error {
	p.mu.Lock()
	if p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		_ = os.Remove(p.configPath)
		return nil
	}
	p.stopping = true
	proc := p.cmd.Process
	done := p.done
	p.mu.Unlock()

	if runtime.GOOS == "windows" {
		_ = proc.Kill()
		_ = waitDone(done, 5*time.Second)
	} else {
		_ = proc.Signal(syscall.SIGTERM)
		if !waitDone(done, 5*time.Second) {
			_ = proc.Kill()
			_ = waitDone(done, 2*time.Second)
		}
	}
	_ = os.Remove(p.configPath)
	logger.Info("[naive/" + p.tag + "] stopped")
	return nil
}

func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *Process) UptimeSeconds() int64 {
	if !p.IsRunning() {
		return 0
	}
	return int64(time.Since(p.startedAt).Seconds())
}

func (p *Process) LastError() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.exitErr == nil {
		return ""
	}
	return p.exitErr.Error()
}

func (p *Process) Logs(rows int) []string {
	return p.logWriter.GetLogs(rows)
}

func Installed() bool {
	_, err := os.Stat(BinaryPath())
	return err == nil
}

func InstalledVersion() string {
	if !Installed() {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, BinaryPath(), "--version").CombinedOutput()
	if err != nil && len(output) == 0 {
		return ""
	}
	line := strings.TrimSpace(string(output))
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func UninstallBinary() error {
	if err := os.Remove(BinaryPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
