package integration

import (
	"github.com/mhsanaei/3x-ui/v3/internal/tor"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// TorService manages the panel's own Tor sidecar (see internal/tor) and
// remembers whether it should be running across restarts.
type TorService struct {
	service.SettingService
}

// TorStatus reports the managed Tor daemon's current state to the UI.
type TorStatus struct {
	Available bool   `json:"available"`
	Running   bool   `json:"running"`
	Port      int    `json:"port"`
	LastLog   string `json:"lastLog,omitempty"`
}

func (s *TorService) Status() TorStatus {
	return TorStatus{
		Available: tor.IsAvailable(),
		Running:   tor.GetManager().IsRunning(),
		Port:      tor.SocksPort,
		LastLog:   tor.GetManager().LastResult(),
	}
}

// Start launches the managed tor daemon and persists the choice so panel
// boot brings it back up automatically (see the auto-start in web.go).
func (s *TorService) Start() error {
	if err := tor.GetManager().Start(); err != nil {
		return err
	}
	return s.SetTorEnable(true)
}

// Stop stops the managed tor daemon and persists the choice, mirroring Start.
func (s *TorService) Stop() error {
	if err := tor.GetManager().Stop(); err != nil {
		return err
	}
	return s.SetTorEnable(false)
}
