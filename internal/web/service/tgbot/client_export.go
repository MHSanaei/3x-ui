package tgbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

func privateAdmin(chat telego.Chat, userID int64) bool {
	return chat.Type == "private" && chat.ID == userID && checkAdmin(userID)
}

func (t *Tgbot) handleClientExport(query *telego.CallbackQuery) bool {
	if query == nil {
		return false
	}
	decoded, err := t.decodeQuery(query.Data)
	if err != nil || !strings.HasPrefix(decoded, "client_export_") {
		return false
	}
	if query.Message == nil || !privateAdmin(query.Message.GetChat(), query.From.ID) {
		t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.answers.errorOperation"))
		return true
	}
	parts := strings.SplitN(decoded, " ", 2)
	if len(parts) != 2 || parts[1] == "" {
		t.answerSilent(query.ID)
		return true
	}
	chatID, email := query.Message.GetChat().ID, parts[1]
	if parts[0] == "client_export_menu" {
		if _, err := t.clientService.GetRecordByEmail(nil, email); err != nil {
			t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.noResult"))
			return true
		}
		t.answerSilent(query.ID)
		t.screenClientExport(chatID, email)
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := t.sendClientExport(ctx, chatID, email, strings.TrimPrefix(parts[0], "client_export_")); err != nil {
		t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.messages.exportFailed"))
	} else {
		t.answerSilent(query.ID)
	}
	return true
}

func (t *Tgbot) screenClientExport(chatID int64, email string) {
	var rows [][]telego.InlineKeyboardButton
	for _, item := range []struct{ action, key string }{
		{"summary", "tgbot.buttons.exportSummary"},
		{"url", "tgbot.buttons.exportUrl"},
		{"txt", "tgbot.buttons.exportTxt"},
		{"qr", "tgbot.buttons.exportQr"},
	} {
		rows = append(rows, tu.InlineKeyboardRow(t.btn(item.key, t.encodeQuery("client_export_"+item.action+" "+email))))
	}
	if enabled, err := t.settingService.GetSubJsonEnable(); err == nil && enabled {
		rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.exportJson", t.encodeQuery("client_export_json "+email))))
	}
	rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.clientCard", t.encodeQuery("client_get_usage "+email))), t.backRow())
	t.renderScreen(chatID, t.newScreen("client", t.clientScreenBody(email, t.I18nBot("tgbot.buttons.shareExport")), rows...))
}

func validateSubscriptionURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid subscription URL")
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || strings.ContainsFunc(raw, unicode.IsSpace) || strings.ContainsFunc(raw, unicode.IsControl) {
		return errors.New("invalid subscription URL")
	}
	return nil
}

func exportFilename(email string) string {
	var name strings.Builder
	for _, r := range email {
		if name.Len() >= 100 {
			break
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' {
			name.WriteRune(r)
		}
	}
	if name.Len() == 0 {
		name.WriteString("client")
	}
	return name.String() + ".txt"
}

func telegramTextFits(text string, limit int) bool { return len(utf16.Encode([]rune(text))) <= limit }

func (t *Tgbot) sendExportFile(ctx context.Context, chatID int64, data []byte, filename string) error {
	b := liveBot()
	if b == nil {
		return errors.New("Telegram bot unavailable")
	}
	_, err := b.SendDocument(ctx, &telego.SendDocumentParams{ChatID: tu.ID(chatID), Document: tu.FileFromBytes(data, filename)})
	return err
}

func (t *Tgbot) sendExportText(ctx context.Context, chatID int64, text, filename string) error {
	if !telegramTextFits(text, 4096) {
		plain := html.UnescapeString(strings.TrimSuffix(strings.TrimPrefix(text, "<code>"), "</code>"))
		return t.sendExportFile(ctx, chatID, []byte(plain+"\n"), filename)
	}
	b := liveBot()
	if b == nil {
		return errors.New("Telegram bot unavailable")
	}
	_, err := b.SendMessage(ctx, &telego.SendMessageParams{ChatID: tu.ID(chatID), Text: text, ParseMode: "HTML", LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: true}})
	return err
}

func (t *Tgbot) sendClientExport(ctx context.Context, chatID int64, email, kind string) error {
	if kind == "summary" {
		body, err := t.clientExportSummary(email)
		if err != nil {
			return err
		}
		return t.sendExportText(ctx, chatID, body, exportFilename(email))
	}
	primary, jsonURL, err := t.buildSubscriptionURLs(email)
	if err != nil {
		return err
	}
	if enabled, err := t.settingService.GetSubEnable(); err != nil || !enabled {
		return errors.New("subscription server disabled")
	}
	if err := validateSubscriptionURL(primary); err != nil {
		return err
	}
	switch kind {
	case "url":
		return t.sendExportText(ctx, chatID, "<code>"+html.EscapeString(primary)+"</code>", exportFilename(email))
	case "json":
		if err := validateSubscriptionURL(jsonURL); err != nil {
			return err
		}
		return t.sendExportText(ctx, chatID, "<code>"+html.EscapeString(jsonURL)+"</code>", exportFilename(email))
	case "txt":
		links, err := t.clientSubLinks(email, primary)
		if err != nil {
			return err
		}
		if len(links) == 0 {
			return errors.New("no individual links available")
		}
		return t.sendExportFile(ctx, chatID, []byte(strings.Join(links, "\n")+"\n"), exportFilename(email))
	case "qr":
		picture, err := qrPNG(primary, 512)
		if err != nil {
			return fmt.Errorf("subscription QR: %w", err)
		}
		caption := "<code>" + html.EscapeString(primary) + "</code>"
		separate := !telegramTextFits(caption, botCaptionLimit)
		if separate {
			caption = ""
		}
		b := liveBot()
		if b == nil {
			return errors.New("Telegram bot unavailable")
		}
		_, err = b.SendPhoto(ctx, &telego.SendPhotoParams{ChatID: tu.ID(chatID), Photo: tu.FileFromBytes(picture, "subscription.png"), Caption: caption, ParseMode: "HTML"})
		if err != nil {
			return err
		}
		if separate {
			return t.sendExportText(ctx, chatID, "<code>"+html.EscapeString(primary)+"</code>", exportFilename(email))
		}
		return nil
	default:
		return errors.New("unknown export action")
	}
}

func (t *Tgbot) expiryLabel(expiry int64) string {
	if expiry == 0 {
		return t.I18nBot("tgbot.unlimited")
	}
	if expiry < 0 {
		return strconv.FormatInt(-expiry/86400000, 10) + " " + t.I18nBot("tgbot.days") + " " + t.I18nBot("tgbot.messages.firstUse")
	}
	return time.UnixMilli(expiry).Format("2006-01-02 15:04:05 MST")
}

func (t *Tgbot) clientExportSummary(email string) (string, error) {
	record, err := t.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return "", err
	}
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil || traffic == nil {
		return "", errors.New("client traffic unavailable")
	}
	inbounds, err := t.clientService.GetAttachedInboundLabels(email)
	if err != nil {
		return "", err
	}
	labels := make([]string, 0, len(inbounds))
	for _, inbound := range inbounds {
		labels = append(labels, fmt.Sprintf("#%d %s", inbound.Id, inbound.Remark))
	}
	enabled := t.I18nBot("tgbot.messages.no")
	if record.Enable {
		enabled = t.I18nBot("tgbot.messages.yes")
	}
	quota, remaining := t.I18nBot("tgbot.unlimited"), t.I18nBot("tgbot.unlimited")
	if traffic.Total > 0 {
		quota = common.FormatTraffic(traffic.Total)
		left := traffic.Total - traffic.Up - traffic.Down
		if left < 0 {
			left = 0
		}
		remaining = common.FormatTraffic(left)
	}
	return t.I18nBot("tgbot.messages.exportSummary",
		"Email=="+html.EscapeString(email), "Enabled=="+enabled, "Quota=="+quota,
		"Used=="+common.FormatTraffic(traffic.Up+traffic.Down), "Remaining=="+remaining,
		"Expiry=="+html.EscapeString(t.expiryLabel(traffic.ExpiryTime)), "Inbounds=="+html.EscapeString(strings.Join(labels, ", "))), nil
}
