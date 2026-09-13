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

	tr := translator(s.settingService)
	days := status.Uptime / 86400
	hours := (status.Uptime % 86400) / 3600
	uptimeStr := tr("discord.values.uptime", "Days=="+fmt.Sprint(days), "Hours=="+fmt.Sprint(hours))

	ramStr := fmt.Sprintf("%s / %s", common.FormatTraffic(int64(status.Mem.Current)), common.FormatTraffic(int64(status.Mem.Total)))
	trafficStr := tr("discord.values.traffic",
		"Up=="+common.FormatTraffic(int64(status.NetTraffic.Sent)),
		"Down=="+common.FormatTraffic(int64(status.NetTraffic.Recv)),
		"Total=="+common.FormatTraffic(int64(status.NetTraffic.Sent+status.NetTraffic.Recv)),
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
		{Name: tr("host"), Value: hostname, Inline: true},
		{Name: tr("discord.fields.panelVersion"), Value: config.GetPanelVersion(), Inline: true},
		{Name: tr("discord.fields.xrayCore"), Value: fmt.Sprintf("%s (%s)", status.Xray.Version, status.Xray.State), Inline: true},
		{Name: tr("pages.index.uptime"), Value: uptimeStr, Inline: true},
		{Name: tr("discord.fields.systemLoad"), Value: loadStr, Inline: true},
		{Name: tr("pages.index.memory"), Value: ramStr, Inline: true},
		{Name: tr("discord.fields.networkTraffic"), Value: trafficStr, Inline: false},
		{Name: tr("pages.index.historyTabConnections"), Value: fmt.Sprintf("TCP: %d | UDP: %d", status.TcpCount, status.UdpCount), Inline: true},
		{Name: tr("pages.index.historyTitleOnline"), Value: strconv.Itoa(len(onlines)), Inline: true},
		{Name: tr("tgbot.inbounds"), Value: tr("discord.values.counts", "Total=="+strconv.Itoa(totalInbounds), "Depleting=="+strconv.Itoa(exhaustedInbounds), "Disabled=="+strconv.Itoa(disabledInbounds)), Inline: false},
		{Name: tr("clients"), Value: tr("discord.values.counts", "Total=="+strconv.Itoa(totalClients), "Depleting=="+strconv.Itoa(exhaustedClients), "Disabled=="+strconv.Itoa(disabledClients)), Inline: false},
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
		Title:       tr("discord.report.title"),
		Description: tr("discord.report.summary", "Host=="+hostname),
		Color:       ColorBlue,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Fields:      fields,
		Footer: &EmbedFooter{
			Text: tr("discord.report.footer", "RunTime=="+runTime),
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
	// Separate messages: a backup over Discord's upload cap must not drop the report with it.
	if err := s.SendMessage(ctx, payload); err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	return s.SendMessageWithFiles(ctx, MessagePayload{}, files...)
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
