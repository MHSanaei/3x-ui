package tgbot

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// helpers.go holds the small shared primitives the screen layer needs, so the
// router never grows a second copy of them.

// xrayProcessOrNil returns the running Xray process, or nil; a panel whose Xray
// is down is a normal state the screens must render, not an error.
func xrayProcessOrNil() *xray.Process {
	process := service.XrayProcess()
	if process == nil || !process.IsRunning() {
		return nil
	}
	return process
}

// panelVersion is the version string the menu shows.
func panelVersion() string { return config.GetPanelVersion() }

// sendDocumentWithHide uploads one file as its own message with a hide button:
// a document cannot be the picture of a screen, so a file is a notice.
func (t *Tgbot) sendDocumentWithHide(chatID int64, data []byte, name string) {
	if bot == nil || data == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := bot.SendDocument(ctx, &telego.SendDocumentParams{
		ChatID:      tu.ID(chatID),
		Document:    tu.FileFromBytes(data, name),
		ReplyMarkup: t.hideButton(),
	})
	if err != nil {
		logger.Warning("Failed to upload the file:", err)
	}
}
