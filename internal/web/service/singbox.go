package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
	"github.com/mhsanaei/3x-ui/v3/internal/util"
	"github.com/mhsanaei/3x-ui/v3/model"
)

var singBoxProcess = singbox.NewProcess()

type SingBoxService struct {
	settingService SettingService
	xrayService    XrayService
}

func NewSingBoxService(settingService SettingService, xrayService XrayService) *SingBoxService {
	return &SingBoxService{settingService: settingService, xrayService: xrayService}
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

func (s *SingBoxService) ConnectionCount(ctx context.Context) (int, error) {
	api := singbox.NewConnectionAPIClient()
	defer api.Close()
	connections, err := api.Snapshot(ctx)
	if err == nil {
		return len(connections), nil
	}
	clashConnections, err := singbox.NewClashStatsClient().Connections(ctx)
	if err != nil {
		return 0, err
	}
	return len(clashConnections), nil
}

func (s *SingBoxService) OnlineClientIPs(ctx context.Context) (map[string]map[string]struct{}, error) {
	api := singbox.NewConnectionAPIClient()
	defer api.Close()
	connections, err := api.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	online := make(map[string]map[string]struct{})
	for _, connection := range connections {
		if connection == nil || connection.User == "" || connection.Source == "" {
			continue
		}
		if _, ok := online[connection.User]; !ok {
			online[connection.User] = make(map[string]struct{})
		}
		online[connection.User][connection.Source] = struct{}{}
	}
	return online, nil
}

func (s *SingBoxService) GetConnectionStats(ctx context.Context) (map[string]any, error) {
	api := singbox.NewConnectionAPIClient()
	defer api.Close()
	connections, err := api.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"connections": connections}, nil
}

func (s *SingBoxService) Close() error {
	return singBoxProcess.Close()
}

func (s *SingBoxService) Restart(ctx context.Context) error {
	return singBoxProcess.Restart(ctx)
}

func (s *SingBoxService) Start(ctx context.Context) error {
	return singBoxProcess.Start(ctx)
}

func (s *SingBoxService) Stop() error {
	return singBoxProcess.Stop()
}

func (s *SingBoxService) GetProcessInfo() map[string]any {
	return singBoxProcess.GetProcessInfo()
}

func (s *SingBoxService) GetConfig(ctx context.Context) (map[string]any, error) {
	return singBoxProcess.GetConfig(ctx)
}

func (s *SingBoxService) GenerateConfig(ctx context.Context) error {
	return singBoxProcess.GenerateConfig(ctx)
}

func (s *SingBoxService) GetLog(ctx context.Context) (string, error) {
	return singBoxProcess.GetLog(ctx)
}

func (s *SingBoxService) SetLogLevel(level string) error {
	return singBoxProcess.SetLogLevel(level)
}

func (s *SingBoxService) GetStatus(ctx context.Context) (map[string]any, error) {
	return singBoxProcess.GetStatus(ctx)
}

func (s *SingBoxService) TestConfig(ctx context.Context, config string) error {
	return singBoxProcess.TestConfig(ctx, config)
}

func (s *SingBoxService) GetMetrics(ctx context.Context) (map[string]any, error) {
	return singBoxProcess.GetMetrics(ctx)
}

func (s *SingBoxService) GetMemory(ctx context.Context) (map[string]any, error) {
	return singBoxProcess.GetMemory(ctx)
}

func (s *SingBoxService) GetConnections(ctx context.Context) ([]singbox.ClashConnection, error) {
	return singbox.NewClashStatsClient().Connections(ctx)
}

func (s *SingBoxService) GetConnection(ctx context.Context, id string) (singbox.ClashConnection, error) {
	return singbox.NewClashStatsClient().Connection(ctx, id)
}

func (s *SingBoxService) CloseConnection(ctx context.Context, id string) error {
	return singbox.NewClashStatsClient().CloseConnection(ctx, id)
}

func (s *SingBoxService) getXraySettings() (*model.XraySetting, error) {
	setting, err := s.settingService.GetXraySetting()
	if err != nil {
		return nil, err
	}
	if setting == nil {
		return nil, errors.New("xray setting is nil")
	}
	return setting, nil
}

func (s *SingBoxService) GenerateXrayConfig(ctx context.Context) (string, error) {
	setting, err := s.getXraySettings()
	if err != nil {
		return "", err
	}
	config := singbox.NewConfig()
	if setting.Log != "" {
		config.Log = map[string]any{"level": setting.Log}
	}
	if setting.Dns != "" {
		var dns map[string]any
		if err := util.UnmarshalJSON([]byte(setting.Dns), &dns); err != nil {
			return "", fmt.Errorf("parse DNS: %w", err)
		}
		translated, err := singbox.TranslateXrayDNS(dns)
		if err != nil {
			return "", err
		}
		config.DNS = translated
	}
	if setting.Routing != "" {
		var routing map[string]any
		if err := util.UnmarshalJSON([]byte(setting.Routing), &routing); err != nil {
			return "", fmt.Errorf("parse routing: %w", err)
		}
		translated, err := singbox.TranslateXrayRouting(routing)
		if err != nil {
			return "", err
		}
		config.Route = translated
	}
	if setting.Api != "" {
		var api map[string]any
		if err := util.UnmarshalJSON([]byte(setting.Api), &api); err != nil {
			return "", fmt.Errorf("parse API: %w", err)
		}
		config.Experimental = api
	}
	return config.Marshal()
}

func (s *SingBoxService) GenerateInboundConfig(inbound *model.XrayInbound) (map[string]any, error) {
	if inbound == nil {
		return nil, errors.New("inbound is nil")
	}
	return singbox.TranslateXrayInbound(inbound)
}

func (s *SingBoxService) GenerateOutboundConfig(outbound *model.XrayOutbound) (map[string]any, error) {
	if outbound == nil {
		return nil, errors.New("outbound is nil")
	}
	return singbox.TranslateXrayOutbound(outbound)
}

func (s *SingBoxService) GenerateConfigForInbound(ctx context.Context, inbound *model.XrayInbound) (string, error) {
	_ = ctx
	config := singbox.NewConfig()
	translated, err := s.GenerateInboundConfig(inbound)
	if err != nil {
		return "", err
	}
	config.Inbounds = []map[string]any{translated}
	return config.Marshal()
}

func (s *SingBoxService) GenerateConfigForOutbound(ctx context.Context, outbound *model.XrayOutbound) (string, error) {
	_ = ctx
	config := singbox.NewConfig()
	translated, err := s.GenerateOutboundConfig(outbound)
	if err != nil {
		return "", err
	}
	config.Outbounds = append(config.Outbounds, translated)
	return config.Marshal()
}

func (s *SingBoxService) UpdateConnectionStats(ctx context.Context) error {
	_, err := s.GetConnections(ctx)
	return err
}

func (s *SingBoxService) HealthCheck(ctx context.Context) error {
	if !s.IsRunning() {
		return errors.New("sing-box is not running")
	}
	_, err := s.Version(ctx)
	return err
}

func (s *SingBoxService) Reload(ctx context.Context) error {
	return s.Restart(ctx)
}

func (s *SingBoxService) ListenPort() int {
	return singBoxProcess.ListenPort()
}

func (s *SingBoxService) APIAddress() string {
	return singBoxProcess.APIAddress()
}

func (s *SingBoxService) SetAPIAddress(address string) error {
	return singBoxProcess.SetAPIAddress(address)
}

func (s *SingBoxService) SetListenPort(port int) error {
	return singBoxProcess.SetListenPort(port)
}

func (s *SingBoxService) SetBinaryPath(path string) error {
	return singBoxProcess.SetBinaryPath(path)
}

func (s *SingBoxService) BinaryPath() string {
	return singBoxProcess.BinaryPath()
}

func (s *SingBoxService) IsInstalled() bool {
	return singBoxProcess.IsInstalled()
}

func (s *SingBoxService) Install(ctx context.Context) error {
	return singBoxProcess.Install(ctx)
}

func (s *SingBoxService) Uninstall(ctx context.Context) error {
	return singBoxProcess.Uninstall(ctx)
}

func (s *SingBoxService) Upgrade(ctx context.Context) error {
	return singBoxProcess.Upgrade(ctx)
}

func (s *SingBoxService) HTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *SingBoxService) SetEnvironment(env map[string]string) error {
	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *SingBoxService) GetEnvironment(keys []string) map[string]string {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = os.Getenv(key)
	}
	return result
}

func (s *SingBoxService) ParsePort(value string) int {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return port
}

var _ = sync.Mutex{}
var _ = logger.GetLogger("singbox")
