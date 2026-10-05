package tgbot

import (
	"context"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Views for the read-only screens. Each one is a pure "read state, return a
// screen" function; the router only decides which view to show.

// screenHome is the admin's landing screen: the numbers you open the bot for,
// then the menu. The picture is the bot's own avatar until the panel can host
// its own artwork.
func (t *Tgbot) screenHome(chatID int64) {
	body := t.homeSummary()
	t.renderScreen(chatID, t.newScreen("main", body, t.homeRows()...))
}

// homeSummary is the panel overview line used by the menu and the report, so
// the two can never disagree about the host or the version.
func (t *Tgbot) homeSummary() string {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load inbounds for the menu:", err)
	}
	clients := 0
	online := 0
	traffic := int64(0)
	for _, ib := range inbounds {
		clients += len(ib.ClientStats)
		traffic += ib.Up + ib.Down
	}
	if process := xrayProcessOrNil(); process != nil {
		online = len(process.GetOnlineClients())
	}
	body := ""
	body += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	body += t.I18nBot("tgbot.messages.version", "Version=="+panelVersion())
	body += t.I18nBot("tgbot.messages.inboundsCount", "Count=="+strconv.Itoa(len(inbounds)))
	body += t.I18nBot("tgbot.messages.clientsCount", "Count=="+strconv.Itoa(clients))
	body += t.I18nBot("tgbot.messages.onlinesCount", "Count=="+strconv.Itoa(online))
	body += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(traffic),
		"Upload=="+common.FormatTraffic(traffic), "Download=="+common.FormatTraffic(0))
	return body
}

// screenServer renders the server-usage screen.
func (t *Tgbot) screenServer(chatID int64) {
	body := strings.TrimRight(t.prepareServerUsageInfo(), "\r\n")
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", cbServer)),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("status", body, rows...))
}

// screenOnlines lists who is connected right now. The client names used to be
// buttons; they are plain text here, and the inline browser takes over the
// per-client pick in the next step.
func (t *Tgbot) screenOnlines(chatID int64) {
	process := xrayProcessOrNil()
	if process == nil {
		t.renderScreen(chatID, t.newScreen("onlines", t.I18nBot("tgbot.wentWrong"),
			tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", cbOnlines)), t.backRow()))
		return
	}
	onlines := process.GetOnlineClients()
	body := t.I18nBot("tgbot.messages.onlinesCount", "Count=="+strconv.Itoa(len(onlines)))
	for _, online := range onlines {
		label := online
		if _, inbound, err := t.inboundService.GetClientInboundByEmail(online); err == nil && inbound != nil && inbound.Remark != "" {
			label = online + " · " + inbound.Remark
		}
		body += "• " + html.EscapeString(label) + "\n"
	}
	body += t.I18nBot("tgbot.messages.refreshedOn", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", cbOnlines)),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("onlines", body, rows...))
}

// screenDeplete renders the "running out" screen. Both the scheduled report
// and the button use this text, so a client can never be listed twice with a
// different story.
func (t *Tgbot) screenDeplete(chatID int64) {
	body := t.depleteReport()
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", cbDeplete)),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("deplete", body, rows...))
}

// depleteReport builds the exhausted/disabled summary as plain text.
func (t *Tgbot) depleteReport() string {
	trDiff, exDiff := t.depletionThresholds()
	now := time.Now().Unix() * 1000

	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load inbounds for the depletion report:", err)
	}
	exhaustedInbounds, disabledInbounds := 0, 0
	exhaustedClients, disabledClients := 0, 0
	seen := map[string]bool{}
	var clientLines strings.Builder

	for _, inbound := range inbounds {
		if !inbound.Enable {
			disabledInbounds++
			continue
		}
		if (inbound.ExpiryTime > 0 && inbound.ExpiryTime-now < exDiff) ||
			(inbound.Total > 0 && inbound.Total-(inbound.Up+inbound.Down) < trDiff) {
			exhaustedInbounds++
		}
		for _, client := range inbound.ClientStats {
			if seen[client.Email] {
				continue
			}
			seen[client.Email] = true
			if !client.Enable {
				disabledClients++
				continue
			}
			if (client.ExpiryTime > 0 && client.ExpiryTime-now < exDiff) ||
				(client.Total > 0 && client.Total-(client.Up+client.Down) < trDiff) {
				exhaustedClients++
				clientLines.WriteString(t.clientInfoMsg(&client, true, false, false, true, true, false))
				clientLines.WriteString("\n")
			}
		}
	}

	body := ""
	body += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.inbounds"))
	body += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(disabledInbounds))
	body += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(exhaustedInbounds))
	body += "\n"
	body += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.clients"))
	body += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(disabledClients))
	body += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(exhaustedClients))
	if clientLines.Len() > 0 {
		body += "\n" + html.EscapeString(clientLines.String())
	}
	body += t.I18nBot("tgbot.messages.refreshedOn", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	return body
}

// depletionThresholds reads the two "running out" windows from the settings.
func (t *Tgbot) depletionThresholds() (traffic, expire int64) {
	if v, err := t.settingService.GetTrafficDiff(); err == nil && v > 0 {
		traffic = int64(v) * 1073741824
	}
	if v, err := t.settingService.GetExpireDiff(); err == nil && v > 0 {
		expire = int64(v) * 86400000
	}
	return traffic, expire
}

// screenReport renders the sorted traffic-usage report. The screen pages it,
// so a large panel gets one message with page controls, not a message flood.
func (t *Tgbot) screenReport(chatID int64) {
	emails, err := t.inboundService.GetAllEmails()
	if err != nil {
		t.renderScreen(chatID, t.newScreen("reports", t.I18nBot("tgbot.answers.errorOperation"), t.backRow()))
		return
	}
	validEmails, missingEmails, err := t.inboundService.FilterAndSortClientEmails(emails)
	if err != nil {
		t.renderScreen(chatID, t.newScreen("reports", t.I18nBot("tgbot.answers.errorOperation"), t.backRow()))
		return
	}

	var report strings.Builder
	for _, email := range validEmails {
		traffic, err := t.inboundService.GetClientTrafficByEmail(email)
		if err != nil || traffic == nil {
			fmt.Fprintf(&report, "📧 %s\n%s\n", html.EscapeString(email), t.I18nBot("tgbot.noResult"))
			continue
		}
		report.WriteString(t.clientInfoMsg(traffic, false, false, false, false, true, false))
		report.WriteString("\n")
	}
	for _, email := range missingEmails {
		fmt.Fprintf(&report, "📧 %s\n%s\n", html.EscapeString(email), t.I18nBot("tgbot.noResult"))
	}
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", cbReport)),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("reports", html.EscapeString(report.String()), rows...))
}

// screenCommands explains the classic commands; kept because muscle memory is
// real, but the menu no longer advertises them.
func (t *Tgbot) screenCommands(chatID int64, isAdmin bool) {
	key := "tgbot.commands.helpClientCommands"
	if isAdmin {
		key = "tgbot.commands.helpAdminCommands"
	}
	t.renderScreen(chatID, t.newScreen("commands", t.I18nBot(key), t.backRow()))
}

// screenActions holds the heavier operations behind one extra tap.
func (t *Tgbot) screenActions(chatID int64) {
	t.renderScreen(chatID, t.newScreen("main", t.I18nBot("tgbot.messages.actionsHint"), t.actionsRows()...))
}

// sendBackupScreen delivers the backup files. A file cannot be a screen's
// picture, so each file is a message with a hide button and the chat keeps no
// screen state.
func (t *Tgbot) sendBackupScreen(chatID int64) {
	dbData, err := t.serverService.GetDb()
	if err != nil {
		logger.Error("Error in getting db backup:", err)
	}
	output := t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	output += t.I18nBot("tgbot.messages.backupTime", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	t.sendNotice(chatID, output)

	if dbData != nil {
		t.sendDocumentWithHide(chatID, dbData, t.serverService.BackupFilename(""))
	}
}

// screenBanLogs sends the ban log file as a notice, never as a screen.
func (t *Tgbot) screenBanLogs(chatID int64) {
	output := t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	output += t.I18nBot("tgbot.messages.datetime", "DateTime=="+time.Now().Format("2006-01-02 15:04:05"))
	t.sendNotice(chatID, output)
	t.sendBanLogsFile(chatID)
}

// sendNotice posts a plain bot message that is explicitly not a screen: it
// carries the hide button, and it never becomes the chat's tracked screen.
func (t *Tgbot) sendNotice(chatID int64, text string) {
	if bot == nil {
		return
	}
	for _, chunk := range pageMessage(text, telegramPageLimit) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: tu.ID(chatID), Text: chunk, ParseMode: "HTML", ReplyMarkup: t.hideButton(),
		})
		cancel()
		if err != nil {
			logger.Warning("Failed to send a notice:", err)
		}
	}
}

// sendBanLogsFile delivers the banned-IP log as a notice with a hide button.
func (t *Tgbot) sendBanLogsFile(chatID int64) {
	file, err := os.Open(xray.GetIPLimitBannedPrevLogPath())
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	defer file.Close()
	if bot == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err = bot.SendDocument(ctx, &telego.SendDocumentParams{
		ChatID:      tu.ID(chatID),
		Document:    tu.File(file),
		ReplyMarkup: t.hideButton(),
	})
	if err != nil {
		logger.Warning("Failed to upload the ban log:", err)
	}
}
