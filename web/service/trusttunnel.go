package service

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/trusttunnel"
)

// Package-level TrustTunnel service instance, set during server startup.
// Used by InboundService to trigger TrustTunnel restarts on inbound changes.
var ttService *TrustTunnelService

// SetTrustTunnelService sets the package-level TrustTunnel service instance.
func SetTrustTunnelService(s *TrustTunnelService) {
	ttService = s
}

// GetTrustTunnelService returns the package-level TrustTunnel service instance.
func GetTrustTunnelService() *TrustTunnelService {
	return ttService
}

// TrustTunnelService manages TrustTunnel endpoint processes.
// Each TrustTunnel inbound runs its own process alongside Xray.
type TrustTunnelService struct {
	mu        sync.Mutex
	processes map[string]*trusttunnel.Process // keyed by inbound tag
}

func NewTrustTunnelService() *TrustTunnelService {
	return &TrustTunnelService{
		processes: make(map[string]*trusttunnel.Process),
	}
}

// StartAll starts TrustTunnel processes for all enabled TrustTunnel inbounds.
func (s *TrustTunnelService) StartAll() {
	if !trusttunnel.IsBinaryInstalled() {
		logger.Debug("TrustTunnel binary not installed, skipping")
		return
	}

	inbounds, err := s.getTrustTunnelInbounds()
	if err != nil {
		logger.Warning("Failed to get TrustTunnel inbounds:", err)
		return
	}

	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		if err := s.startInbound(inbound); err != nil {
			logger.Warningf("Failed to start TrustTunnel for %s: %v", inbound.Tag, err)
		}
	}
}

// StopAll stops all running TrustTunnel processes.
func (s *TrustTunnelService) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for tag, proc := range s.processes {
		if proc.IsRunning() {
			if err := proc.Stop(); err != nil {
				logger.Warningf("Failed to stop TrustTunnel %s: %v", tag, err)
			}
		}
	}
	s.processes = make(map[string]*trusttunnel.Process)
}

// RestartForInbound restarts the TrustTunnel process for a specific inbound.
func (s *TrustTunnelService) RestartForInbound(inbound *model.Inbound) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing process for this tag
	if proc, ok := s.processes[inbound.Tag]; ok && proc.IsRunning() {
		proc.Stop()
	}

	if !inbound.Enable {
		delete(s.processes, inbound.Tag)
		return nil
	}

	return s.startInboundLocked(inbound)
}

// StopForInbound stops the TrustTunnel process for a specific inbound tag.
func (s *TrustTunnelService) StopForInbound(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if proc, ok := s.processes[tag]; ok {
		if proc.IsRunning() {
			proc.Stop()
		}
		delete(s.processes, tag)
	}
}

// IsRunning checks if a TrustTunnel process is running for the given tag.
func (s *TrustTunnelService) IsRunning(tag string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if proc, ok := s.processes[tag]; ok {
		return proc.IsRunning()
	}
	return false
}

// CheckAndRestart checks for crashed TrustTunnel processes and restarts them.
func (s *TrustTunnelService) CheckAndRestart() {
	if !trusttunnel.IsBinaryInstalled() {
		return
	}

	inbounds, err := s.getTrustTunnelInbounds()
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		proc, ok := s.processes[inbound.Tag]
		if !ok || !proc.IsRunning() {
			logger.Infof("TrustTunnel %s not running, restarting...", inbound.Tag)
			s.startInboundLocked(inbound)
		}
	}
}

func (s *TrustTunnelService) startInbound(inbound *model.Inbound) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startInboundLocked(inbound)
}

func (s *TrustTunnelService) startInboundLocked(inbound *model.Inbound) error {
	settings, err := trusttunnel.ParseSettings(inbound.Settings)
	if err != nil {
		return err
	}

	proc := trusttunnel.NewProcess(inbound.Tag)

	if err := proc.WriteConfig(inbound.Listen, inbound.Port, settings); err != nil {
		return err
	}

	if err := proc.Start(); err != nil {
		return err
	}

	s.processes[inbound.Tag] = proc
	logger.Infof("TrustTunnel started for inbound %s on port %d", inbound.Tag, inbound.Port)
	return nil
}

func (s *TrustTunnelService) getTrustTunnelInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("protocol = ?", model.TrustTunnel).Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}
