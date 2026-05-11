// Package controller provides HTTP request handlers and controllers for the 3x-ui web management panel.
// It handles routing, authentication, and API endpoints for managing Xray inbounds, settings, and more.
package controller

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/web/locale"
	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// BaseController provides common functionality for all controllers, including authentication checks.
type BaseController struct{}

// checkLogin is a middleware that verifies user authentication and handles unauthorized access.
func (a *BaseController) checkLogin(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		if isAjax(c) {
			pureJsonMsg(c, http.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
		} else {
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		}
		c.Abort()
		return
	}
	if isDefaultAdminCredential(user.Username, user.Password) && !credentialChangeRouteAllowed(c) {
		if isAjax(c) {
			pureJsonMsg(c, http.StatusForbidden, false, "Change the default admin credentials before continuing.")
		} else {
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path")+"panel/settings")
		}
		c.Abort()
	} else {
		c.Next()
	}
}

func isDefaultAdminCredential(username string, hashedPassword string) bool {
	return username == "admin" && crypto.CheckPasswordHash(hashedPassword, "admin")
}

func credentialChangeRouteAllowed(c *gin.Context) bool {
	basePath := c.GetString("base_path")
	path := c.Request.URL.Path
	allowedPrefixes := []string{
		basePath + "panel/settings",
		basePath + "panel/setting/",
		basePath + "panel/csrf-token",
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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
