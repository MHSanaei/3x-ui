package integration

import (
	"crypto/tls"
	"io"

	"github.com/mhsanaei/3x-ui/v3/internal/adguard"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/frontproxy"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// FrontProxyService manages the panel's own reverse proxy (internal/frontproxy)
// and remembers whether it should be running across restarts.
type FrontProxyService struct {
	service.SettingService
}

// DecoyDir is where an uploaded decoy site lives, following the same
// "sidecar owns a subdirectory of bin/" convention the Tor sidecar uses.
func DecoyDir() string { return config.GetBinFolderPath() + "/frontproxy-decoy" }

// certStorageDir keeps ACME material next to the rest of the panel's state
// so it is picked up by anything that backs that directory up.
func certStorageDir() string { return config.GetBinFolderPath() + "/frontproxy-certs" }

// FrontProxyStatus reports the reverse proxy's current state to the UI.
type FrontProxyStatus struct {
	Running       bool                  `json:"running"`
	Port          int                   `json:"port"`
	Templates     []string              `json:"templates"`
	DecoyUploaded bool                  `json:"decoyUploaded"`
	Cert          frontproxy.CertStatus `json:"cert"`
}

func (s *FrontProxyService) Status() FrontProxyStatus {
	port, _ := s.GetFrontProxyPort()
	return FrontProxyStatus{
		Running:       frontproxy.GetManager().IsRunning(),
		Port:          port,
		Templates:     frontproxy.DecoyTemplateNames(),
		DecoyUploaded: frontproxy.DecoyInstalled(DecoyDir()),
		Cert:          frontproxy.CurrentCertStatus(),
	}
}

// InstallDecoy unpacks an uploaded site archive. The door is restarted when
// it is already up so the new content is served without a panel restart.
func (s *FrontProxyService) InstallDecoy(r io.ReaderAt, size int64) error {
	if err := frontproxy.InstallDecoyArchive(DecoyDir(), r, size); err != nil {
		return err
	}
	return s.Reload()
}

// RemoveDecoy deletes the uploaded site, leaving the reverse proxy on whatever
// its other decoy modes provide.
func (s *FrontProxyService) RemoveDecoy() error {
	if err := frontproxy.RemoveDecoy(DecoyDir()); err != nil {
		return err
	}
	return s.Reload()
}

// Reload applies changed routing and decoy settings without interrupting the
// listener, so picking a different decoy takes effect on save rather than
// needing the admin to stop and start the proxy by hand.
//
// The listener itself keeps the port and certificate it was started with;
// changing those still needs a restart, and the tab says so.
func (s *FrontProxyService) Reload() error {
	if !frontproxy.GetManager().IsRunning() {
		return nil
	}
	opts, err := s.Options()
	if err != nil {
		return err
	}
	frontproxy.GetManager().Reload(opts)
	return nil
}

// Options assembles the running configuration from settings. The panel's own
// base path and the subscription server's path double as the secret paths.
func (s *FrontProxyService) Options() (frontproxy.Options, error) {
	port, err := s.GetFrontProxyPort()
	if err != nil {
		return frontproxy.Options{}, err
	}
	listen, err := s.GetFrontProxyListen()
	if err != nil {
		return frontproxy.Options{}, err
	}
	basePath, err := s.GetBasePath()
	if err != nil {
		return frontproxy.Options{}, err
	}
	panelPort, err := s.GetPort()
	if err != nil {
		return frontproxy.Options{}, err
	}
	subEnable, err := s.GetSubEnable()
	if err != nil {
		return frontproxy.Options{}, err
	}
	subPath, err := s.GetSubPath()
	if err != nil {
		return frontproxy.Options{}, err
	}
	subPort, err := s.GetSubPort()
	if err != nil {
		return frontproxy.Options{}, err
	}

	decoy, err := s.decoyConfig()
	if err != nil {
		return frontproxy.Options{}, err
	}
	tlsSettings, err := s.tlsSettings()
	if err != nil {
		return frontproxy.Options{}, err
	}

	return frontproxy.Options{
		Listen: listen,
		Port:   port,
		Routing: frontproxy.Config{
			PanelBasePath: basePath,
			PanelPort:     panelPort,
			SubPath:       subPath,
			SubPort:       subPort,
			SubEnabled:    subEnable,
			UpstreamTLS:   s.upstreamServesTLS(),
		},
		Decoy: decoy,
		TLS:   tlsSettings,
	}, nil
}

// upstreamServesTLS mirrors the condition the panel and subscription servers
// use to decide whether to wrap their own listeners in TLS: both files set and
// loadable. Get this wrong in the "yes" direction and the hop fails outright;
// wrong in the "no" direction and their HTTP-to-HTTPS redirector loops.
func (s *FrontProxyService) upstreamServesTLS() bool {
	certFile, err := s.GetCertFile()
	if err != nil || certFile == "" {
		return false
	}
	keyFile, err := s.GetKeyFile()
	if err != nil || keyFile == "" {
		return false
	}
	_, err = tls.LoadX509KeyPair(certFile, keyFile)
	return err == nil
}

func (s *FrontProxyService) decoyConfig() (frontproxy.DecoyConfig, error) {
	mode, err := s.GetFrontProxyDecoyMode()
	if err != nil {
		return frontproxy.DecoyConfig{}, err
	}
	template, err := s.GetFrontProxyDecoyTemplate()
	if err != nil {
		return frontproxy.DecoyConfig{}, err
	}
	proxyURL, err := s.GetFrontProxyDecoyProxyURL()
	if err != nil {
		return frontproxy.DecoyConfig{}, err
	}
	seed, err := s.GetFrontProxyDecoySeed()
	if err != nil {
		return frontproxy.DecoyConfig{}, err
	}
	// Read straight from AdGuard Home's own config rather than from settings:
	// it rewrites that file itself, so its config is the only thing that says
	// where it is actually listening. A zero here makes the decoy fall back to
	// a template instead of proxying into a closed port.
	adGuardPort, err := adguard.WebPort()
	if err != nil {
		adGuardPort = 0
	}
	return frontproxy.DecoyConfig{
		Mode:        frontproxy.DecoyMode(mode),
		Template:    template,
		Dir:         DecoyDir(),
		ProxyURL:    proxyURL,
		Seed:        seed,
		AdGuardPort: adGuardPort,
	}, nil
}

// tlsSettings resolves the certificate half. Manual mode reuses the panel's
// own cert files rather than introducing a second place to configure them.
func (s *FrontProxyService) tlsSettings() (frontproxy.TLSSettings, error) {
	mode, err := s.GetFrontProxyCertMode()
	if err != nil {
		return frontproxy.TLSSettings{}, err
	}
	domain, err := s.GetFrontProxyDomain()
	if err != nil {
		return frontproxy.TLSSettings{}, err
	}
	email, err := s.GetFrontProxyEmail()
	if err != nil {
		return frontproxy.TLSSettings{}, err
	}
	certFile, err := s.GetCertFile()
	if err != nil {
		return frontproxy.TLSSettings{}, err
	}
	keyFile, err := s.GetKeyFile()
	if err != nil {
		return frontproxy.TLSSettings{}, err
	}
	return frontproxy.TLSSettings{
		Mode:       frontproxy.CertMode(mode),
		Domain:     domain,
		Email:      email,
		CertFile:   certFile,
		KeyFile:    keyFile,
		StorageDir: certStorageDir(),
	}, nil
}

// Start brings the reverse proxy up and persists the choice so panel boot
// restores it automatically (see the auto-start in web.go).
func (s *FrontProxyService) Start() error {
	opts, err := s.Options()
	if err != nil {
		return err
	}
	if err := frontproxy.GetManager().Start(opts); err != nil {
		return err
	}
	return s.SetFrontProxyEnable(true)
}

// Stop shuts the reverse proxy down and persists the choice, mirroring Start.
func (s *FrontProxyService) Stop() error {
	if err := frontproxy.GetManager().Stop(); err != nil {
		return err
	}
	return s.SetFrontProxyEnable(false)
}

// AutoStart brings the door up at boot if the admin left it enabled. It
// never returns an error: the reverse proxy is a secondary feature and must
// not be able to stop the panel itself from starting.
//
// Logged at Info: the file log already captures Debug on every boot, but
// Info additionally reaches console/syslog/journal without a config change.
func (s *FrontProxyService) AutoStart() {
	logger.Info("frontproxy: AutoStart: checking whether enabled")
	enabled, err := s.GetFrontProxyEnable()
	if err != nil {
		logger.Warningf("frontproxy: AutoStart: cannot read enabled setting: %v", err)
		return
	}
	if !enabled {
		logger.Info("frontproxy: AutoStart: disabled, staying down")
		return
	}
	logger.Info("frontproxy: AutoStart: enabled, resolving config from settings")
	opts, err := s.Options()
	if err != nil {
		logger.Warningf("frontproxy: cannot build config, reverse proxy stays down: %v", err)
		return
	}
	logger.Info("frontproxy: AutoStart: config resolved, starting manager")
	if err := frontproxy.GetManager().Start(opts); err != nil {
		logger.Warningf("frontproxy: failed to auto-start on boot: %v", err)
		return
	}
	logger.Info("frontproxy: AutoStart: manager started")
}
