package tgbot

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/link"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// The link's remark is the name the client app will show, so it is the label a
// customer recognises when picking one of several.
func linkLabel(shareLink string, index int) string {
	fallback := fmt.Sprintf("Link %d", index+1)
	if scheme, _, found := strings.Cut(shareLink, "://"); found && scheme != "" {
		fallback = fmt.Sprintf("%d. %s", index+1, strings.ToUpper(scheme))
	}
	remark := linkRemark(shareLink)
	if remark == "" {
		return fallback
	}
	return fmt.Sprintf("%d. %s", index+1, remark)
}

// VMess keeps its remark as "ps" inside the base64 JSON; every other scheme
// carries it as the "#remark" fragment.
func linkRemark(shareLink string) string {
	if strings.HasPrefix(shareLink, "vmess://") {
		parsed, err := link.ParseLink(shareLink)
		if err != nil {
			return ""
		}
		tag, _ := parsed.Outbound["tag"].(string)
		return strings.TrimSpace(tag)
	}
	hash := strings.LastIndex(shareLink, "#")
	if hash < 0 || hash+1 >= len(shareLink) {
		return ""
	}
	remark := strings.TrimSpace(shareLink[hash+1:])
	if decoded, err := url.QueryUnescape(remark); err == nil {
		remark = strings.TrimSpace(decoded)
	}
	return remark
}

// splitEmailIndex reads link_one's "<email> <index>". The index is cut from the
// end, so an email holding a space still names the right client.
func splitEmailIndex(arg string) (string, int, bool) {
	arg = strings.TrimSpace(arg)
	cut := strings.LastIndex(arg, " ")
	if cut <= 0 {
		return "", 0, false
	}
	index, err := strconv.Atoi(arg[cut+1:])
	email := strings.TrimSpace(arg[:cut])
	if err != nil || index < 0 || email == "" {
		return "", 0, false
	}
	return email, index, true
}

// individualLinks returns the client's share links, built in-process for the
// host the subscription URL names, never by fetching that URL.
func (t *Tgbot) individualLinks(email string) ([]string, error) {
	subURL, _, err := t.buildSubscriptionURLs(email)
	if err != nil {
		return nil, err
	}
	return t.clientSubLinks(email, subURL)
}

// screenOneLinkPicker asks which link before showing one, so "Get one" on a
// client attached to six inbounds is not the same wall of links as "All".
func (t *Tgbot) screenOneLinkPicker(chatID int64, email string) {
	links, err := t.individualLinks(email)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if len(links) == 0 {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	if len(links) == 1 {
		t.screenOneLink(chatID, email, 0)
		return
	}
	rows := make([][]telego.InlineKeyboardButton, 0, len(links)+1)
	for i, link := range links {
		rows = append(rows, tu.InlineKeyboardRow(
			t.btn2(linkLabel(link, i), t.encodeQuery(fmt.Sprintf("link_one %s %d", email, i)))))
	}
	rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.links", t.encodeQuery("client_sub_links "+email))))
	t.renderScreen(chatID, t.newScreen("links",
		t.clientHeader(email)+t.I18nBot("tgbot.messages.chooseOneConfig"), rows...))
}

// screenOneLink shows one link with its own QR as the picture. The list is read
// again rather than trusted from the button, so a stale tap cannot show a gone link.
func (t *Tgbot) screenOneLink(chatID int64, email string, index int) {
	links, err := t.individualLinks(email)
	if err != nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	if index < 0 || index >= len(links) {
		t.sendNotice(chatID, t.I18nBot("tgbot.noResult"))
		return
	}
	link := links[index]
	body := t.clientHeader(email) + html.EscapeString(linkLabel(link, index)) +
		":\r\n<code>" + html.EscapeString(link) + "</code>"
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			t.btn("tgbot.buttons.getOneConfig", t.encodeQuery("client_one_link "+email)),
			t.btn("tgbot.buttons.links", t.encodeQuery("client_sub_links "+email)),
		),
	}
	sc := t.newScreen("qr", body, rows...)
	sc.qrPicture = qrSheetPicture([]subscriptionQRCode{{content: link, kind: "link", app: index + 1}})
	sc.qrSource = link
	if sc.qrPicture == nil {
		t.sendNotice(chatID, t.I18nBot("tgbot.answers.errorOperation"))
		return
	}
	t.renderScreen(chatID, sc)
}
