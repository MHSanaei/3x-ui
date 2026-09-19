package service

import (
	"context"
	"net"
	"sort"
	"sync"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

var (
	singBoxInboundService InboundService
	singBoxSettingService SettingService
	singBoxProcess = singbox.NewProcess(singbox.GetConfigPath())
)

// SetSingBoxDependencies wires the panel services used by the sing-box
// config generator. It mirrors the existing Xray service lifecycle wiring.
func SetSingBoxDependencies(inbound *InboundService, settings *SettingService) {
	if inbound != nil {
		singBoxInboundService = *inbound
	}
	if settings != nil {
		singBoxSettingService = *settings
	}
}

type SingBoxService struct{}

type singBoxConnectionTrafficState struct {
	Uplink   int64
	Downlink int64
}

var (
	singBoxTrafficMu    sync.Mutex
	singBoxTrafficState = make(map[string]singBoxConnectionTrafficState)
)

func (s *SingBoxService) GetConfig() (*singbox.Config, error) {
	inbounds, err := singBoxInboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	cfg := singbox.NewConfig()

	// Translate the panel's Xray outbound template as well. This keeps the
	// operator's egress/routing choices intact instead of silently forcing
	// every sing-box inbound to direct.
	if template, err := singBoxSettingService.GetXrayConfigTemplate(); err == nil {
		var xrayCfg map[string]any
		if json.Unmarshal([]byte(template), &xrayCfg) == nil {
			if rawDNS := rawObject(xrayCfg, "dns"); len(rawDNS) > 0 {
				if dns, err := singbox.TranslateXrayDNS(rawDNS); err == nil && len(dns) > 0 {
					cfg.DNS = dns
				}
			}
			if rawRouting := rawObject(xrayCfg, "routing"); len(rawRouting) > 0 {
				if route, err := singbox.TranslateXrayRouting(rawRouting); err != nil {
					return nil, err
				} else if len(route) > 0 {
					cfg.Route = route
				}
			}
			if rawOutbounds, ok := xrayCfg["outbounds"].([]any); ok {
				for _, raw := range rawOutbounds {
					ob, ok := raw.(map[string]any)
					if !ok { continue }
					translated, err := singbox.TranslateXrayOutbound(ob)
					if err != nil {
						return nil, err
					}
					cfg.Outbounds = append(cfg.Outbounds, translated)
				}
			}
		}
	}

	if len(cfg.Outbounds) == 0 {
		cfg.Outbounds = append(cfg.Outbounds,
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "block", "tag": "blocked"},
		)
	}

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
		dbClients, listErr := singBoxInboundService.clientService.ListForInbound(nil, inbound.Id)
		if listErr != nil {
			return nil, listErr
		}

		// Match XrayService's effective-client reconciliation: ClientStats is
		// the panel's source of truth for expiry/traffic enforcement, while the
		// client row itself controls the explicit enable flag.
		enableMap := make(map[string]bool, len(inbound.ClientStats))
		for _, stat := range inbound.ClientStats {
			enableMap[stat.Email] = stat.Enable
		}

		clients := make([]any, 0, len(dbClients))
		for _, client := range dbClients {
			if enabled, exists := enableMap[client.Email]; exists && !enabled {
				continue
			}
			if !client.Enable {
				continue
			}
			entry := map[string]any{"email": client.Email}
			switch inbound.Protocol {
			case model.VLESS:
				if client.ID != "" { entry["id"] = client.ID }
				if client.Flow != "" && !inbound.DisableFlow { entry["flow"] = client.Flow }
			case model.VMESS:
				if client.ID != "" { entry["id"] = client.ID }
				if client.Security != "" { entry["security"] = client.Security }
			case model.Trojan:
				if client.Password != "" { entry["password"] = client.Password }
				if client.Flow != "" && !inbound.DisableFlow { entry["flow"] = client.Flow }
			case model.Shadowsocks:
				if client.Password != "" { entry["password"] = client.Password }
			case model.Hysteria:
				if client.Auth != "" { entry["auth"] = client.Auth }
			case model.TUIC:
				if client.ID != "" { entry["uuid"] = client.ID }
				if client.Password != "" { entry["password"] = client.Password }
			}
			clients = append(clients, entry)
		}
		settings, _ := raw["settings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
		}
		settings["clients"] = clients
		raw["settings"] = settings

		translated, err := singbox.TranslateXrayInbound(raw)
		if err != nil {
			unsupported = append(unsupported, fmt.Sprintf("%s: %v", inbound.Tag, err))
			continue
		}
		cfg.Inbounds = append(cfg.Inbounds, translated)
	}
	stats["inbounds"] = statInbounds
	stats["users"] = statUsers

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
		if path[i] == '/' || path[i] == '\\' {
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



func (s *SingBoxService) ConnectionCount(ctx context.Context) (int, error) {
	api := singbox.NewConnectionAPIClient()
	defer api.Close()
	connections, err := api.Snapshot(ctx)
	if err == nil {
		return len(connections), nil
	}
	connections, err = singbox.NewClashStatsClient().Connections(ctx)
	if err != nil {
		return 0, err
	}
	return len(connections), nil
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
		ip := connection.Source
		if host, _, splitErr := net.SplitHostPort(ip); splitErr == nil {
			ip = host
		}
		if ip == "" {
			continue
		}
		if online[connection.User] == nil {
			online[connection.User] = make(map[string]struct{})
		}
		online[connection.User][ip] = struct{}{}
	}
	return online, nil
}

func (s *SingBoxService) PollTraffic(ctx context.Context) error {
	api := singbox.NewConnectionAPIClient()
	defer api.Close()
	connections, err := api.Snapshot(ctx)
	if err != nil {
		return err
	}

	inboundDeltas := make(map[string]*xray.Traffic)
	clientDeltas := make(map[string]*xray.ClientTraffic)
	onlineEmails := make(map[string]struct{})
	currentIDs := make(map[string]struct{}, len(connections))

	singBoxTrafficMu.Lock()
	for _, connection := range connections {
		if connection == nil || connection.ID == "" {
			continue
		}
		currentIDs[connection.ID] = struct{}{}
		previous, existed := singBoxTrafficState[connection.ID]
		uplinkDelta := connection.Uplink
		downlinkDelta := connection.Downlink
		if existed {
			uplinkDelta -= previous.Uplink
			downlinkDelta -= previous.Downlink
			if uplinkDelta < 0 {
				uplinkDelta = connection.Uplink
			}
			if downlinkDelta < 0 {
				downlinkDelta = connection.Downlink
			}
		}
		singBoxTrafficState[connection.ID] = singBoxConnectionTrafficState{
			Uplink: connection.Uplink,
			Downlink: connection.Downlink,
		}

		if connection.User != "" {
			onlineEmails[connection.User] = struct{}{}
		}
		if uplinkDelta == 0 && downlinkDelta == 0 {
			continue
		}
		if connection.Inbound != "" {
			traffic := inboundDeltas[connection.Inbound]
			if traffic == nil {
				traffic = &xray.Traffic{Tag: connection.Inbound, IsInbound: true}
				inboundDeltas[connection.Inbound] = traffic
			}
			traffic.Up += uplinkDelta
			traffic.Down += downlinkDelta
		}
		if connection.User != "" {
			traffic := clientDeltas[connection.User]
			if traffic == nil {
				traffic = &xray.ClientTraffic{Email: connection.User}
				clientDeltas[connection.User] = traffic
			}
			traffic.Up += uplinkDelta
			traffic.Down += downlinkDelta
		}
	}
	for id := range singBoxTrafficState {
		if _, active := currentIDs[id]; !active {
			delete(singBoxTrafficState, id)
		}
	}
	singBoxTrafficMu.Unlock()

	inboundTraffic := make([]*xray.Traffic, 0, len(inboundDeltas))
	for _, traffic := range inboundDeltas {
		inboundTraffic = append(inboundTraffic, traffic)
	}
	clientTraffic := make([]*xray.ClientTraffic, 0, len(clientDeltas))
	for _, traffic := range clientDeltas {
		clientTraffic = append(clientTraffic, traffic)
	}
	if len(inboundTraffic) > 0 || len(clientTraffic) > 0 {
		if _, _, err = singBoxInboundService.AddTraffic(inboundTraffic, clientTraffic); err != nil {
			return err
		}
	}

	emails := make([]string, 0, len(onlineEmails))
	for email := range onlineEmails {
		emails = append(emails, email)
	}
	sort.Strings(emails)
	if len(emails) > 0 {
		return singBoxInboundService.BumpClientsLastOnline(emails)
	}
	return nil
}

func (s *SingBoxService) InstallLatest(ctx context.Context) (string, error) {
	return singbox.InstallLatest(ctx)
}

func (s *SingBoxService) BinaryPath() string {
	return singbox.GetBinaryPath()
}


func (s *SingBoxService) ProcessConfigPath() string {
	return singbox.GetConfigPath()
}
