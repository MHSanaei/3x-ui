package controller

import (
	"fmt"
	"net/http"
    "strings"

	"x-ui/logger"
	"x-ui/web/locale"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
	"x-ui/web/service"
)

type BaseController struct{
	settingService service.SettingService
}

func (a *BaseController) checkLogin(c *gin.Context) {
	if !session.IsLogin(c) {
		if isAjax(c) {
			pureJsonMsg(c, http.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
		} else {
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		}
		c.Abort()
	} else {
		c.Next()
	}
}

func I18nWeb(c *gin.Context, name string, params ...string) string {
	anyfunc, funcExists := c.Get("I18n")
	if !funcExists {
		logger.Warning("I18n function not exists in gin context!")
		return ""
	}
	i18nFunc, _ := anyfunc.(func(i18nType locale.I18nType, key string, keyParams ...string) string)
	msg := i18nFunc(locale.Web, name, params...)
	return msg
}

func (a *BaseController) apiTokenGuard(c *gin.Context) {
	bearerToken := c.Request.Header.Get("Authorization")
	tokenParts := strings.Split(bearerToken, " ")
	if len(tokenParts) != 2 {
		pureJsonMsg(c, http.StatusUnauthorized, false, "Invalid token format")
		c.Abort()
		return
	}
	reqToken := tokenParts[1]
	token, err := a.settingService.GetApiToken()

	if err != nil {
		pureJsonMsg(c, http.StatusUnauthorized, false, err.Error())
		c.Abort()
		return
	}

	if reqToken != token {
		pureJsonMsg(c, http.StatusUnauthorized, false, "Auth failed")
		c.Abort()
		return
	}

	userService := service.UserService{}
	user, err := userService.GetFirstUser()
	if err != nil {
		fmt.Println("get current user info failed, error info:", err)
	}

	session.SetSessionUser(c, user)

	c.Next()

	session.ClearSession(c)
}