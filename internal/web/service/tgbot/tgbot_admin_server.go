package tgbot

import (
	"html"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	// SendMsgToTgbot only breaks a long message on blank lines, and a log dump
	// has none, so it has to arrive already trimmed under Telegram's size cap.
	logMessageLimit = 3500
	logLineCount    = "50"
	panelLogLevel   = "info"
)

// GetXrayLogs classifies every access-log line as direct, blocked or proxied,
// in that order, and hands the classification back as a bare index.
func xrayEventIcon(event int) string {
	switch event {
	case 0:
		return "🌐"
	case 1:
		return "⛔"
	default:
		return "🔀"
	}
}

// An admin scrolling a log wants the newest lines, so an oversized dump loses
// its head; every line is escaped because the bot sends with ParseMode HTML.
func logTail(lines []string, limit int) string {
	kept := make([]string, 0, len(lines))
	total := 0
	for i := len(lines) - 1; i >= 0; i-- {
		line := html.EscapeString(lines[i])
		if total+len(line)+2 > limit {
			break
		}
		total += len(line) + 2
		kept = append(kept, line)
	}
	slices.Reverse(kept)
	return strings.Join(kept, "\r\n")
}

// A parse failure leaves individual fields blank rather than dropping the entry,
// so each optional part is emitted only when it actually carries something.
func formatXrayLogEntries(entries []service.LogEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		var line strings.Builder
		line.WriteString(xrayEventIcon(entry.Event))
		if !entry.DateTime.IsZero() {
			line.WriteString(" " + entry.DateTime.Local().Format("01-02 15:04:05"))
		}
		line.WriteString(" " + entry.FromAddress + " → " + entry.ToAddress)
		if entry.Email != "" {
			line.WriteString(" (" + entry.Email + ")")
		}
		lines = append(lines, line.String())
	}
	return lines
}

// Split from serverMenu so the layout can be asserted without a running Xray
// or a live bot.
func (t *Tgbot) serverKeyboard(running bool) *telego.InlineKeyboardMarkup {
	xrayToggle := tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.xrayStart")).WithCallbackData(t.encodeQuery("server_xray_restart"))
	if running {
		xrayToggle = tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.xrayStop")).WithCallbackData(t.encodeQuery("server_xray_stop"))
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.serverUsage")).WithCallbackData(t.encodeQuery("get_usage")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.dbBackup")).WithCallbackData(t.encodeQuery("get_backup")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.panelLogs")).WithCallbackData(t.encodeQuery("server_panel_logs")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.xrayLogs")).WithCallbackData(t.encodeQuery("server_xray_logs")),
		),
		tu.InlineKeyboardRow(
			xrayToggle,
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.xrayRestart")).WithCallbackData(t.encodeQuery("server_xray_restart")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.toggleInbound")).WithCallbackData(t.encodeQuery("server_inbounds")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.panelRestart")).WithCallbackData(t.encodeQuery("server_panel_restart")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_panel")),
		),
	)
}

func (t *Tgbot) serverMenu(chatId int64) {
	running := t.xrayService.IsXrayRunning()

	header := t.I18nBot("tgbot.messages.serverXrayStopped")
	if running {
		header = t.I18nBot("tgbot.messages.serverXrayRunning")
	}
	t.SendMsgToTgbot(chatId, header, t.serverKeyboard(running))
}

func (t *Tgbot) sendPanelLogs(chatId int64) {
	lines := serverService.GetLogs(logLineCount, panelLogLevel, "false")
	// logger.GetLogs answers newest first, which reads backwards in a chat, so
	// the dump is flipped into file order before the tail is taken.
	slices.Reverse(lines)
	t.sendLogDump(chatId, lines)
}

func (t *Tgbot) sendXrayLogs(chatId int64) {
	freedoms, blackholes := serverService.GetDefaultLogOutboundTags()
	entries := serverService.GetXrayLogs(logLineCount, "", "true", "true", "true", freedoms, blackholes)
	t.sendLogDump(chatId, formatXrayLogEntries(entries))
}

func (t *Tgbot) sendLogDump(chatId int64, lines []string) {
	dump := logTail(lines, logMessageLimit)
	if dump == "" {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.logsEmpty"))
		return
	}
	t.SendMsgToTgbot(chatId, "<code>"+dump+"</code>")
}

func (t *Tgbot) restartXray(chatId int64) {
	if err := t.xrayService.RestartXray(true); err != nil {
		logger.Warning("tgbot: xray restart failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.restartFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.restartSuccess"))
}

func (t *Tgbot) stopXray(chatId int64) {
	if err := t.xrayService.StopXray(); err != nil {
		logger.Warning("tgbot: xray stop failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.xrayStopFailed", "Error=="+err.Error()))
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.xrayStopped"))
}

// The confirmation is sent before the restart is scheduled, because the process
// is replaced a few seconds later and nothing queued after it would go out.
func (t *Tgbot) restartPanel(chatId int64) {
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.panelRestarting"))
	if err := t.panelService.RestartPanel(3 * time.Second); err != nil {
		logger.Warning("tgbot: panel restart failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.panelRestartFailed", "Error=="+err.Error()))
	}
}

func (t *Tgbot) toggleInbound(chatId int64, arg string, messageID int) {
	id, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.incorrect_input"))
		return
	}
	inbound, err := t.inboundService.GetInbound(id)
	if err != nil {
		logger.Warning("tgbot: inbound lookup failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inboundToggleFailed", "Error=="+err.Error()))
		return
	}

	needRestart, err := t.inboundService.SetInboundEnable(id, !inbound.Enable)
	if needRestart {
		t.xrayService.SetToNeedRestart()
	}
	if err != nil {
		logger.Warning("tgbot: inbound toggle failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.inboundToggleFailed", "Error=="+err.Error()))
		return
	}

	messageKey := "tgbot.messages.inboundDisabled"
	if !inbound.Enable {
		messageKey = "tgbot.messages.inboundEnabled"
	}
	t.SendMsgToTgbot(chatId, t.I18nBot(messageKey, "Remark=="+inbound.Remark))
	t.refreshInboundPicker(chatId, messageID)
}

func (t *Tgbot) refreshInboundPicker(chatId int64, messageID int) {
	keyboard, err := t.getInboundsFor("server_inbound_toggle")
	if err != nil {
		t.SendMsgToTgbot(chatId, err.Error())
		return
	}
	if messageID > 0 {
		t.editMessageCallbackTgBot(chatId, messageID, keyboard)
		return
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), keyboard)
}

func (t *Tgbot) confirmKeyboard(confirmKey, confirmData, cancelData string) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery(cancelData)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot(confirmKey)).WithCallbackData(t.encodeQuery(confirmData)),
		),
	)
}
