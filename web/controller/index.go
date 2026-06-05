package controller

import (
	"errors"
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

// RegisterForm represents the self-registration request structure.
type RegisterForm struct {
	FullName        string `json:"fullName" form:"fullName"`
	Phone           string `json:"phone" form:"phone"`
	Email           string `json:"email" form:"email"`
	Username        string `json:"username" form:"username"`
	Password        string `json:"password" form:"password"`
	ConfirmPassword string `json:"confirmPassword" form:"confirmPassword"`
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
	g.GET("/register", a.registerPage)
	g.GET("/csrf-token", a.csrfToken)

	g.POST("/login", middleware.CSRFMiddleware(), a.login)
	g.POST("/register", middleware.CSRFMiddleware(), a.register)
	g.POST("/logout", middleware.CSRFMiddleware(), a.logout)
	g.POST("/getTwoFactorEnable", middleware.CSRFMiddleware(), a.getTwoFactorEnable)
	g.POST("/getRegistrationEnable", middleware.CSRFMiddleware(), a.getRegistrationEnable)
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

// registerPage serves the self-registration SPA shell. Logged-in users are sent
// to the panel; when registration is disabled the page is not exposed and the
// visitor is redirected to the login screen.
func (a *IndexController) registerPage(c *gin.Context) {
	if session.IsLogin(c) {
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path")+"panel/")
		return
	}
	if enabled, err := a.settingService.GetRegistrationEnable(); err != nil || !enabled {
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
		return
	}
	serveDistPage(c, "register.html")
}

// getRegistrationEnable reports whether public self-registration is enabled so
// the login/registration pages can show or hide the relevant controls.
func (a *IndexController) getRegistrationEnable(c *gin.Context) {
	status, err := a.settingService.GetRegistrationEnable()
	if err == nil {
		jsonObj(c, status, nil)
	}
}

// register creates a new panel user from the self-registration form. It is
// gated behind the registrationEnable setting, rate limited per client IP, and
// validates/normalizes every field before delegating uniqueness and hashing to
// the user service.
func (a *IndexController) register(c *gin.Context) {
	enabled, err := a.settingService.GetRegistrationEnable()
	if err != nil || !enabled {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.register.toasts.disabled"))
		return
	}

	remoteIP := getRemoteIp(c)
	if _, ok := defaultRegisterLimiter.allow(remoteIP, registrationLimitBucket); !ok {
		logger.Warningf("registration throttled: IP=%q", remoteIP)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.register.toasts.tooManyAttempts"))
		return
	}

	var form RegisterForm
	if err := c.ShouldBind(&form); err != nil {
		defaultRegisterLimiter.registerFailure(remoteIP, registrationLimitBucket)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.register.toasts.invalidFormData"))
		return
	}

	if form.Password != form.ConfirmPassword {
		defaultRegisterLimiter.registerFailure(remoteIP, registrationLimitBucket)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.register.toasts.passwordMismatch"))
		return
	}

	user, regErr := a.userService.Register(service.RegisterInput{
		FullName: form.FullName,
		Phone:    form.Phone,
		Email:    form.Email,
		Username: form.Username,
		Password: form.Password,
	})
	if regErr != nil {
		defaultRegisterLimiter.registerFailure(remoteIP, registrationLimitBucket)
		pureJsonMsg(c, http.StatusOK, false, registerFailureMessage(c, regErr))
		return
	}

	defaultRegisterLimiter.registerSuccess(remoteIP, registrationLimitBucket)
	logger.Infof("new user %q registered, Ip Address: %s", template.HTMLEscapeString(user.Username), remoteIP)
	jsonMsg(c, I18nWeb(c, "pages.register.toasts.success"), nil)
}

// registerFailureMessage maps a registration service error to a localized,
// non-leaky message. Unknown errors fall back to a generic failure string.
func registerFailureMessage(c *gin.Context, err error) string {
	switch {
	case errors.Is(err, service.ErrUsernameTaken):
		return I18nWeb(c, "pages.register.toasts.usernameTaken")
	case errors.Is(err, service.ErrEmailTaken):
		return I18nWeb(c, "pages.register.toasts.emailTaken")
	case errors.Is(err, service.ErrInvalidUsername):
		return I18nWeb(c, "pages.register.toasts.invalidUsername")
	case errors.Is(err, service.ErrInvalidEmail):
		return I18nWeb(c, "pages.register.toasts.invalidEmail")
	case errors.Is(err, service.ErrInvalidPhone):
		return I18nWeb(c, "pages.register.toasts.invalidPhone")
	case errors.Is(err, service.ErrInvalidFullName):
		return I18nWeb(c, "pages.register.toasts.invalidFullName")
	case errors.Is(err, service.ErrWeakPassword):
		return I18nWeb(c, "pages.register.toasts.weakPassword")
	default:
		logger.Warning("registration failed:", err)
		return I18nWeb(c, "pages.register.toasts.failed")
	}
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
