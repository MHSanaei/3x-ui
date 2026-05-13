package controller

import (
	"net/http"
	"text/template"
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// LoginForm represents the login request structure.
type LoginForm struct {
	Username      string `json:"username" form:"username"`
	Password      string `json:"password" form:"password"`
	TwoFactorCode string `json:"twoFactorCode" form:"twoFactorCode"`
}

// IndexController handles the main index and login-related routes.
type IndexController struct {
	BaseController

	settingService service.SettingService
	userService    service.UserService
	tgbot          service.Tgbot
}

// NewIndexController creates a new IndexController and initializes its routes.
func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for index, login, logout, and two-factor authentication.
func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
	g.GET("/csrf-token", a.csrfToken)

	g.POST("/login", middleware.CSRFMiddleware(), a.login)
	g.POST("/logout", middleware.CSRFMiddleware(), a.logout)
	g.POST("/getTwoFactorEnable", middleware.CSRFMiddleware(), a.getTwoFactorEnable)
}

// index handles the root route, redirecting logged-in users to the panel or showing the login page.
func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path")+"panel/")
		return
	}
	serveDistPage(c, "login.html")
}

// login handles user authentication and session creation.
func (a *IndexController) login(c *gin.Context) {
	var form LoginForm

	if err := c.ShouldBind(&form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return
	}

	remoteIP := getRemoteIp(c)
	safeUser := template.HTMLEscapeString(form.Username)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	if blockedUntil, ok := defaultLoginLimiter.allow(remoteIP, form.Username); !ok {
		reason := "too many failed attempts"
		logger.Warningf("failed login: username=%q, IP=%q, reason=%q, blocked_until=%s", safeUser, remoteIP, reason, blockedUntil.Format(time.RFC3339))
		a.tgbot.UserLoginNotify(service.LoginAttempt{
			Username: safeUser,
			IP:       remoteIP,
			Time:     timeStr,
			Status:   service.LoginFail,
			Reason:   reason,
		})
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	}

	user, checkErr := a.userService.CheckUser(form.Username, form.Password, form.TwoFactorCode)

	if user == nil {
		reason := loginFailureReason(checkErr)
		if blockedUntil, blocked := defaultLoginLimiter.registerFailure(remoteIP, form.Username); blocked {
			logger.Warningf("failed login: username=%q, IP=%q, reason=%q, blocked_until=%s", safeUser, remoteIP, reason, blockedUntil.Format(time.RFC3339))
		} else {
			logger.Warningf("failed login: username=%q, IP=%q, reason=%q", safeUser, remoteIP, reason)
		}
		a.tgbot.UserLoginNotify(service.LoginAttempt{
			Username: safeUser,
			IP:       remoteIP,
			Time:     timeStr,
			Status:   service.LoginFail,
			Reason:   reason,
		})
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	}

	defaultLoginLimiter.registerSuccess(remoteIP, form.Username)
	logger.Infof("%s logged in successfully, Ip Address: %s\n", safeUser, remoteIP)
	a.tgbot.UserLoginNotify(service.LoginAttempt{
		Username: safeUser,
		IP:       remoteIP,
		Time:     timeStr,
		Status:   service.LoginSuccess,
	})

	if err := session.SetLoginUser(c, user); err != nil {
		logger.Warning("Unable to save session:", err)
		return
	}

	logger.Infof("%s logged in successfully", safeUser)
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), nil)
}

func loginFailureReason(err error) string {
	if err != nil && err.Error() == "invalid 2fa code" {
		return "invalid 2FA code"
	}
	return "invalid credentials"
}

func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Infof("%s logged out successfully", user.Username)
	}
	if err := session.ClearSession(c); err != nil {
		logger.Warning("Unable to clear session on logout:", err)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// csrfToken returns the session CSRF token. Public — the login page
// needs a token before authenticating.
func (a *IndexController) csrfToken(c *gin.Context) {
	token, err := session.EnsureCSRFToken(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": token})
}

// getTwoFactorEnable retrieves the current status of two-factor authentication.
func (a *IndexController) getTwoFactorEnable(c *gin.Context) {
	status, err := a.settingService.GetTwoFactorEnable()
	if err == nil {
		jsonObj(c, status, nil)
	}
}
