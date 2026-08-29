package integration

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/adguard"
	"github.com/mhsanaei/3x-ui/v3/internal/frontproxy"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// installTimeout bounds the whole download. Generous because this pulls tens
// of megabytes from GitHub, often through the panel's own egress proxy on a
// link where that is exactly the slow part.
const installTimeout = 5 * time.Minute

// AdGuardService manages the panel-managed AdGuard Home instance (see
// internal/adguard) that the reverse proxy can serve as its cover story.
type AdGuardService struct {
	service.SettingService
}

// AdGuardStatus reports the sidecar's current state to the UI.
type AdGuardStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	WebPort   int    `json:"webPort"`
	DNSPort   int    `json:"dnsPort"`
	User      string `json:"user"`
	Password  string `json:"password,omitempty"`
	IsDecoy   bool   `json:"isDecoy"`
	LastLog   string `json:"lastLog,omitempty"`
}

// Status reads the live ports out of AdGuard Home's own config when it is
// installed, since it owns that file once seeded, and falls back to the
// pending settings values when it is not.
func (s *AdGuardService) Status() AdGuardStatus {
	status := AdGuardStatus{
		Installed: adguard.IsInstalled(),
		Running:   adguard.GetManager().IsRunning(),
		User:      adguard.AdminUser,
		LastLog:   adguard.GetManager().LastResult(),
	}
	status.Password, _ = s.GetAdGuardPassword()
	if mode, err := s.GetFrontProxyDecoyMode(); err == nil {
		status.IsDecoy = frontproxy.DecoyMode(mode) == frontproxy.DecoyAdGuard
	}
	if webPort, err := adguard.WebPort(); err == nil {
		status.WebPort = webPort
	} else {
		status.WebPort, _ = s.GetAdGuardWebPort()
	}
	if dnsPort, err := adguard.DNSPort(); err == nil {
		status.DNSPort = dnsPort
	} else {
		status.DNSPort, _ = s.GetAdGuardDNSPort()
	}
	return status
}

// Install is the one-button path: fetch AdGuard Home, seed a config that skips
// its first-run wizard, start it, and point the reverse proxy's decoy at it.
//
// Switching the decoy mode is deliberate rather than a separate step -- an
// installed but unserved AdGuard Home would be a cover story nobody can see,
// which is not what the button offers. What it does not touch is the Xray
// inbound: pointing REALITY's fallback at the reverse proxy stays a conscious
// edit, since that is the change that can take the panel off the air.
func (s *AdGuardService) Install() error {
	webPort, err := s.GetAdGuardWebPort()
	if err != nil {
		return err
	}
	dnsPort, err := s.GetAdGuardDNSPort()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()
	if err := adguard.Install(ctx, s.NewProxiedHTTPClient(installTimeout)); err != nil {
		return err
	}

	password, err := adguard.Seed(adguard.SeedOptions{WebPort: webPort, DNSPort: dnsPort})
	if err != nil {
		return err
	}
	// An empty password means an existing config was kept, so the stored one
	// still matches it -- overwriting would lose the real credential.
	if password != "" {
		if err := s.SetAdGuardPassword(password); err != nil {
			return err
		}
	}

	if err := adguard.GetManager().Start(); err != nil {
		return err
	}
	if err := s.SetAdGuardEnable(true); err != nil {
		return err
	}
	if err := s.SetFrontProxyDecoyMode(string(frontproxy.DecoyAdGuard)); err != nil {
		return err
	}
	return (&FrontProxyService{}).Reload()
}

// Uninstall stops AdGuard Home, deletes it along with its config and filters,
// and returns the decoy to a template so the reverse proxy still shows a site
// rather than a bad gateway.
func (s *AdGuardService) Uninstall() error {
	if err := adguard.Uninstall(); err != nil {
		return err
	}
	if err := s.SetAdGuardEnable(false); err != nil {
		return err
	}
	if err := s.SetAdGuardPassword(""); err != nil {
		return err
	}
	mode, err := s.GetFrontProxyDecoyMode()
	if err != nil {
		return err
	}
	if frontproxy.DecoyMode(mode) == frontproxy.DecoyAdGuard {
		if err := s.SetFrontProxyDecoyMode(string(frontproxy.DecoyTemplate)); err != nil {
			return err
		}
	}
	return (&FrontProxyService{}).Reload()
}

// Start launches AdGuard Home and persists the choice so panel boot brings it
// back up automatically.
func (s *AdGuardService) Start() error {
	if err := adguard.GetManager().Start(); err != nil {
		return err
	}
	if err := s.SetAdGuardEnable(true); err != nil {
		return err
	}
	return (&FrontProxyService{}).Reload()
}

// Stop stops AdGuard Home and persists the choice, mirroring Start. The decoy
// mode is left alone: a stopped sidecar falls back to a template on its own,
// and the admin most likely means to start it again.
func (s *AdGuardService) Stop() error {
	if err := adguard.GetManager().Stop(); err != nil {
		return err
	}
	if err := s.SetAdGuardEnable(false); err != nil {
		return err
	}
	return (&FrontProxyService{}).Reload()
}

// AutoStart brings AdGuard Home up at boot if the admin left it enabled. It
// never returns an error: a cover story failing to start must not be able to
// stop the panel itself from starting.
func (s *AdGuardService) AutoStart() {
	enabled, err := s.GetAdGuardEnable()
	if err != nil || !enabled {
		return
	}
	if !adguard.IsInstalled() {
		logger.Warning("adguard: enabled but not installed, staying down")
		return
	}
	if err := adguard.GetManager().Start(); err != nil {
		logger.Warningf("adguard: failed to auto-start on boot: %v", err)
	}
}
