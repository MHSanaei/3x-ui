package controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// updateUserForm represents the form for updating user credentials.
type updateUserForm struct {
	OldUsername string `json:"oldUsername" form:"oldUsername"`
	OldPassword string `json:"oldPassword" form:"oldPassword"`
	NewUsername string `json:"newUsername" form:"newUsername"`
	NewPassword string `json:"newPassword" form:"newPassword"`
}

// SettingController handles settings and user management operations.
type SettingController struct {
	settingService service.SettingService
	userService    service.UserService
	panelService   service.PanelService
}

// NewSettingController creates a new SettingController and initializes its routes.
func NewSettingController(g *gin.RouterGroup) *SettingController {
	a := &SettingController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for settings management.
func (a *SettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/setting")

	g.POST("/all", a.getAllSetting)
	g.POST("/defaultSettings", a.getDefaultSettings)
	g.POST("/update", a.updateSetting)
	g.POST("/updateUser", a.updateUser)
	g.POST("/restartPanel", a.restartPanel)
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	g.POST("/ai/update", a.updateAISetting)
	g.GET("/ai/status", a.getAIStatus)
}

// getAllSetting retrieves all current settings.
func (a *SettingController) getAllSetting(c *gin.Context) {
	allSetting, err := a.settingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, allSetting, nil)
}

// getDefaultSettings retrieves the default settings based on the host.
func (a *SettingController) getDefaultSettings(c *gin.Context) {
	result, err := a.settingService.GetDefaultSettings(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, result, nil)
}

// updateSetting updates all settings with the provided data.
func (a *SettingController) updateSetting(c *gin.Context) {
	allSetting := &entity.AllSetting{}
	err := c.ShouldBind(allSetting)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	err = a.settingService.UpdateAllSetting(allSetting)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
}

// updateUser updates the current user's username and password.
func (a *SettingController) updateUser(c *gin.Context) {
	form := &updateUserForm{}
	err := c.ShouldBind(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user.Username != form.OldUsername || !crypto.CheckPasswordHash(user.Password, form.OldPassword) {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.originalUserPassIncorrect")))
		return
	}
	if form.NewUsername == "" || form.NewPassword == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUserError"), errors.New(I18nWeb(c, "pages.settings.toasts.userPassMustBeNotEmpty")))
		return
	}
	err = a.userService.UpdateUser(user.Id, form.NewUsername, form.NewPassword)
	if err == nil {
		user.Username = form.NewUsername
		user.Password, _ = crypto.HashPasswordAsBcrypt(form.NewPassword)
		session.SetLoginUser(c, user)
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
}

// restartPanel restarts the panel service after a delay.
func (a *SettingController) restartPanel(c *gin.Context) {
	err := a.panelService.RestartPanel(time.Second * 3)
	jsonMsg(c, I18nWeb(c, "pages.settings.restartPanelSuccess"), err)
}

// getDefaultXrayConfig retrieves the default Xray configuration.
func (a *SettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.settingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}

// updateAISetting updates AI configuration settings
func (a *SettingController) updateAISetting(c *gin.Context) {
	var req struct {
		Enabled     bool    `json:"enabled"`
		ApiKey      string  `json:"apiKey"`
		MaxTokens   int     `json:"maxTokens"`
		Temperature float64 `json:"temperature"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSettings"), err)
		return
	}

	// Update settings
	if err := a.settingService.SetAIEnabled(req.Enabled); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSettings"), err)
		return
	}

	if req.ApiKey != "" {
		if err := a.settingService.SetAIApiKey(req.ApiKey); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSettings"), err)
			return
		}
	}

	if req.MaxTokens > 0 {
		if err := a.settingService.SetAIMaxTokens(req.MaxTokens); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSettings"), err)
			return
		}
	}

	if req.Temperature > 0 {
		tempStr := fmt.Sprintf("%.1f", req.Temperature)
		if err := a.settingService.SetAISetting("aiTemperature", tempStr); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSettings"), err)
			return
		}
	}

	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.updateSettings"), nil)
}

// getAIStatus returns the current AI service status
func (a *SettingController) getAIStatus(c *gin.Context) {
	enabled, _ := a.settingService.GetAIEnabled()
	hasApiKey := false
	if apiKey, err := a.settingService.GetAIApiKey(); err == nil && apiKey != "" {
		hasApiKey = true
	}

	jsonObj(c, gin.H{
		"enabled":   enabled,
		"hasApiKey": hasApiKey,
		"ready":     enabled && hasApiKey,
	}, nil)
}

