package vkturn

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
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type process struct {
	instance Instance
	cmd      *exec.Cmd
	done     chan struct{}
}

type Manager struct {
	mu         sync.Mutex
	binary     string
	processes  map[int]*process
	lastErrors map[int]string
}

func GetBinaryName() string {
	if runtime.GOOS == "windows" {
		return "vk-turn-proxy-server.exe"
	}
	return "vk-turn-proxy-server"
}

func GetBinaryPath() string { return filepath.Join(config.GetBinFolderPath(), GetBinaryName()) }

var manager = newManager("")

func GetManager() *Manager { return manager }

func newManager(binary string) *Manager {
	return &Manager{binary: binary, processes: make(map[int]*process), lastErrors: make(map[int]string)}
}

func (m *Manager) Ensure(instance Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(instance)
}

func (m *Manager) ensureLocked(instance Instance) error {
	if previous, ok := m.processes[instance.ID]; ok {
		if previous.instance == instance && previous.running() {
			return nil
		}
		if err := m.removeLocked(instance.ID); err != nil {
			return err
		}
	}
	binary := m.binary
	if binary == "" {
		binary = GetBinaryPath()
	}
	cmd := exec.CommandContext(context.Background(), binary, "-listen", instance.Listen, "-connect", instance.Connect, "-srtp")
	configureChild(cmd)
	writer := &logWriter{id: instance.ID}
	cmd.Stdout, cmd.Stderr = writer, writer
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("VK TURN inbound %d: start %s: %w", instance.ID, binary, err)
	}
	p := &process{instance: instance, cmd: cmd, done: make(chan struct{})}
	m.processes[instance.ID] = p
	delete(m.lastErrors, instance.ID)
	go func() {
		_ = cmd.Wait()
		writer.flush()
		close(p.done)
	}()
	return nil
}

func (p *process) running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (m *Manager) Remove(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removeLocked(id)
}

func (m *Manager) removeLocked(id int) error {
	p, ok := m.processes[id]
	if !ok {
		return nil
	}
	if p.running() {
		var err error
		if runtime.GOOS == "windows" {
			err = p.cmd.Process.Kill()
		} else {
			err = p.cmd.Process.Signal(syscall.SIGTERM)
		}
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-p.done:
		case <-timer.C:
			if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return err
			}
			<-p.done
		}
	}
	delete(m.processes, id)
	delete(m.lastErrors, id)
	return nil
}

func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keep := make(map[int]bool, len(desired))
	for _, instance := range desired {
		keep[instance.ID] = true
	}
	for id := range m.processes {
		if !keep[id] {
			m.logError(id, m.removeLocked(id))
		}
	}
	for _, instance := range desired {
		m.logError(instance.ID, m.ensureLocked(instance))
	}
}

func (m *Manager) logError(id int, err error) {
	if err != nil && m.lastErrors[id] != err.Error() {
		logger.Warningf("VK TURN inbound %d: %v", id, err)
		m.lastErrors[id] = err.Error()
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.processes {
		m.logError(id, m.removeLocked(id))
	}
}

type logWriter struct {
	mu      sync.Mutex
	id      int
	pending string
}

func (w *logWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending += string(data)
	for {
		line, rest, found := strings.Cut(w.pending, "\n")
		if !found {
			break
		}
		w.pending = rest
		logger.Infof("VK TURN inbound %d: %s", w.id, strings.TrimSpace(line))
	}
	if len(w.pending) > 8192 {
		w.pending = w.pending[len(w.pending)-8192:]
	}
	return len(data), nil
}

func (w *logWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending != "" {
		logger.Infof("VK TURN inbound %d: %s", w.id, strings.TrimSpace(w.pending))
		w.pending = ""
	}
}
