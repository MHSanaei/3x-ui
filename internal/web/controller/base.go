// Package controller provides HTTP request handlers and controllers for the 3x-ui web management panel.
// It handles routing, authentication, and API endpoints for managing Xray inbounds, settings, and more.
package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// BaseController provides common functionality for all controllers, including authentication checks.
type BaseController struct{}

// checkLogin is a middleware that verifies user authentication and handles unauthorized access.
func (a *BaseController) checkLogin(c *gin.Context) {
	if session.IsLogin(c) {
		c.Next()
		return
	}
	// A self-service (user-tier) session has no panel User, so it fails IsLogin;
	// send it to its cabinet instead of the admin login page.
	if session.GetLoginClientSubID(c) != "" {
		if isAjax(c) {
			pureJsonMsg(c, http.StatusForbidden, false, I18nWeb(c, "pages.login.loginAgain"))
		} else {
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path")+"cabinet/")
		}
		c.Abort()
		return
	}
	if isAjax(c) {
		pureJsonMsg(c, http.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
	} else {
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
	}
	c.Abort()
}

// I18nWeb retrieves an internationalized message for the web interface based on the current locale.
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
