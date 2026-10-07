package tgbot

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// The QR screen lays every code (subscription, JSON, up to five app links) on
// ONE picture instead of one document each — up to seven files before.

// subscriptionQRCode is one code on the sheet, in the order it is drawn.
type subscriptionQRCode struct {
	content string
	// kind selects the legend line: "sub", "json" or "link".
	kind string
	// app numbers an app link in the legend.
	app int
}

// subscriptionQRCodes collects every code for a client in the order Telegram
// used to deliver them: subscription, JSON subscription, then the app links.
func (t *Tgbot) subscriptionQRCodes(email, subURL string) []subscriptionQRCode {
	if subURL == "" {
		return nil
	}
	codes := []subscriptionQRCode{{content: subURL, kind: "sub"}}
	if _, subJSON, err := t.buildSubscriptionURLs(email); err == nil && subJSON != "" {
		codes = append(codes, subscriptionQRCode{content: subJSON, kind: "json"})
	}
	if links, err := t.clientSubLinks(email, subURL); err == nil {
		for i, link := range links {
			if i >= qrSheetMaxLinks {
				break
			}
			codes = append(codes, subscriptionQRCode{content: link, kind: "link", app: i + 1})
		}
	}
	if len(codes) > qrSheetMaxCodes {
		codes = codes[:qrSheetMaxCodes]
	}
	return codes
}

const (
	qrTileSize     = 320
	qrSheetPad     = 24
	qrSheetMaxCols = 3
	// The sheet stays inside Telegram's photo size limits while covering what
	// the original sent as separate documents.
	qrSheetMaxCodes = 6
	qrSheetMaxLinks = 4
)

// qrCodeLegend numbers the codes in the picture's own order, so a code can be
// told apart without reading it: the tiles carry no text, they are scanned.
func (t *Tgbot) qrCodeLegend(codes []subscriptionQRCode) string {
	if len(codes) == 0 {
		return ""
	}
	var body bytes.Buffer
	body.WriteString("\r\n")
	body.WriteString(t.I18nBot("tgbot.messages.qrLegendHeader"))
	body.WriteString("\r\n")
	for i, code := range codes {
		number := strconv.Itoa(i + 1)
		switch code.kind {
		case "json":
			body.WriteString(t.I18nBot("tgbot.messages.qrLegendJSON", "Code=="+number))
		case "link":
			body.WriteString(t.I18nBot("tgbot.messages.qrLegendLink",
				"Code=="+number, "App=="+strconv.Itoa(code.app)))
		default:
			body.WriteString(t.I18nBot("tgbot.messages.qrLegendSubscription", "Code=="+number))
		}
		body.WriteString("\r\n")
	}
	return body.String()
}

// qrSheetPicture draws every code onto one picture in the legend's order and
// returns it as an uploadable file; nil means nothing could be rendered.
func qrSheetPicture(codes []subscriptionQRCode) *telego.InputFile {
	tiles := make([]image.Image, 0, len(codes))
	for _, code := range codes {
		codePNG, err := qrPNG(code.content, qrTileSize)
		if err != nil {
			logger.Warning("Failed to render a subscription QR:", err)
			continue
		}
		img, err := png.Decode(bytes.NewReader(codePNG))
		if err != nil {
			logger.Warning("Failed to decode a subscription QR:", err)
			continue
		}
		tiles = append(tiles, img)
	}
	if len(tiles) == 0 {
		return nil
	}

	cols := len(tiles)
	if cols > qrSheetMaxCols {
		cols = qrSheetMaxCols
	}
	rows := (len(tiles) + cols - 1) / cols
	width := cols*qrTileSize + (cols+1)*qrSheetPad
	height := rows*qrTileSize + (rows+1)*qrSheetPad

	// The codes must keep their light background to stay scannable, so the sheet
	// is white rather than the dark tone the screens use.
	sheet := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	for i, tile := range tiles {
		x := qrSheetPad + (i%cols)*(qrTileSize+qrSheetPad)
		y := qrSheetPad + (i/cols)*(qrTileSize+qrSheetPad)
		draw.Draw(sheet, image.Rect(x, y, x+qrTileSize, y+qrTileSize), tile, tile.Bounds().Min, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		logger.Warning("Failed to encode the QR sheet:", err)
		return nil
	}
	file := tu.FileFromBytes(buf.Bytes(), "qr.png")
	return &file
}
