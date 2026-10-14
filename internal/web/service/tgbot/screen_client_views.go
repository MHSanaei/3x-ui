package tgbot

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/skip2/go-qrcode"
)

func qrPNG(content string, size int) ([]byte, error) {
	if size <= 0 {
		size = 256
	}
	return qrcode.Encode(content, qrcode.Medium, size)
}

// The client card and its screens (limits, expiry, IP log, TG binding, links,
// QR) all redraw the same tracked message, so none leaves the previous behind.

// clientScreenBody is the shared header of every per-client screen: a user who
// tapped through four levels still sees which client they are on.
func (t *Tgbot) clientScreenBody(email, title string) string {
	body := "<b>" + html.EscapeString(email) + "</b>\n"
	if title != "" {
		body += html.EscapeString(title) + "\n\n"
	}
	return body
}

// screenClientIps draws the IP log with refresh and clear.
func (t *Tgbot) screenClientIps(chatID int64, email string) {
	body := t.clientScreenBody(email, "")
	ips, err := t.inboundService.GetInboundClientIps(email)
	if err != nil || len(ips) == 0 {
		body += t.I18nBot("tgbot.noIpRecord")
	} else {
		body += t.I18nBot("tgbot.messages.ips", "IPs=="+html.EscapeString(formatIPLog(ips)))
	}
	body += t.I18nBot("tgbot.messages.refreshedOn", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", t.encodeQuery("ips_refresh "+email))),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.clearIPs", t.encodeQuery("clear_ips "+email))),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.clientCard", t.encodeQuery("client_get_usage "+email))),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("client", body, rows...))
}

// formatIPLog renders the stored IP log, which is either a list of addresses or
// a list of {ip, timestamp} objects depending on when it was written.
func formatIPLog(raw string) string {
	type ipWithTimestamp struct {
		IP        string `json:"ip"`
		Timestamp int64  `json:"timestamp"`
	}
	var entries []ipWithTimestamp
	if json.Unmarshal([]byte(raw), &entries) == nil && len(entries) > 0 {
		lines := make([]string, 0, len(entries))
		for _, item := range entries {
			if item.IP == "" {
				continue
			}
			if item.Timestamp > 0 {
				lines = append(lines, fmt.Sprintf("%s (%s)", item.IP, time.Unix(item.Timestamp, 0).Format("2006-01-02 15:04:05")))
				continue
			}
			lines = append(lines, item.IP)
		}
		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}
	var plain []string
	if json.Unmarshal([]byte(raw), &plain) == nil && len(plain) > 0 {
		return strings.Join(plain, "\n")
	}
	return raw
}

// screenClientTG draws a client's Telegram binding. The user picker is the one
// reply keyboard kept: the API has no inline way to request a user.
func (t *Tgbot) screenClientTG(chatID int64, email string, withPicker bool) {
	traffic, client, err := t.inboundService.GetClientByEmail(email)
	if err != nil || client == nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	tgID := t.I18nBot("tgbot.noValue")
	if client.TgID != 0 {
		tgID = strconv.FormatInt(client.TgID, 10)
	}
	body := t.clientScreenBody(email, "")
	body += t.I18nBot("tgbot.messages.TGUser", "TelegramID=="+tgID)
	body += t.I18nBot("tgbot.messages.refreshedOn", "Time=="+time.Now().Format("2006-01-02 15:04:05"))

	rows := [][]telego.InlineKeyboardButton{}
	if withPicker {
		rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.selectTGUser", t.encodeQuery("tgid_pick "+email))))
	}
	rows = append(rows,
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.refresh", t.encodeQuery("tgid_refresh "+email))),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.removeTGUser", t.encodeQuery("tgid_remove "+email))),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.clientCard", t.encodeQuery("client_get_usage "+email))),
		t.backRow(),
	)
	t.renderScreen(chatID, t.newScreen("client", body, rows...))
	if withPicker {
		t.sendTGPicker(chatID, traffic.Id)
	}
}

// sendTGPicker posts the native user picker — the only way the API asks for a
// user. It is a real message, so it carries the hide button and is removed later.
func (t *Tgbot) sendTGPicker(chatID int64, trafficID int) {
	if liveBot() == nil {
		return
	}
	t.sendNotice(chatID, t.I18nBot("tgbot.buttons.selectOneTGUser"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	request := telego.KeyboardButtonRequestUsers{RequestID: int32(trafficID), UserIsBot: new(bool)}
	keyboard := tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(t.I18nBot("tgbot.buttons.selectTGUser")).WithRequestUsers(&request)),
		tu.KeyboardRow(tu.KeyboardButton(t.I18nBot("tgbot.buttons.closeKeyboard"))),
	).WithIsPersistent().WithResizeKeyboard()
	_, _ = liveBot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID: tu.ID(chatID), Text: t.I18nBot("tgbot.buttons.selectOneTGUser"), ReplyMarkup: keyboard,
	})
}

// ownSubscriptionOnly reports whether the screen is the client's own single
// subscription, where admin-only controls must stay off (reached via "Links").
func (t *Tgbot) ownSubscriptionOnly(chatID int64, email string) bool {
	if checkAdmin(chatID) {
		return false
	}
	traffics, err := t.inboundService.GetClientTrafficTgBot(chatID)
	if err != nil {
		return false
	}
	return len(traffics) == 1 && traffics[0].Email == email
}

// screenClientLinks draws the subscription and the links of a client.
func (t *Tgbot) screenClientLinks(chatID int64, email string) {
	subURL, subJSON, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	body := t.clientScreenBody(email, "")
	body += t.I18nBot("tgbot.messages.subURL", "URL=="+html.EscapeString(subURL))
	if subJSON != "" {
		body += t.I18nBot("tgbot.messages.jsonURL", "URL=="+html.EscapeString(subJSON))
	}
	body += t.I18nBot("tgbot.messages.refreshedOn", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.individualLinks", t.encodeQuery("client_individual_links "+email))),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.qrCode", t.encodeQuery("client_qr_links "+email))),
	}
	// A client looking at its own subscription has nothing behind the admin's
	// card: it only repeats the usage already on its home screen.
	if !t.ownSubscriptionOnly(chatID, email) {
		rows = append(rows, tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.clientCard", t.encodeQuery("client_get_usage "+email))))
	}
	rows = append(rows, t.backRow())
	t.renderScreen(chatID, t.newScreen("links", body, rows...))
}

// screenIndividualLinks renders the per-app links of the subscription as one
// (paged) message instead of one message per app.
func (t *Tgbot) screenIndividualLinks(chatID int64, email string) {
	subURL, _, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	links, err := t.clientSubLinks(email, subURL)
	if err != nil || len(links) == 0 {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	var body strings.Builder
	body.WriteString(t.clientScreenBody(email, t.I18nBot("tgbot.buttons.individualLinks")))
	for _, link := range links {
		body.WriteString("<code>" + html.EscapeString(link) + "</code>\n")
	}
	t.renderScreen(chatID, t.newScreen("links", body.String(), t.individualLinksRows()...))
}

// individualLinksRows is the individual-links screen's keyboard: exactly one
// control, the shared back row, since a second "Links" button was a duplicate.
func (t *Tgbot) individualLinksRows() [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{t.backRow()}
}

// screenClientQR renders the subscription QR as the screen picture and leaves
// the individual QRs to a document, so one screen covers the tap.
func (t *Tgbot) screenClientQR(chatID int64, email string) {
	subURL, _, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	body := t.clientScreenBody(email, t.I18nBot("tgbot.answers.qrCodeForClient", "Email=="+email))
	body += t.I18nBot("tgbot.messages.subURL", "URL=="+html.EscapeString(subURL))
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.individualLinks", t.encodeQuery("client_individual_links "+email))),
		t.backRow(),
	}
	// One picture carrying every code the bot once sent as separate documents;
	// tiles carry no text (they are scanned), so the caption legend numbers them.
	codes := t.subscriptionQRCodes(email, subURL)
	if len(codes) == 0 {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	sc := t.newScreen("qr", body+t.qrCodeLegend(codes), rows...)
	sc.qrPicture = qrSheetPicture(codes)
	sc.qrSource = subURL
	if sc.qrPicture == nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	t.renderScreen(chatID, sc)
}

// renderPickerOnClient redraws the client card with a picker's rows appended, so
// tapping a limit opens its value list on the same tracked screen.
func (t *Tgbot) renderPickerOnClient(chatID int64, email string, picker [][]telego.InlineKeyboardButton) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	rows := append([][]telego.InlineKeyboardButton{}, picker...)
	rows = append(rows, t.backRow())
	body := t.clientScreenBody(email, t.clientInfoMsg(traffic, true, true, true, true, true, false))
	t.renderScreen(chatID, t.newScreen("client", body, rows...))
}

// renderDraftPicker redraws the add-client draft with a value picker's rows
// appended: the wizard's own card stays visible while a value is chosen.
func (t *Tgbot) renderDraftPicker(chatID int64, draft *clientDraft, picker [][]telego.InlineKeyboardButton) {
	rows := append([][]telego.InlineKeyboardButton{}, t.getCommonClientButtons(draft)...)
	rows = append(rows, picker...)
	t.renderScreen(chatID, t.newScreen("wizard", t.BuildClientDraftMessage(draft), rows...))
}

// adoptScreen marks the message the user is looking at as this chat's screen,
// so the next render edits it instead of stacking a second screen under it.
func (t *Tgbot) adoptScreen(chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	if _, ok := t.screens().get(chatID); ok {
		return
	}
	t.screens().put(chatID, &screen{msgID: msgID})
}

// clientTrafficID resolves the traffic row id a client's TG-user picker needs.
// A missing row is 0: the caller's screen already reports the unknown client.
func (t *Tgbot) clientTrafficID(email string) int {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		return 0
	}
	return traffic.Id
}

// sendNoticeNoKeyboard reports a result and drops the client picker's reply
// keyboard in one message: this is the only place it is torn down.
func (t *Tgbot) sendNoticeNoKeyboard(chatID int64, text string) {
	if liveBot() == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := liveBot().SendMessage(ctx, &telego.SendMessageParams{
		ChatID: tu.ID(chatID), Text: text, ParseMode: "HTML",
		ReplyMarkup: tu.ReplyKeyboardRemove(),
	})
	if err != nil {
		logger.Warning("Failed to send a notice:", err)
	}
}

// wizardPrompt asks for a text value on the draft screen itself: the wizard owns
// one message, so the question replaces the card; the reply keyboard is untouched.
func (t *Tgbot) wizardPrompt(chatID int64, draft *clientDraft, prompt string) {
	rows := append([][]telego.InlineKeyboardButton{}, t.getCommonClientButtons(draft)...)
	rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.use_default", "add_client_default_info")))
	t.renderScreen(chatID, t.newScreen("wizard", prompt, rows...))
}

// wizardInvalidInput redraws the draft with a rejection hint: the wizard keeps
// ONE message, so a retry does not stack a hint under a stale card.
func (t *Tgbot) wizardInvalidInput(chatID int64, draft *clientDraft) {
	rows := append([][]telego.InlineKeyboardButton{}, t.getCommonClientButtons(draft)...)
	rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.use_default", "add_client_default_info")))
	t.renderScreen(chatID, t.newScreen("wizard",
		t.BuildClientDraftMessage(draft)+"\n\n"+t.I18nBot("tgbot.messages.incorrect_input"), rows...))
}

// screenInviteLink draws the invite link of a client as instructions plus the
// link itself, so an admin can forward it without leaving the screen.
func (t *Tgbot) screenInviteLink(chatID int64, email string) {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	username := botUsername()
	if err != nil || record.SubID == "" || username == "" {
		logger.Warning("tgbot: invite link unavailable for", email, err)
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	payload, ok := encodeInvitePayload(record.SubID)
	if !ok {
		logger.Warning("tgbot: subId of", email, "is too long for a Telegram invite link")
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	link := "https://t.me/" + username + "?start=" + payload
	body := t.clientScreenBody(email, t.I18nBot("tgbot.buttons.inviteLink"))
	body += "<code>" + html.EscapeString(link) + "</code>\n\n"
	body += t.I18nBot("tgbot.messages.inviteHint")
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.clientCard", t.encodeQuery("client_get_usage "+email))),
		t.backRow(),
	}
	t.renderScreen(chatID, t.newScreen("client", body, rows...))
}

// openOwnClientScreen resolves the client behind a Telegram user and opens the
// asked-for screen; several subscriptions become one button each.
func (t *Tgbot) openOwnClientScreen(chatID, tgUserID int64, action string) {
	traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if len(traffics) == 0 {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.askToAddUserId", "TgUserID=="+strconv.FormatInt(tgUserID, 10)))
		return
	}
	if len(traffics) == 1 {
		t.openClientAction(chatID, action, traffics[0].Email)
		return
	}
	var buttons []telego.InlineKeyboardButton
	for _, tr := range traffics {
		buttons = append(buttons, tu.InlineKeyboardButton(tr.Email).
			WithCallbackData(t.encodeQuery(action+" "+tr.Email)))
	}
	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	rows := append(keyboard.InlineKeyboard, t.backRow())
	t.renderScreen(chatID, t.newScreen("links", t.I18nBot("tgbot.commands.pleaseChoose"), rows...))
}

// openClientAction opens one per-client screen by name.
func (t *Tgbot) openClientAction(chatID int64, action, email string) {
	switch action {
	case "client_sub_links":
		t.screenClientLinks(chatID, email)
	case "client_individual_links":
		t.screenIndividualLinks(chatID, email)
	case "client_qr_links":
		t.screenClientQR(chatID, email)
	}
}
