package tgbot

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// SendReport sends a periodic report to admin chats.
func (t *Tgbot) SendReport() {
	runTime, err := t.settingService.GetTgbotRuntime()
	if err == nil && len(runTime) > 0 {
		msg := ""
		msg += t.I18nBot("tgbot.messages.report", "RunTime=="+runTime)
		msg += t.I18nBot("tgbot.messages.datetime", "DateTime=="+time.Now().Format("2006-01-02 15:04:05"))
		t.SendMsgToTgbotAdmins(msg)
	}

	t.SendMsgToTgbotAdmins(t.prepareServerUsageInfo())

	t.sendExhaustedToAdmins()
	t.notifyExhausted()

	backupEnable, err := t.settingService.GetTgBotBackup()
	if err == nil && backupEnable {
		t.SendBackupToAdmins()
	}
}

// backupSummary is the caption that travels with a scheduled backup.
func (t *Tgbot) backupSummary() string {
	summary := t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	summary += t.I18nBot("tgbot.messages.backupTime", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	return trimCaption(summary+"\n\n"+t.depleteReport(), botCaptionLimit)
}

// SendBackupToAdmins sends the database, and the generated Xray config when it
// exists, to admin chats. A failed DB read is reported instead of sending
// nothing: a silent empty backup looks like the bot is broken.
func (t *Tgbot) SendBackupToAdmins() {
	if !t.IsRunning() {
		return
	}
	dbData, err := t.serverService.GetDb()
	if err != nil {
		logger.Error("Error in getting db backup: ", err)
	}
	dbFilename := t.serverService.BackupFilename("")
	config, configErr := os.ReadFile(xray.GetConfigPath())
	if configErr != nil {
		logger.Warning("Cannot read the Xray config for the backup: ", configErr)
	}
	admins := adminSnapshot()
	for i, adminId := range admins {
		if dbData == nil {
			t.sendNotice(adminId, t.I18nBot("tgbot.messages.backupFailed"))
		} else {
			t.sendDocumentWithCaption(adminId, dbData, dbFilename, t.backupSummary())
			time.Sleep(500 * time.Millisecond)
			if configErr == nil {
				t.sendDocumentWithCaption(adminId, config, "config.json", "")
			}
		}
		// Add delay between sends to avoid Telegram rate limits
		if i < len(admins)-1 {
			time.Sleep(1 * time.Second)
		}
	}
}

// sendExhaustedToAdmins sends the depletion summary to each admin as a screen.
func (t *Tgbot) sendExhaustedToAdmins() {
	if !t.IsRunning() {
		return
	}
	for _, adminId := range adminSnapshot() {
		// A scheduled report must never edit the screen the admin is working on:
		// a cron landing on the add-client wizard would replace the draft. This
		// posts a free-standing message instead.
		t.sendNotice(adminId, t.depleteReport())
	}
}

// prepareServerUsageInfo prepares the server usage information string.
func (t *Tgbot) prepareServerUsageInfo() string {
	// Check if we have cached data first
	if cachedStats, found := t.getCachedServerStats(); found {
		return cachedStats
	}

	info, ipv4, ipv6 := "", "", ""

	// get latest status of server with caching
	if cachedStatus, found := t.getCachedStatus(); found {
		t.lastStatus = cachedStatus
	} else {
		t.lastStatus = t.serverService.GetStatus(t.lastStatus)
		t.setCachedStatus(t.lastStatus)
	}
	var onlines []string
	if process := service.XrayProcess(); process != nil {
		onlines = process.GetOnlineClients()
	}

	info += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	info += t.I18nBot("tgbot.messages.version", "Version=="+config.GetPanelVersion())
	info += t.I18nBot("tgbot.messages.xrayVersion", "XrayVersion=="+fmt.Sprint(t.lastStatus.Xray.Version))

	// get ip address
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("net.Interfaces failed, err: ", err.Error())
		info += t.I18nBot("tgbot.messages.ip", "IP=="+t.I18nBot("tgbot.unknown"))
		info += "\r\n"
	} else {
		for i := range netInterfaces {
			if (netInterfaces[i].Flags & net.FlagUp) != 0 {
				addrs, _ := netInterfaces[i].Addrs()

				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ipv4 += ipnet.IP.String() + " "
						} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLinkLocalUnicast() {
							ipv6 += ipnet.IP.String() + " "
						}
					}
				}
			}
		}

		info += t.I18nBot("tgbot.messages.ipv4", "IPv4=="+ipv4)
		info += t.I18nBot("tgbot.messages.ipv6", "IPv6=="+ipv6)
	}

	info += t.I18nBot("tgbot.messages.serverUpTime", "UpTime=="+strconv.FormatUint(t.lastStatus.Uptime/86400, 10), "Unit=="+t.I18nBot("tgbot.days"))
	info += t.I18nBot("tgbot.messages.serverLoad", "Load1=="+strconv.FormatFloat(t.lastStatus.Loads[0], 'f', 2, 64), "Load2=="+strconv.FormatFloat(t.lastStatus.Loads[1], 'f', 2, 64), "Load3=="+strconv.FormatFloat(t.lastStatus.Loads[2], 'f', 2, 64))
	info += t.I18nBot("tgbot.messages.serverMemory", "Current=="+common.FormatTraffic(int64(t.lastStatus.Mem.Current)), "Total=="+common.FormatTraffic(int64(t.lastStatus.Mem.Total)))
	info += t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(len(onlines)))
	info += t.I18nBot("tgbot.messages.tcpCount", "Count=="+strconv.Itoa(t.lastStatus.TcpCount))
	info += t.I18nBot("tgbot.messages.udpCount", "Count=="+strconv.Itoa(t.lastStatus.UdpCount))
	info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), "Upload=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), "Download=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv)))
	info += t.I18nBot("tgbot.messages.xrayStatus", "State=="+fmt.Sprint(t.lastStatus.Xray.State))

	// Cache the complete server stats
	t.setCachedServerStats(info)

	return info
}

// UserLoginNotify publishes a login event to the event bus.
func (t *Tgbot) UserLoginNotify(attempt LoginAttempt) {
	if attempt.Username == "" || attempt.IP == "" || attempt.Time == "" {
		logger.Warning("UserLoginNotify failed, invalid info!")
		return
	}

	if EventBus == nil {
		return
	}

	status := "fail"
	if attempt.Status == LoginSuccess {
		status = "success"
	}

	EventBus.Publish(eventbus.Event{
		Type:   eventbus.EventLoginAttempt,
		Source: attempt.IP,
		Data: &eventbus.LoginEventData{
			Username: attempt.Username,
			IP:       attempt.IP,
			Time:     attempt.Time,
			Status:   status,
			Reason:   attempt.Reason,
		},
	})
}

// notifyExhausted sends notifications for exhausted clients.
func (t *Tgbot) notifyExhausted() {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000

	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		exDiff = int64(ExpireThreshold) * 86400000
	}
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load Inbounds", err)
	}

	var chatIDsDone []int64
	for _, inbound := range inbounds {
		if inbound.Enable {
			if len(inbound.ClientStats) > 0 {
				clients, err := t.inboundService.GetClients(inbound)
				if err == nil {
					for _, client := range clients {
						if client.TgID != 0 {
							chatID := client.TgID
							if !int64Contains(chatIDsDone, chatID) && !checkAdmin(chatID) {
								var disabledClients []xray.ClientTraffic
								var exhaustedClients []xray.ClientTraffic
								traffics, err := t.inboundService.GetClientTrafficTgBot(client.TgID)
								if err == nil && len(traffics) > 0 {
									var output strings.Builder
									output.WriteString(t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.clients")))
									for _, traffic := range traffics {
										if traffic.Enable {
											if (traffic.ExpiryTime > 0 && (traffic.ExpiryTime-now < exDiff)) ||
												(traffic.Total > 0 && (traffic.Total-(traffic.Up+traffic.Down) < trDiff)) {
												exhaustedClients = append(exhaustedClients, *traffic)
											}
										} else {
											disabledClients = append(disabledClients, *traffic)
										}
									}
									if len(exhaustedClients) > 0 {
										output.WriteString(t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledClients))))
										if len(disabledClients) > 0 {
											output.WriteString(t.I18nBot("tgbot.clients"))
											output.WriteString(":\r\n")
											for _, traffic := range disabledClients {
												output.WriteString(" ")
												output.WriteString(traffic.Email)
											}
											output.WriteString("\r\n")
										}
										output.WriteString("\r\n")
										output.WriteString(t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(len(exhaustedClients))))
										for _, traffic := range exhaustedClients {
											output.WriteString(t.clientInfoMsg(&traffic, true, false, false, true, true, false))
											output.WriteString("\r\n")
										}
										t.SendMsgToTgbot(chatID, output.String())
									}
									chatIDsDone = append(chatIDsDone, chatID)
								}
							}
						}
					}
				}
			}
		}
	}
}
