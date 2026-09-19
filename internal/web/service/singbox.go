package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
)

type SingBoxService struct {
	inboundService InboundService
	settingService SettingService
}

var singBoxProcess = singbox.NewProcess(singbox.GetConfigPath())

func (s *SingBoxService) GetConfig() (*singbox.Config, error) {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	cfg := singbox.NewConfig()
	var unsupported []string
	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeID != nil {
			continue
		}
		rawBytes, err := json.Marshal(inbound)
		if err != nil {
			return nil, err
		}
		var raw map[string]any
		if err := json.Unmarshal(rawBytes, &raw); err != nil {
			return nil, err
		}
		translated, err := singbox.TranslateXrayInbound(raw)
		if err != nil {
			unsupported = append(unsupported, fmt.Sprintf("%s: %v", inbound.Tag, err))
			continue
		}
		cfg.Inbounds = append(cfg.Inbounds, translated)
	}

	if len(unsupported) > 0 {
		return nil, fmt.Errorf("sing-box cannot represent enabled inbounds: %s", strings.Join(unsupported, "; "))
	}
	return cfg, nil
}

func (s *SingBoxService) WriteConfig() error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	data, err := cfg.Marshal()
	if err != nil {
		return err
	}
	path := singbox.GetConfigPath()
	if err := os.MkdirAll(singBoxConfigDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func singBoxConfigDir() string {
	path := singbox.GetConfigPath()
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\' {
			return path[:i]
		}
	}
	return "."
}

func (s *SingBoxService) Restart(ctx context.Context) error {
	if err := s.WriteConfig(); err != nil {
		return err
	}
	return singBoxProcess.Restart(ctx)
}

func (s *SingBoxService) Start(ctx context.Context) error {
	if err := s.WriteConfig(); err != nil {
		return err
	}
	return singBoxProcess.Start(ctx)
}

func (s *SingBoxService) Stop(ctx context.Context) error {
	return singBoxProcess.Stop()
}

func (s *SingBoxService) IsRunning() bool {
	return singBoxProcess.IsRunning()
}

func (s *SingBoxService) Version(ctx context.Context) (string, error) {
	return singBoxProcess.Version(ctx)
}

func (s *SingBoxService) Validate(ctx context.Context) error {
	return singBoxProcess.Validate(ctx)
}

