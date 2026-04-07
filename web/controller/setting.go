package controller

import (
	"errors"
	"net/http"
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
	databaseService service.DatabaseService
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

	database := g.Group("/database")
	database.POST("/get", a.getDatabaseSetting)
	database.POST("/test", a.testDatabaseSetting)
	database.POST("/install-postgres", a.installLocalPostgres)
	database.POST("/switch", a.switchDatabase)
	database.POST("/export", a.exportDatabase)
	database.GET("/export", a.exportDatabase)
	database.POST("/import", a.importDatabase)
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

func (a *SettingController) getDatabaseSetting(c *gin.Context) {
	setting, err := a.databaseService.GetSetting()
	if err != nil {
		jsonMsg(c, "Failed to load database settings", err)
		return
	}
	jsonObj(c, setting, nil)
}

func (a *SettingController) testDatabaseSetting(c *gin.Context) {
	setting := &entity.DatabaseSetting{}
	if err := c.ShouldBind(setting); err != nil {
		jsonMsg(c, "Failed to parse database settings", err)
		return
	}
	if err := a.databaseService.TestSetting(setting); err != nil {
		jsonMsg(c, "Connection test failed", err)
		return
	}
	jsonMsg(c, "Database connection test succeeded", nil)
}

func (a *SettingController) installLocalPostgres(c *gin.Context) {
	output, err := a.databaseService.InstallLocalPostgres()
	if err != nil {
		jsonMsg(c, "Failed to install PostgreSQL", err)
		return
	}
	jsonObj(c, output, nil)
}

func (a *SettingController) switchDatabase(c *gin.Context) {
	setting := &entity.DatabaseSetting{}
	if err := c.ShouldBind(setting); err != nil {
		jsonMsg(c, "Failed to parse database settings", err)
		return
	}
	err := a.databaseService.SwitchDatabase(setting)
	if err == nil {
		err = a.panelService.RestartPanel(3 * time.Second)
	}
	jsonMsg(c, "Database switch scheduled", err)
}

func (a *SettingController) exportDatabase(c *gin.Context) {
	exportType := c.Query("type")
	if exportType == "" {
		exportType = c.PostForm("type")
	}
	if exportType == "" {
		exportType = "portable"
	}

	var (
		data     []byte
		filename string
		err      error
	)

	switch exportType {
	case "sqlite":
		data, filename, err = a.databaseService.ExportNativeSQLite()
	default:
		data, filename, err = a.databaseService.ExportPortableBackup()
	}
	if err != nil {
		jsonMsg(c, "Failed to export database", err)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (a *SettingController) importDatabase(c *gin.Context) {
	file, _, err := c.Request.FormFile("backup")
	if err != nil {
		jsonMsg(c, "Failed to read uploaded backup", err)
		return
	}
	defer file.Close()

	backupType, err := a.databaseService.ImportBackup(file)
	if err == nil {
		err = a.panelService.RestartPanel(3 * time.Second)
	}
	if err != nil {
		jsonMsg(c, "Failed to import database backup", err)
		return
	}
	jsonObj(c, backupType, nil)
}
