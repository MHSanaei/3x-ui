package service

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/xray"

	"go.uber.org/atomic"
)

var (
	p                 *xray.Process
	lock              sync.Mutex
	isNeedXrayRestart atomic.Bool // Indicates that restart was requested for Xray
	isManuallyStopped atomic.Bool // Indicates that Xray was stopped manually from the panel
	result            string
)

// XrayService provides business logic for Xray process management.
// It handles starting, stopping, restarting Xray, and managing its configuration.
type XrayService struct {
	inboundService InboundService
	settingService SettingService
	xrayAPI        xray.XrayAPI
}

// IsXrayRunning checks if the Xray process is currently running.
func (s *XrayService) IsXrayRunning() bool {
	return p != nil && p.IsRunning()
}

// GetXrayErr returns the error from the Xray process, if any.
func (s *XrayService) GetXrayErr() error {
	if p == nil {
		return nil
	}

	err := p.GetErr()
	if err == nil {
		return nil
	}

	if runtime.GOOS == "windows" && err.Error() == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return nil
	}

	return err
}

// GetXrayResult returns the result string from the Xray process.
func (s *XrayService) GetXrayResult() string {
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	if p == nil {
		return ""
	}

	result = p.GetResult()

	if runtime.GOOS == "windows" && result == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return ""
	}

	return result
}

// GetXrayVersion returns the version of the running Xray process.
func (s *XrayService) GetXrayVersion() string {
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

// RemoveIndex removes an element at the specified index from a slice.
// Returns a new slice with the element removed.
func RemoveIndex(s []any, index int) []any {
	return append(s[:index], s[index+1:]...)
}

// GetXrayConfig retrieves and builds the Xray configuration from settings and inbounds.
func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}
	xrayConfig.LogConfig = resolveXrayLogPaths(xrayConfig.LogConfig)

	_, _, _ = s.inboundService.AddTraffic(nil, nil)

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		if inbound.NodeID != nil {
			continue
		}
		settings := map[string]any{}
		json.Unmarshal([]byte(inbound.Settings), &settings)

		dbClients, listErr := s.inboundService.clientService.ListForInbound(nil, inbound.Id)
		if listErr != nil {
			return nil, listErr
		}

		clientStats := inbound.ClientStats
		enableMap := make(map[string]bool, len(clientStats))
		for _, clientTraffic := range clientStats {
			enableMap[clientTraffic.Email] = clientTraffic.Enable
		}

		var finalClients []any
		for i := range dbClients {
			c := dbClients[i]
			if enable, exists := enableMap[c.Email]; exists && !enable {
				logger.Infof("Remove Inbound User %s due to expiration or traffic limit", c.Email)
				continue
			}
			if !c.Enable {
				continue
			}
			flow := c.Flow
			if flow == "xtls-rprx-vision-udp443" {
				flow = "xtls-rprx-vision"
			}
			entry := map[string]any{"email": c.Email}
			switch inbound.Protocol {
			case model.VLESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if flow != "" {
					entry["flow"] = flow
				}
				if c.Reverse != nil {
					entry["reverse"] = c.Reverse
				}
			case model.VMESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if c.Security != "" {
					entry["security"] = c.Security
				}
			case model.Trojan:
				if c.Password != "" {
					entry["password"] = c.Password
				}
				if flow != "" {
					entry["flow"] = flow
				}
			case model.Shadowsocks:
				if c.Password != "" {
					entry["password"] = c.Password
				}
			case model.Hysteria:
				if c.Auth != "" {
					entry["auth"] = c.Auth
				}
			}
			finalClients = append(finalClients, entry)
		}

		_, hadClients := settings["clients"]
		mutated := hadClients || len(finalClients) > 0
		if mutated {
			settings["clients"] = finalClients
		}

		if inboundCanHostFallbacks(inbound) {
			fallbacks, fbErr := s.inboundService.fallbackService.BuildFallbacksJSON(nil, inbound.Id)
			if fbErr != nil {
				return nil, fbErr
			}
			if len(fallbacks) > 0 {
				generic := make([]any, 0, len(fallbacks))
				for _, f := range fallbacks {
					generic = append(generic, f)
				}
				settings["fallbacks"] = generic
				mutated = true
			}
		}

		if mutated {
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.Settings = string(modifiedSettings)
		}

		if len(inbound.StreamSettings) > 0 {
			// Unmarshal stream JSON
			var stream map[string]any
			json.Unmarshal([]byte(inbound.StreamSettings), &stream)

			// Remove the "settings" field under "tlsSettings" and "realitySettings"
			tlsSettings, ok1 := stream["tlsSettings"].(map[string]any)
			realitySettings, ok2 := stream["realitySettings"].(map[string]any)
			if ok1 || ok2 {
				if ok1 {
					delete(tlsSettings, "settings")
				} else if ok2 {
					delete(realitySettings, "settings")
				}
			}

			delete(stream, "externalProxy")

			newStream, err := json.MarshalIndent(stream, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.StreamSettings = string(newStream)
		}

		if inbound.Protocol == model.Shadowsocks {
			if healed, ok := model.HealShadowsocksClientMethods(inbound.Settings); ok {
				inbound.Settings = healed
			}
		}

		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

// resolveXrayLogPaths rewrites relative `log.access` / `log.error` values to
// absolute paths under config.GetLogFolder(), so Xray writes those files
// alongside the panel's other logs regardless of the working directory the
// panel was launched from. Values that are empty, "none", or already absolute
// are left untouched, as are unparseable log blocks.
func resolveXrayLogPaths(logCfg json_util.RawMessage) json_util.RawMessage {
	if len(logCfg) == 0 {
		return logCfg
	}
	var parsed map[string]any
	if err := json.Unmarshal(logCfg, &parsed); err != nil {
		return logCfg
	}
	changed := false
	for _, key := range []string{"access", "error"} {
		v, ok := parsed[key].(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || strings.EqualFold(trimmed, "none") {
			continue
		}
		if filepath.IsAbs(trimmed) {
			continue
		}
		cleaned := filepath.ToSlash(filepath.Clean(trimmed))
		base := filepath.Base(cleaned)
		if base == "" || base == "." || base == string(filepath.Separator) {
			continue
		}
		// Only rewrite bare names ("./access.log", "access.log").
		// A nested relative path like "./logs/foo.log" is treated as
		// a deliberate user choice and left alone.
		if cleaned != base {
			continue
		}
		parsed[key] = filepath.Join(config.GetLogFolder(), base)
		changed = true
	}
	if !changed {
		return logCfg
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		return logCfg
	}
	return out
}


// GetXrayTraffic fetches the current traffic statistics from the running Xray process.
func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	if !s.IsXrayRunning() {
		err := errors.New("xray is not running")
		logger.Debug("Attempted to fetch Xray traffic, but Xray is not running:", err)
		return nil, nil, err
	}
	apiPort := p.GetAPIPort()
	if err := s.xrayAPI.Init(apiPort); err != nil {
		logger.Debug("Failed to initialize Xray API:", err)
		return nil, nil, err
	}
	defer s.xrayAPI.Close()

	traffic, clientTraffic, err := s.xrayAPI.GetTraffic()
	if err != nil {
		logger.Debug("Failed to fetch Xray traffic:", err)
		return nil, nil, err
	}
	return traffic, clientTraffic, nil
}

// RestartXray restarts the Xray process, optionally forcing a restart even if config unchanged.
func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart Xray, force:", isForce)
	isManuallyStopped.Store(false)

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	if s.IsXrayRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) && !isNeedXrayRestart.Load() {
			logger.Debug("It does not need to restart Xray")
			return nil
		}
		p.Stop()
	}

	p = xray.NewProcess(xrayConfig)
	result = ""
	s.xrayAPI.StatsLastValues = nil
	err = p.Start()
	if err != nil {
		return err
	}

	return nil
}

// StopXray stops the running Xray process.
func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	isManuallyStopped.Store(true)
	logger.Debug("Attempting to stop Xray...")
	if s.IsXrayRunning() {
		return p.Stop()
	}
	return errors.New("xray is not running")
}

// SetToNeedRestart marks that Xray needs to be restarted.
func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

// GetXrayAPIPort returns the port the local xray process is listening on
// for its gRPC HandlerService, or 0 when xray isn't currently running.
// Exposed for the runtime package's LocalRuntime adapter — runtime can't
// reach into the package-level `p` directly without a service-package
// import cycle.
func (s *XrayService) GetXrayAPIPort() int {
	if p == nil || !p.IsRunning() {
		return 0
	}
	return p.GetAPIPort()
}

// IsNeedRestartAndSetFalse checks if restart is needed and resets the flag to false.
func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CompareAndSwap(true, false)
}

// DidXrayCrash checks if Xray crashed by verifying it's not running and wasn't manually stopped.
func (s *XrayService) DidXrayCrash() bool {
	return !s.IsXrayRunning() && !isManuallyStopped.Load()
}
