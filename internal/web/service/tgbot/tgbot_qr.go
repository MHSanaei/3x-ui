package tgbot

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/skip2/go-qrcode"
)

// A share link ends in "#remark", which is the name the client app will show,
// so it is the label a customer recognises when picking one of several.
func linkLabel(link string, index int) string {
	fallback := fmt.Sprintf("Link %d", index+1)
	if scheme, _, found := strings.Cut(link, "://"); found && scheme != "" {
		fallback = fmt.Sprintf("%d. %s", index+1, strings.ToUpper(scheme))
	}
	hash := strings.LastIndex(link, "#")
	if hash < 0 || hash+1 >= len(link) {
		return fallback
	}
	remark := strings.TrimSpace(link[hash+1:])
	if decoded, err := url.QueryUnescape(remark); err == nil {
		remark = strings.TrimSpace(decoded)
	}
	if remark == "" {
		return fallback
	}
	return fmt.Sprintf("%d. %s", index+1, remark)
}

func createQR(content string, size int) ([]byte, error) {
	if size <= 0 {
		size = 256
	}
	return qrcode.Encode(content, qrcode.Medium, size)
}

// Every QR carries a caption naming what it encodes: an image with no context
// is unreadable once it has scrolled up a chat.
func (t *Tgbot) sendQRPhoto(chatId int64, content, filename, caption string) {
	png, err := createQR(content, 320)
	if err != nil {
		logger.Warning("tgbot: qr encode failed:", err)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	photo := tu.Photo(tu.ID(chatId), tu.FileFromBytes(png, filename)).WithCaption(caption)
	if _, err := bot.SendPhoto(context.Background(), photo); err != nil {
		logger.Warning("tgbot: qr send failed:", err)
	}
}

// fetchIndividualLinks returns the client's share links, newline separated by
// the subscription server and optionally base64 encoded by its settings.
func (t *Tgbot) fetchIndividualLinks(email string) ([]string, error) {
	subURL, _, err := t.buildSubscriptionURLs(email)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, subURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/plain, */*;q=0.1")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := optimizedHTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(bodyBytes)
	if encoded, _ := t.settingService.GetSubEncrypt(); encoded {
		if decoded, err := base64.StdEncoding.DecodeString(content); err == nil {
			content = string(decoded)
		}
	}

	cleaned := make([]string, 0, 8)
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return cleaned, nil
}

func (t *Tgbot) sendClientIndividualLinks(chatId int64, email string) {
	cleaned, err := t.fetchIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if len(cleaned) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	const maxPerMessage = 50
	for i := 0; i < len(cleaned); i += maxPerMessage {
		j := min(i+maxPerMessage, len(cleaned))
		var msg strings.Builder
		// Only the first chunk is headed, so a long link list does not repeat
		// the client's expiry once per message.
		if i == 0 {
			msg.WriteString(t.clientHeader(email))
		}
		msg.WriteString(t.I18nBot("tgbot.messages.configHeaderAll"))
		msg.WriteString(":\r\n")
		for _, link := range cleaned[i:j] {
			msg.WriteString("<code>")
			msg.WriteString(link)
			msg.WriteString("</code>\r\n")
		}
		// The QR button rides on the last chunk only, so a long link list does
		// not repeat it once per message.
		if j >= len(cleaned) {
			t.SendMsgToTgbot(chatId, msg.String(), tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrForLink")).WithCallbackData(t.encodeQuery("qr_pick "+email)),
				),
			))
			continue
		}
		t.SendMsgToTgbot(chatId, msg.String())
	}
}

func (t *Tgbot) sendSubscriptionQR(chatId int64, email string, jsonVariant bool) {
	subURL, subJsonURL, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if jsonVariant {
		if subJsonURL == "" {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
			return
		}
		t.sendQRPhoto(chatId, subJsonURL, "subjson.png", t.I18nBot("tgbot.messages.qrCaptionSubJson", "Email=="+email))
		return
	}
	t.sendQRPhoto(chatId, subURL, "sub.png", t.I18nBot("tgbot.messages.qrCaptionSub", "Email=="+email))
}

// Asks which link before sending anything: a client attached to six inbounds
// would otherwise receive six images nobody asked for.
func (t *Tgbot) qrLinkPicker(chatId int64, email string) {
	cleaned, err := t.fetchIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if len(cleaned) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}
	if len(cleaned) == 1 {
		t.sendIndividualLinkQR(chatId, email, 0)
		return
	}

	buttons := make([]telego.InlineKeyboardButton, 0, len(cleaned))
	for i, link := range cleaned {
		buttons = append(buttons, tu.InlineKeyboardButton(linkLabel(link, i)).
			WithCallbackData(t.encodeQuery(fmt.Sprintf("qr_one %s %d", email, i))))
	}
	t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.qrChooseLink", "Email=="+email),
		tu.InlineKeyboardGrid(tu.InlineKeyboardCols(1, buttons...)))
}

// The list is re-fetched rather than cached, so a stale button from an older
// message cannot send a QR for a link that has since changed or gone.
func (t *Tgbot) sendIndividualLinkQR(chatId int64, email string, index int) {
	cleaned, err := t.fetchIndividualLinks(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	if index < 0 || index >= len(cleaned) {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}
	link := cleaned[index]
	caption := t.I18nBot("tgbot.messages.qrCaptionLink", "Email=="+email, "Label=="+linkLabel(link, index))
	t.sendQRPhoto(chatId, link, email+".png", caption)
}

// The entry point from the main keyboard, where there is no message context to
// say which QR was meant, so it asks rather than sending everything.
func (t *Tgbot) sendClientQRLinks(chatId int64, email string) {
	_, subJsonURL, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrSubscription")).WithCallbackData(t.encodeQuery("qr_sub " + email)),
		),
	}
	if subJsonURL != "" {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrSubJson")).WithCallbackData(t.encodeQuery("qr_subjson "+email)),
		))
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.qrForLink")).WithCallbackData(t.encodeQuery("qr_pick "+email)),
	))
	t.SendMsgToTgbot(chatId, t.clientHeader(email)+t.I18nBot("tgbot.messages.qrChoose", "Email=="+email), tu.InlineKeyboardGrid(rows))
}

// splitEmailIndexTarget pulls "<email> <index>" out of a qr_one or link_one
// callback.
func splitEmailIndexTarget(arg string) (string, int, bool) {
	email, idx, found := strings.Cut(strings.TrimSpace(arg), " ")
	if !found || email == "" {
		return "", 0, false
	}
	index, err := strconv.Atoi(strings.TrimSpace(idx))
	if err != nil || index < 0 {
		return "", 0, false
	}
	return email, index, true
}
