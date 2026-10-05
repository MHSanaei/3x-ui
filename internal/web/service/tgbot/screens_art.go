package tgbot

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Screen art: the bot's own avatar serves every screen's picture, and a panel
// without an avatar gets a generated black tile. A screen must never fail to
// render for want of a picture.

var (
	screenArtMu  sync.RWMutex
	screenArtIDs = map[string]string{}

	blackTileOnce  sync.Once
	blackTileBytes []byte
)

// resolveScreenArt caches the bot avatar's file_id for every screen kind.
// Telegram serves one photo per bot, so every kind shares it until the panel
// can host its own artwork.
func (t *Tgbot) resolveScreenArt() {
	if bot == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	me, err := bot.GetMe(ctx)
	if err != nil {
		logger.Warning("Failed to read the bot identity for screen art:", err)
		return
	}
	chat, err := bot.GetChat(ctx, &telego.GetChatParams{ChatID: tu.ID(me.ID)})
	if err != nil {
		logger.Debug("Failed to read the bot chat for screen art:", err)
		return
	}
	if chat.Photo == nil || chat.Photo.BigFileID == "" {
		logger.Debug("Bot has no avatar; screens will use the black fallback")
		return
	}
	screenArtMu.Lock()
	screenArtIDs = map[string]string{}
	for _, kind := range screenKinds {
		screenArtIDs[kind] = chat.Photo.BigFileID
	}
	screenArtMu.Unlock()
}

// artFileID returns the cached Telegram file_id for a screen kind.
func artFileID(kind string) (string, bool) {
	screenArtMu.RLock()
	defer screenArtMu.RUnlock()
	id, ok := screenArtIDs[kind]
	return id, ok && id != ""
}

// artUpload is the last-resort picture: the generated black tile.
func artUpload(kind string) (telego.InputFile, bool) {
	data := blackTile()
	return tu.FileFromBytes(data, "screen.png"), data != nil
}

// blackTile builds the 512x512 black tile once.
func blackTile() []byte {
	blackTileOnce.Do(func() {
		img := image.NewRGBA(image.Rect(0, 0, 512, 512))
		draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 13, G: 15, B: 20, A: 255}}, image.Point{}, draw.Src)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			logger.Warning("Failed to encode the fallback screen art:", err)
			return
		}
		blackTileBytes = buf.Bytes()
	})
	return blackTileBytes
}

// cacheArtFileID remembers a file_id Telegram just gave us, so a screen that
// had to upload its art edits in place afterwards.
func cacheArtFileID(kind, fileID string) {
	if kind == "" || fileID == "" {
		return
	}
	screenArtMu.Lock()
	screenArtIDs[kind] = fileID
	screenArtMu.Unlock()
}

// resetScreenArt drops the cached file_ids; tests use it between mock servers.
func resetScreenArt() {
	screenArtMu.Lock()
	screenArtIDs = map[string]string{}
	screenArtMu.Unlock()
}

// screenKinds is the catalogue of screen pictures. Until the panel can host its
// own artwork every kind resolves to the same avatar, but the kinds stay named
// so a screen's intent is visible in the code.
var screenKinds = []string{
	"main", "status", "inbounds", "clients", "client", "links", "qr",
	"wizard", "reports", "backup", "onlines", "deplete", "commands", "broadcast",
}
