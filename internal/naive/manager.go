package naive

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"gorm.io/gorm"
)

type InstanceStatus struct {
	Tag           string `json:"tag"`
	Running       bool   `json:"running"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	Error         string `json:"error,omitempty"`
}

type Manager struct {
	mu    sync.Mutex
	procs map[string]*Process
}

var (
	manager     *Manager
	managerOnce sync.Once
)

func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{procs: map[string]*Process{}}
	})
	return manager
}

func allocatePort(db *gorm.DB) (int, error) {
	for port := 30000; port <= 39999; port++ {
		var count int64
		if err := db.Model(&model.NaiveOutbound{}).Where("local_port = ?", port).Count(&count).Error; err != nil {
			return 0, err
		}
		if count > 0 {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, errors.New("no free port in range 30000-39999")
}

func AllocatePort(db *gorm.DB) (int, error) {
	return allocatePort(db)
}

func (m *Manager) StartAll() error {
	var records []model.NaiveOutbound
	if err := database.GetDB().Where("enabled = ?", true).Find(&records).Error; err != nil {
		return err
	}
	if !Installed() {
		if len(records) > 0 {
			logger.Warning("[naive] start skipped: binary is not installed, enabled outbounds: ", len(records))
		}
		return nil
	}
	for _, record := range records {
		if err := m.Start(record.Tag); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Start(tag string) error {
	var record model.NaiveOutbound
	if err := database.GetDB().Where("tag = ?", tag).First(&record).Error; err != nil {
		return err
	}
	if !record.Enabled {
		return nil
	}
	if !Installed() {
		logger.Warning("[naive/" + tag + "] start skipped: binary is not installed")
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if proc, ok := m.procs[tag]; ok && proc.IsRunning() {
		return nil
	}
	proc, err := Start(tag, ToConfig(&record))
	if err != nil {
		return err
	}
	m.procs[tag] = proc
	return nil
}

func (m *Manager) Stop(tag string) error {
	m.mu.Lock()
	proc := m.procs[tag]
	delete(m.procs, tag)
	m.mu.Unlock()
	if proc == nil {
		_ = os.Remove(ConfigPath(tag))
		return nil
	}
	return proc.Stop()
}

func (m *Manager) Restart(tag string) error {
	if err := m.Stop(tag); err != nil {
		return err
	}
	return m.Start(tag)
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	tags := make([]string, 0, len(m.procs))
	for tag := range m.procs {
		tags = append(tags, tag)
	}
	m.mu.Unlock()
	for _, tag := range tags {
		_ = m.Stop(tag)
	}
}

func (m *Manager) IsRunning(tag string) bool {
	m.mu.Lock()
	proc := m.procs[tag]
	m.mu.Unlock()
	return proc != nil && proc.IsRunning()
}

func (m *Manager) Statuses() ([]InstanceStatus, error) {
	var records []model.NaiveOutbound
	if err := database.GetDB().Order("tag asc").Find(&records).Error; err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	statuses := make([]InstanceStatus, 0, len(records))
	for _, record := range records {
		status := InstanceStatus{Tag: record.Tag}
		if proc := m.procs[record.Tag]; proc != nil {
			status.Running = proc.IsRunning()
			status.UptimeSeconds = proc.UptimeSeconds()
			status.Error = proc.LastError()
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Tag < statuses[j].Tag })
	return statuses, nil
}

func ReadLogLines(tag string, rows int) ([]string, error) {
	if rows <= 0 {
		return nil, errors.New("rows must be positive")
	}
	file, err := os.Open(ToConfig(&model.NaiveOutbound{Tag: tag}).Log)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()
	lines := make([]string, 0, rows)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > rows {
			lines = append([]string(nil), lines[len(lines)-rows:]...)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
