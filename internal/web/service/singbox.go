package service

import (
	"sort"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/singbox"
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

	v2rayAPI, _ := cfg.Experimental["v2ray_api"].(map[string]any)
	stats, _ := v2rayAPI["stats"].(map[string]any)
	var statInbounds []string
	var statUsers []string
	for _, outbound := range cfg.Outbounds {
		if tag, ok := outbound["tag"].(string); ok && tag != "" {
			if list, ok := stats["outbounds"].([]string); ok {
				stats["outbounds"] = append(list, tag)
			}
		}
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
		if tag, ok := translated["tag"].(string); ok && tag != "" {
			statInbounds = append(statInbounds, tag)
		}
		if users, ok := translated["users"].([]map[string]any); ok {
			for _, user := range users {
				if name, ok := user["name"].(string); ok && name != "" {
					statUsers = append(statUsers, name)
				}
			}
		}
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
	connections, err := singbox.NewClashStatsClient().Connections(ctx)
	if err != nil {
		return 0, err
	}
	return len(connections), nil
}

func (s *SingBoxService) OnlineClientEmails(ctx context.Context) ([]string, error) {
	connections, err := singbox.NewClashStatsClient().Connections(ctx)
	if err != nil {
		return nil, err
	}

	// Clash API exposes source IP and inbound tag, but not the authenticated
	// sing-box username/email. Attribute an IP to a panel client only when the
	// panel's existing IP history maps that IP to exactly one email. This avoids
	// inventing an identity when several users share the same address.
	ips, err := singBoxInboundService.GetAllInboundClientIps()
	if err != nil {
		return nil, err
	}

	type ipEntry struct {
		IP string `json:"ip"`
	}
	owners := make(map[string]map[string]struct{})
	for _, row := range ips {
		if row.ClientEmail == "" || row.Ips == "" {
			continue
		}
		var entries []ipEntry
		if err := json.Unmarshal([]byte(row.Ips), &entries); err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IP == "" {
				continue
			}
			if owners[entry.IP] == nil {
				owners[entry.IP] = make(map[string]struct{})
			}
			owners[entry.IP][row.ClientEmail] = struct{}{}
		}
	}

	online := make(map[string]struct{})
	for _, connection := range connections {
		ip := connection.Metadata.SourceIP
		if ip == "" {
			continue
		}
		if emails := owners[ip]; len(emails) == 1 {
			for email := range emails {
				online[email] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(online))
	for email := range online {
		result = append(result, email)
	}
	sort.Strings(result)
	return result, nil
}

func (s *SingBoxService) PollTraffic(ctx context.Context) error {
	inbounds, err := singBoxInboundService.GetAllInbounds()
	if err != nil {
		return err
	}
	statsClient := &singbox.V2RayStatsClient{}
	inboundTraffic := make([]*xray.Traffic, 0, len(inbounds))
	seenEmails := make(map[string]struct{})
	clientEmails := make([]string, 0)

	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.NodeId != nil {
			continue
		}
		up, down, queryErr := statsClient.QueryInbound(ctx, inbound.Tag, true)
		if queryErr != nil {
			continue
		}
		if up != 0 || down != 0 {
			inboundTraffic = append(inboundTraffic, &xray.Traffic{
				Tag: inbound.Tag, Up: up, Down: down, IsInbound: true,
			})
		}
		clients, listErr := singBoxInboundService.clientService.ListForInbound(nil, inbound.Id)
		if listErr != nil {
			continue
		}
		for _, client := range clients {
			if client == nil || !client.Enable || client.Email == "" {
				continue
			}
			if _, exists := seenEmails[client.Email]; !exists {
				seenEmails[client.Email] = struct{}{}
				clientEmails = append(clientEmails, client.Email)
			}
		}
	}

	clientTraffic := make([]*xray.ClientTraffic, 0, len(clientEmails))
	for _, email := range clientEmails {
		up, down, queryErr := statsClient.QueryUser(ctx, email, true)
		if queryErr != nil {
			continue
		}
		if up == 0 && down == 0 {
			continue
		}
		clientTraffic = append(clientTraffic, &xray.ClientTraffic{
			Email: email, Up: up, Down: down,
		})
	}

	if len(inboundTraffic) > 0 || len(clientTraffic) > 0 {
		if _, _, err = singBoxInboundService.AddTraffic(inboundTraffic, clientTraffic); err != nil {
			return err
		}
	}

	// Refresh last_online independently of byte deltas so an idle but open
	// sing-box connection remains online in the panel.
	emails, err := s.OnlineClientEmails(ctx)
	if err != nil {
		return err
	}
	return singBoxInboundService.BumpClientsLastOnline(emails)
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
