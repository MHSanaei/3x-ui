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
	// artFromAvatar records whether the cached ids came from the bot's own
	// avatar. A tile cached from an upload must never win over an avatar the
	// operator sets afterwards.
	artFromAvatar bool
	artProbedAt   time.Time

	blackTileOnce  sync.Once
	blackTileBytes []byte
)

// Probes are cheap but not free, so a known avatar is trusted for a while and
// a missing one is retried sooner: an operator who uploads a picture usually
// checks right after, and would otherwise see tiles until the next restart.
const (
	artAvatarTTL = 10 * time.Minute
	artRetryTTL  = time.Minute
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

	screenArtMu.Lock()
	defer screenArtMu.Unlock()
	artProbedAt = time.Now()
	if chat.Photo == nil || chat.Photo.BigFileID == "" {
		// Remember the miss so the next render does not probe again, but keep
		// any tile already cached: an avatar-less bot still shows a picture.
		artFromAvatar = false
		logger.Debug("Bot has no avatar; screens will use the black fallback")
		return
	}
	screenArtIDs = map[string]string{}
	for _, kind := range screenKinds {
		screenArtIDs[kind] = chat.Photo.BigFileID
	}
	artFromAvatar = true
}

// ensureScreenArt refreshes the cached artwork in the background when it is
// stale: a known avatar is re-read rarely, a missing one sooner, so a picture
// the operator uploads shows up without restarting the panel.
func (t *Tgbot) ensureScreenArt() {
	screenArtMu.RLock()
	fromAvatar, at := artFromAvatar, artProbedAt
	screenArtMu.RUnlock()

	ttl := artRetryTTL
	if fromAvatar {
		ttl = artAvatarTTL
	}
	if time.Since(at) > ttl {
		go t.resolveScreenArt()
	}
}

// artFileID returns the cached Telegram file_id for a screen kind. A fallback
// tile cached for one kind must not answer for another, so the avatar flag is
// checked by the caller: here the per-kind entry is authoritative.
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

// cacheArtFileID remembers a file_id Telegram just gave us for an uploaded
// tile, so a screen that had to upload its art edits in place afterwards. It
// does NOT mark the art as the avatar: a real avatar uploaded later has to be
// able to replace it.
func cacheArtFileID(kind, fileID string) {
	if kind == "" || fileID == "" {
		return
	}
	screenArtMu.Lock()
	if !artFromAvatar {
		screenArtIDs[kind] = fileID
	}
	screenArtMu.Unlock()
}

// resetScreenArt drops the cached file_ids; tests use it between mock servers.
func resetScreenArt() {
	screenArtMu.Lock()
	screenArtIDs = map[string]string{}
	artFromAvatar = false
	artProbedAt = time.Time{}
	screenArtMu.Unlock()
}

// screenKinds is the catalogue of screen pictures. Until the panel can host its
// own artwork every kind resolves to the same avatar, but the kinds stay named
// so a screen's intent is visible in the code.
var screenKinds = []string{
	"main", "status", "inbounds", "clients", "client", "links", "qr",
	"wizard", "reports", "backup", "onlines", "deplete", "commands", "broadcast",
}
