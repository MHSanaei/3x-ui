package controller

import (
	"net/http"
	"time"
	"x-ui/logger"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type LoginForm struct {
	Username    string `json:"username" form:"username"`
	Password    string `json:"password" form:"password"`
	LoginSecret string `json:"loginSecret" form:"loginSecret"`
}

type IndexController struct {
	BaseController

	settingService service.SettingService
	userService    service.UserService
	tgbot          service.Tgbot
}

func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
	g.POST("/login", a.login)
	g.GET("/logout", a.logout)
	g.POST("/getSecretStatus", a.getSecretStatus)
}

func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Redirect(http.StatusTemporaryRedirect, "panel/")
		return
	}
	html(c, "login.html", "pages.login.title", nil)
}

func (a *IndexController) login(c *gin.Context) {
	var form LoginForm
	err := c.ShouldBind(&form)
	if err != nil {
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return
	}

	user := a.userService.CheckUser(form.Username, form.Password, form.LoginSecret)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	if user == nil {
		logger.Warningf("wrong username or password: \"%s\" \"%s\"", form.Username, form.Password)
		a.tgbot.UserLoginNotify(form.Username, getRemoteIp(c), timeStr, 0)
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	} else {
		logger.Infof("%s login success, Ip Address: %s\n", form.Username, getRemoteIp(c))
		a.tgbot.UserLoginNotify(form.Username, getRemoteIp(c), timeStr, 1)
	}

	sessionMaxAge, err := a.settingService.GetSessionMaxAge()
	if err != nil {
		logger.Warningf("Unable to get session's max age from DB")
	}

	if sessionMaxAge > 0 {
		err = session.SetMaxAge(c, sessionMaxAge*60)
		if err != nil {
			logger.Warningf("Unable to set session's max age")
		}
	}

	err = session.SetLoginUser(c, user)
	logger.Info("user", user.Id, "login success")
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), err)
}

func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Info("user", user.Id, "logout")
	}
	session.ClearSession(c)
	c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
}

func (a *IndexController) getSecretStatus(c *gin.Context) {
	status, err := a.settingService.GetSecretStatus()
	if err == nil {
		jsonObj(c, status, nil)
	}
}
