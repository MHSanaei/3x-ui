package controller

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service/wechat"
)

var (
	wechatService    *wechat.Service
	reloadWechatFunc func()
)

func SetWechatService(s *wechat.Service) { wechatService = s }
func SetReloadWechatFunc(fn func())      { reloadWechatFunc = fn }

func (a *SettingController) testWechat(c *gin.Context) {
	if wechatService == nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.wechatTestFailed"), errors.New("WeChat service is unavailable"))
		return
	}
	enabled, err := a.settingService.GetWechatEnable()
	if err != nil || !enabled {
		jsonMsg(c, I18nWeb(c, "pages.settings.wechatTestFailed"), errors.New("WeChat notifications are disabled"))
		return
	}
	if err := wechatService.SendTest(); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.wechatTestFailed"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.wechatTestSuccess"), nil)
}
