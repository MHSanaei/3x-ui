package discord

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// ServerProvider abstracts server status and database backup operations.
type ServerProvider interface {
	GetStatus(lastStatus *service.Status) *service.Status
	GetDb() ([]byte, error)
	BackupFilename(requestHost string) string
}

// InboundProvider abstracts inbound management operations.
type InboundProvider interface {
	GetAllInbounds() ([]*model.Inbound, error)
}

// BuildReport constructs the status report payload and backup attachments.
func (s *DiscordService) BuildReport(ctx context.Context, server ServerProvider, inbound InboundProvider) (MessagePayload, []FileAttachment, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "3x-ui"
	}

	var status *service.Status
	if server != nil {
		status = server.GetStatus(nil)
	}
	if status == nil {
		status = &service.Status{
			Loads: []float64{0, 0, 0},
		}
		status.Xray.State = service.ProcessState("unknown")
		status.Xray.Version = "unknown"
	}

	var onlines []string
	if process := service.XrayProcess(); process != nil && process.IsRunning() {
		onlines = process.GetOnlineClients()
	}

	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000

	trafficThreshold, err := s.settingService.GetTrafficDiff()
	if err == nil && trafficThreshold > 0 {
		trDiff = int64(trafficThreshold) * 1073741824
	}
	expireThreshold, err := s.settingService.GetExpireDiff()
	if err == nil && expireThreshold > 0 {
		exDiff = int64(expireThreshold) * 86400000
	}

	var totalInbounds, disabledInbounds, exhaustedInbounds int
	var totalClients, disabledClients, exhaustedClients int
	seenClients := make(map[string]bool)

	if inbound != nil {
		inbounds, err := inbound.GetAllInbounds()
		if err != nil {
			logger.Warning("Discord report: unable to load inbounds: ", err)
		} else {
			totalInbounds = len(inbounds)
			for _, in := range inbounds {
				if !in.Enable {
					disabledInbounds++
				} else if (in.ExpiryTime > 0 && (in.ExpiryTime-now < exDiff)) ||
					(in.Total > 0 && (in.Total-(in.Up+in.Down) < trDiff)) {
					exhaustedInbounds++
				}

				for _, client := range in.ClientStats {
					if seenClients[client.Email] {
						continue
					}
					seenClients[client.Email] = true
					totalClients++
					if !client.Enable {
						disabledClients++
					} else if (client.ExpiryTime > 0 && (client.ExpiryTime-now < exDiff)) ||
						(client.Total > 0 && (client.Total-(client.Up+client.Down) < trDiff)) {
						exhaustedClients++
					}
				}
			}
		}
	}

	days := status.Uptime / 86400
	hours := (status.Uptime % 86400) / 3600
	uptimeStr := fmt.Sprintf("%dd %dh", days, hours)

	ramStr := fmt.Sprintf("%s / %s", common.FormatTraffic(int64(status.Mem.Current)), common.FormatTraffic(int64(status.Mem.Total)))
	trafficStr := fmt.Sprintf("↑%s  ↓%s (Total: %s)",
		common.FormatTraffic(int64(status.NetTraffic.Sent)),
		common.FormatTraffic(int64(status.NetTraffic.Recv)),
		common.FormatTraffic(int64(status.NetTraffic.Sent+status.NetTraffic.Recv)),
	)
	load1, load2, load3 := 0.0, 0.0, 0.0
	if len(status.Loads) > 0 {
		load1 = status.Loads[0]
	}
	if len(status.Loads) > 1 {
		load2 = status.Loads[1]
	}
	if len(status.Loads) > 2 {
		load3 = status.Loads[2]
	}
	loadStr := fmt.Sprintf("%.2f, %.2f, %.2f", load1, load2, load3)

	fields := []EmbedField{
		{Name: "Host", Value: hostname, Inline: true},
		{Name: "Panel Version", Value: config.GetPanelVersion(), Inline: true},
		{Name: "Xray Core", Value: fmt.Sprintf("%s (%s)", status.Xray.Version, status.Xray.State), Inline: true},
		{Name: "Uptime", Value: uptimeStr, Inline: true},
		{Name: "System Load", Value: loadStr, Inline: true},
		{Name: "Memory (RAM)", Value: ramStr, Inline: true},
		{Name: "Network Traffic", Value: trafficStr, Inline: false},
		{Name: "Connections", Value: fmt.Sprintf("TCP: %d | UDP: %d", status.TcpCount, status.UdpCount), Inline: true},
		{Name: "Online Clients", Value: strconv.Itoa(len(onlines)), Inline: true},
		{Name: "Inbounds", Value: fmt.Sprintf("Total: %d | Depleting: %d | Disabled: %d", totalInbounds, exhaustedInbounds, disabledInbounds), Inline: false},
		{Name: "Clients", Value: fmt.Sprintf("Total: %d | Depleting: %d | Disabled: %d", totalClients, exhaustedClients, disabledClients), Inline: false},
	}

	ipv4, ipv6 := getInterfaceIPs()
	if ipv4 != "" {
		fields = append(fields, EmbedField{Name: "IPv4", Value: ipv4, Inline: true})
	}
	if ipv6 != "" {
		fields = append(fields, EmbedField{Name: "IPv6", Value: ipv6, Inline: true})
	}

	runTime, _ := s.settingService.GetDiscordRunTime()
	if runTime == "" {
		runTime = "@daily"
	}

	embed := Embed{
		Title:       "📊 3x-ui Status Report",
		Description: fmt.Sprintf("Periodic server and proxy status report for **%s**", hostname),
		Color:       ColorBlue,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      fields,
		Footer: &EmbedFooter{
			Text: fmt.Sprintf("3x-ui Scheduled Report • Schedule: %s", runTime),
		},
	}

	payload := MessagePayload{
		Embeds: []Embed{embed},
	}

	var files []FileAttachment
	backupEnabled, err := s.settingService.GetDiscordBotBackup()
	if err == nil && backupEnabled && server != nil {
		dbData, err := server.GetDb()
		if err != nil {
			logger.Warning("Discord report: failed to get DB backup: ", err)
		} else if len(dbData) > 0 {
			filename := server.BackupFilename("")
			if filename == "" {
				filename = "x-ui.db"
			}
			files = append(files, FileAttachment{
				Filename: filename,
				Data:     dbData,
			})
		}

		configPath := xray.GetConfigPath()
		if configData, err := os.ReadFile(configPath); err == nil && len(configData) > 0 {
			files = append(files, FileAttachment{
				Filename: "config.json",
				Data:     configData,
			})
		}
	}

	return payload, files, nil
}

// SendReport generates and sends the periodic report to Discord.
func (s *DiscordService) SendReport(ctx context.Context, server ServerProvider, inbound InboundProvider) error {
	payload, files, err := s.BuildReport(ctx, server, inbound)
	if err != nil {
		return fmt.Errorf("build discord report: %w", err)
	}
	return s.SendMessageWithFiles(ctx, payload, files...)
}

func getInterfaceIPs() (ipv4, ipv6 string) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return "", ""
	}
	var v4s, v6s []string
	for _, iface := range netInterfaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if ip := ipnet.IP.To4(); ip != nil {
				v4s = append(v4s, ip.String())
			} else if ip := ipnet.IP.To16(); ip != nil && !ipnet.IP.IsLinkLocalUnicast() {
				v6s = append(v6s, ip.String())
			}
		}
	}
	return strings.Join(v4s, ", "), strings.Join(v6s, ", ")
}
