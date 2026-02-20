package service

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/skip2/go-qrcode"
)

// buildSubscriptionURLs builds the HTML sub page URL and JSON subscription URL for a client email.
func (t *Tgbot) buildSubscriptionURLs(email string) (string, string, error) {
	// Resolve subId from client email
	traffic, client, err := t.inboundService.GetClientByEmail(email)
	_ = traffic
	if err != nil || client == nil {
		return "", "", errors.New("client not found")
	}

	// Gather settings to construct absolute URLs
	subURI, _ := t.settingService.GetSubURI()
	subJsonURI, _ := t.settingService.GetSubJsonURI()
	subDomain, _ := t.settingService.GetSubDomain()
	subPort, _ := t.settingService.GetSubPort()
	subPath, _ := t.settingService.GetSubPath()
	subJsonPath, _ := t.settingService.GetSubJsonPath()
	subJsonEnable, _ := t.settingService.GetSubJsonEnable()
	subKeyFile, _ := t.settingService.GetSubKeyFile()
	subCertFile, _ := t.settingService.GetSubCertFile()

	// Fallbacks
	if subDomain == "" {
		// try panel domain, otherwise OS hostname
		if d, err := t.settingService.GetWebDomain(); err == nil && d != "" {
			subDomain = d
		} else if hostname != "" {
			subDomain = hostname
		} else {
			subDomain = "localhost"
		}
	}

	return BuildSubscriptionURLs(SubscriptionURLInput{
		SubID: client.SubID,

		ConfiguredSubURI:     subURI,
		ConfiguredSubJSONURI: subJsonURI,

		SubDomain:   subDomain,
		SubPort:     subPort,
		SubPath:     subPath,
		SubJSONPath: subJsonPath,

		SubKeyFile:  subKeyFile,
		SubCertFile: subCertFile,

		JSONEnabled: subJsonEnable,
	})
}

// sendClientSubLinks sends the subscription links for the client to the chat.
func (t *Tgbot) sendClientSubLinks(chatId int64, email string) {
	subURL, subJsonURL, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	msg := "Subscription URL:\r\n<code>" + subURL + "</code>"
	if subJsonURL != "" {
		msg += "\r\n\r\nJSON URL:\r\n<code>" + subJsonURL + "</code>"
	}
	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("subscription.individualLinks")).WithCallbackData(t.encodeQuery("client_individual_links "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("qrCode")).WithCallbackData(t.encodeQuery("client_qr_links "+email)),
		),
	)
	t.SendMsgToTgbot(chatId, msg, inlineKeyboard)
}

// sendClientIndividualLinks fetches subscription content (individual links) and sends it to the user.
func (t *Tgbot) sendClientIndividualLinks(chatId int64, email string) {
	// Build the HTML sub page URL; we'll call it with header Accept to get raw content
	subURL, _, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}

	// Try to fetch raw subscription links. Prefer plain text response.
	req, err := http.NewRequest("GET", subURL, nil)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	// Force plain text to avoid HTML page; controller respects Accept header
	req.Header.Set("Accept", "text/plain, */*;q=0.1")

	// Use optimized client with connection pooling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := optimizedHTTPClient.Do(req)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}

	// If service is configured to encode (Base64), decode it
	encoded, _ := t.settingService.GetSubEncrypt()
	var content string
	if encoded {
		decoded, err := base64.StdEncoding.DecodeString(string(bodyBytes))
		if err != nil {
			// fallback to raw text
			content = string(bodyBytes)
		} else {
			content = string(decoded)
		}
	} else {
		content = string(bodyBytes)
	}

	// Normalize line endings and trim
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var cleaned []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			cleaned = append(cleaned, l)
		}
	}
	if len(cleaned) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noResult"))
		return
	}

	// Send in chunks to respect message length; use monospace formatting
	const maxPerMessage = 50
	for i := 0; i < len(cleaned); i += maxPerMessage {
		j := i + maxPerMessage
		if j > len(cleaned) {
			j = len(cleaned)
		}
		chunk := cleaned[i:j]
		msg := t.I18nBot("subscription.individualLinks") + ":\r\n"
		for _, link := range chunk {
			// wrap each link in <code>
			msg += "<code>" + link + "</code>\r\n"
		}
		t.SendMsgToTgbot(chatId, msg)
	}
}

// sendClientQRLinks generates QR images for subscription URL, JSON URL, and individual links, then sends them.
func (t *Tgbot) sendClientQRLinks(chatId int64, email string) {
	subURL, subJsonURL, err := t.buildSubscriptionURLs(email)
	if err != nil {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		return
	}

	// Helper to create QR PNG bytes from content
	createQR := func(content string, size int) ([]byte, error) {
		if size <= 0 {
			size = 256
		}
		return qrcode.Encode(content, qrcode.Medium, size)
	}

	// Inform user
	t.SendMsgToTgbot(chatId, "QRCode"+":")

	// Send sub URL QR (filename: sub.png)
	if png, err := createQR(subURL, 320); err == nil {
		document := tu.Document(
			tu.ID(chatId),
			tu.FileFromBytes(png, "sub.png"),
		)
		_, _ = bot.SendDocument(context.Background(), document)
	} else {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
	}

	// Send JSON URL QR (filename: subjson.png) when available
	if subJsonURL != "" {
		if png, err := createQR(subJsonURL, 320); err == nil {
			document := tu.Document(
				tu.ID(chatId),
				tu.FileFromBytes(png, "subjson.png"),
			)
			_, _ = bot.SendDocument(context.Background(), document)
		} else {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation")+"\r\n"+err.Error())
		}
	}

	// Also generate a few individual links' QRs (first up to 5)
	subPageURL := subURL
	req, err := http.NewRequest("GET", subPageURL, nil)
	if err == nil {
		req.Header.Set("Accept", "text/plain, */*;q=0.1")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
		if resp, err := optimizedHTTPClient.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			encoded, _ := t.settingService.GetSubEncrypt()
			var content string
			if encoded {
				if dec, err := base64.StdEncoding.DecodeString(string(body)); err == nil {
					content = string(dec)
				} else {
					content = string(body)
				}
			} else {
				content = string(body)
			}
			lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
			var cleaned []string
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l != "" {
					cleaned = append(cleaned, l)
				}
			}
			if len(cleaned) > 0 {
				max := min(len(cleaned), 5)
				for i := range max {
					if png, err := createQR(cleaned[i], 320); err == nil {
						filename := email + ".png"
						document := tu.Document(
							tu.ID(chatId),
							tu.FileFromBytes(png, filename),
						)
						_, _ = bot.SendDocument(context.Background(), document)
						if i < max-1 {
							time.Sleep(50 * time.Millisecond)
						}
					}
				}
			}
		}
	}
}
