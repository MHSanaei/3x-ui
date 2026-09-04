package integration

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/psiphon"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// PsiphonService manages the panel's own Psiphon sidecar (internal/psiphon)
// and remembers whether it should run across restarts. Mirrors TorService.
type PsiphonService struct {
	service.SettingService
}

// PsiphonStatus reports process state to the UI: Installed/Configured gate
// whether Start can be tried, Running/Tunnel are the process's own cheap view.
type PsiphonStatus struct {
	Installed  bool                 `json:"installed"`
	Configured bool                 `json:"configured"`
	Running    bool                 `json:"running"`
	Port       int                  `json:"port"`
	Tunnel     psiphon.TunnelStatus `json:"tunnel"`
	LastLog    string               `json:"lastLog,omitempty"`
}

func (s *PsiphonService) Status() (PsiphonStatus, error) {
	tunnel, err := psiphon.CurrentTunnel()
	if err != nil {
		return PsiphonStatus{}, err
	}
	return PsiphonStatus{
		Installed:  psiphon.IsInstalled(),
		Configured: psiphon.IsConfigured(),
		Running:    psiphon.GetManager().IsRunning(),
		Port:       psiphon.SocksPort,
		Tunnel:     tunnel,
		LastLog:    psiphon.GetManager().LastResult(),
	}, nil
}

// AutoStart brings Psiphon up at boot if enabled; never errors, mirroring
// AdGuardService.AutoStart, plus an IsConfigured gate AdGuard has no equivalent of.
func (s *PsiphonService) AutoStart() {
	enabled, err := s.GetPsiphonEnable()
	if err != nil || !enabled {
		return
	}
	if !psiphon.IsInstalled() {
		logger.Warning("psiphon: enabled but not installed, staying down")
		return
	}
	if !psiphon.IsConfigured() {
		logger.Warning("psiphon: enabled but no config uploaded yet, staying down")
		return
	}
	if err := psiphon.GetManager().Start(); err != nil {
		logger.Warningf("psiphon: failed to auto-start on boot: %v", err)
	}
}

// Start launches the managed Psiphon process and persists the choice so panel
// boot brings it back up automatically (see the auto-start in web.go).
func (s *PsiphonService) Start() error {
	if err := psiphon.GetManager().Start(); err != nil {
		return err
	}
	return s.SetPsiphonEnable(true)
}

// Stop stops the managed Psiphon process and persists the choice, mirroring
// Start.
func (s *PsiphonService) Stop() error {
	if err := psiphon.GetManager().Stop(); err != nil {
		return err
	}
	return s.SetPsiphonEnable(false)
}

// Install downloads the pinned Psiphon ConsoleClient release via the panel's
// own proxied HTTP client. installTimeout is shared with AdGuardService (adguard.go).
func (s *PsiphonService) Install() error {
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()
	return psiphon.Install(ctx, s.NewProxiedHTTPClient(installTimeout))
}

// Uninstall stops the daemon, removes the binary and config, and clears the
// auto-start flag so a future boot doesn't relaunch something no longer there.
func (s *PsiphonService) Uninstall() error {
	if err := psiphon.Uninstall(); err != nil {
		return err
	}
	return s.SetPsiphonEnable(false)
}

// SaveConfig validates and stores the config, restarting the process if it
// was already running so the change actually takes effect.
func (s *PsiphonService) SaveConfig(raw []byte) error {
	if err := psiphon.SaveConfig(raw); err != nil {
		return err
	}
	if !psiphon.GetManager().IsRunning() {
		return nil
	}
	return psiphon.GetManager().Restart()
}

// AvailableRegions lists every ISO 3166-1 alpha-2 code, not a Psiphon-curated
// subset -- that would only ever be a stale snapshot; SetEgressRegion's live verification answers "does it work," not this list.
func AvailableRegions() []Region { return isoCountries }

// Region is one entry in the picker: an ISO 3166-1 alpha-2 code and its
// display name.
type Region struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// verifyTimeout bounds the live exit check -- more generous than a status
// poll needs, since a fresh Psiphon connection can take a while.
const verifyTimeout = 45 * time.Second

// SetEgressRegion patches EgressRegion, restarts the process, and reports
// what the new tunnel actually reaches -- not just what the config now says.
func (s *PsiphonService) SetEgressRegion(region string) (psiphon.ExitInfo, error) {
	if err := psiphon.SetEgressRegion(region); err != nil {
		return psiphon.ExitInfo{}, err
	}
	if err := psiphon.GetManager().Restart(); err != nil {
		return psiphon.ExitInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()
	return psiphon.CurrentExit(ctx)
}

// CurrentExit is the "Verify" button's action outside of a region change --
// e.g. confirming a long-running tunnel hasn't silently drifted.
func (s *PsiphonService) CurrentExit() (psiphon.ExitInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), verifyTimeout)
	defer cancel()
	return psiphon.CurrentExit(ctx)
}
