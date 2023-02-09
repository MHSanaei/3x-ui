package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"x-ui/web/session"
)

type BaseController struct {
}

func (a *BaseController) checkLogin(c *gin.Context) {
	if !session.IsLogin(c) {
		if isAjax(c) {
			pureJsonMsg(c, false, I18n(c , "pages.login.loginAgain"))
		} else {
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		}
		c.Abort()
	} else {
		c.Next()
	}
}


func I18n(c *gin.Context , name string) string{
    anyfunc, _  := c.Get("I18n")
    i18n, _ := anyfunc.(func(key string, params ...string) (string, error))

    message, _ := i18n(name)

    return message;
}
