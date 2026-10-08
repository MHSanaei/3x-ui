package tgbot

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Screen art: the bot's avatar serves every screen's picture, and a panel without
// one gets a generated black tile, so a screen never fails for want of a picture.

var (
	screenArtMu  sync.RWMutex
	screenArtIDs = map[string]string{}
	// artOverrides is artwork the panel set for one screen kind. It outranks
	// the bot's avatar, so adopting an avatar never silently replaces it.
	artOverrides = map[string]string{}
	// artFromAvatar records whether the cached ids came from the bot's avatar: a
	// tile cached from an upload must never win over an avatar set afterwards.
	artFromAvatar bool
	artProbedAt   time.Time
	// artProbeRunning keeps a burst of renders from starting a burst of probes.
	artProbeRunning atomic.Bool

	blackTileOnce  sync.Once
	blackTileBytes []byte
)

// Probes are cheap but not free: a known avatar is trusted for a while, a missing
// one retried sooner, so an uploaded picture shows without a restart.
const (
	artAvatarTTL = 10 * time.Minute
	artRetryTTL  = time.Minute
)

// resolveScreenArt caches the bot avatar's file_id for every screen kind.
// Telegram serves one photo per bot, so all kinds share it.
func (t *Tgbot) resolveScreenArt() {
	// One read: the panel can swap the bot under us (login/logout), and reading
	// the global per call is a nil-pointer crash in this background goroutine.
	b := liveBot()
	if b == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	me, err := b.GetMe(ctx)
	if err != nil {
		logger.Warning("Failed to read the bot identity for screen art:", err)
		return
	}

	// Use getUserProfilePhotos, not getChat: getChat's ChatPhoto file_id is
	// refused in sendPhoto ("can't use file of type ChatPhoto as Photo").
	photos, err := b.GetUserProfilePhotos(ctx, &telego.GetUserProfilePhotosParams{
		UserID: me.ID, Limit: 1,
	})
	if err != nil {
		logger.Debug("Failed to read the bot's profile photos for screen art:", err)
		return
	}

	screenArtMu.Lock()
	defer screenArtMu.Unlock()
	artProbedAt = time.Now()
	if len(photos.Photos) == 0 || len(photos.Photos[0]) == 0 {
		// Remember the miss so the next render does not probe again, but keep
		// any tile already cached: an avatar-less bot still shows a picture.
		artFromAvatar = false
		logger.Debug("Bot has no avatar; screens will use the black fallback")
		return
	}
	// The last entry is the largest size Telegram keeps.
	sizes := photos.Photos[0]
	fileID := sizes[len(sizes)-1].FileID
	screenArtIDs = map[string]string{}
	for _, kind := range screenKinds {
		screenArtIDs[kind] = fileID
	}
	artFromAvatar = true
}

// ensureScreenArt refreshes stale cached art off the render path, at most one
// probe in flight, so a just-uploaded picture shows without a restart.
func (t *Tgbot) ensureScreenArt() {
	screenArtMu.RLock()
	fromAvatar, at := artFromAvatar, artProbedAt
	screenArtMu.RUnlock()

	ttl := artRetryTTL
	if fromAvatar {
		ttl = artAvatarTTL
	}
	if time.Since(at) <= ttl {
		return
	}
	if !artProbeRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer artProbeRunning.Store(false)
		t.resolveScreenArt()
	}()
}

// artFileID returns the cached file_id for a screen kind. A fallback tile cached
// for one kind must not answer for another, so the per-kind entry is authoritative.
func artFileID(kind string) (string, bool) {
	screenArtMu.RLock()
	defer screenArtMu.RUnlock()
	if id, ok := artOverrides[kind]; ok && id != "" {
		return id, true
	}
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

// cacheArtFileID remembers the file_id Telegram returned for an uploaded tile,
// so that screen edits in place. It does NOT mark the art as the avatar.
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
	artOverrides = map[string]string{}
	artFromAvatar = false
	artProbedAt = time.Time{}
	screenArtMu.Unlock()
}

// screenKinds is the catalogue of screen pictures: every kind resolves to the
// same avatar, but the names keep each screen's intent visible in the code.
var screenKinds = []string{
	"main", "status", "inbounds", "clients", "client", "links", "qr",
	"wizard", "reports", "backup", "onlines", "deplete", "commands", "broadcast",
}
