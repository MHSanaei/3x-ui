package controller

import (
	"net/http"
	"net/netip"
	"text/template"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/telegramauth"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/tgbot"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

const telegramAuthError = "Telegram authentication failed"

var (
	telegramAuthStartLimiter      = newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
	telegramAuthCredentialLimiter = newLoginLimiter(5, 5*time.Minute, 15*time.Minute)
)

type telegramAuthCodeForm struct {
	Code string `json:"code" form:"code"`
}

type telegramAuthCredentialsForm struct {
	Password      string `json:"password" form:"password"`
	TwoFactorCode string `json:"twoFactorCode" form:"twoFactorCode"`
}

func (a *IndexController) telegramAuthStatus(c *gin.Context) {
	user, err := a.userService.GetFirstUser()
	available := err == nil && user.TelegramID != 0 && a.tgbot.IsRunning()
	jsonObj(c, gin.H{"available": available, "linked": err == nil && user.TelegramID != 0, "botUsername": a.tgbot.Username()}, nil)
}

func (a *IndexController) telegramAuthStart(c *gin.Context) {
	ip := getRemoteIp(c)
	rateIP := telegramAuthRateLimitIP(ip)
	if _, ok := telegramAuthStartLimiter.allow(rateIP, "telegram"); !ok {
		pureJsonMsg(c, http.StatusOK, false, "Too many login requests. Try again later.")
		return
	}
	if !a.tgbot.IsRunning() {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return
	}
	csrf, err := session.EnsureCSRFToken(c)
	if err != nil {
		pureJsonMsg(c, http.StatusInternalServerError, false, telegramAuthError)
		return
	}
	code, expiresAt, err := telegramauth.Default.StartLogin(csrf, ip)
	if err != nil {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return
	}
	telegramAuthStartLimiter.registerFailure(rateIP, "telegram")
	jsonObj(c, gin.H{"code": code, "expiresAt": expiresAt.UnixMilli()}, nil)
}

func (a *IndexController) telegramAuthComplete(c *gin.Context) {
	var form telegramAuthCodeForm
	if err := c.ShouldBind(&form); err != nil || form.Code == "" {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return
	}
	csrf, err := session.EnsureCSRFToken(c)
	if err != nil {
		pureJsonMsg(c, http.StatusInternalServerError, false, telegramAuthError)
		return
	}
	user, pending, err := telegramauth.Default.CompleteLogin(form.Code, csrf)
	if err != nil {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return
	}
	if pending {
		jsonObj(c, gin.H{"pending": true}, nil)
		return
	}
	if err := session.SetLoginUser(c, user); err != nil {
		logger.Warning("Unable to save Telegram login session:", err)
		pureJsonMsg(c, http.StatusInternalServerError, false, telegramAuthError)
		return
	}
	telegramAuthStartLimiter.registerSuccess(telegramAuthRateLimitIP(getRemoteIp(c)), "telegram")
	logger.Infof("Telegram login: username=%q, IP=%q", user.Username, getRemoteIp(c))
	a.tgbot.UserLoginNotify(tgbot.LoginAttempt{
		Username: template.HTMLEscapeString(user.Username),
		IP:       getRemoteIp(c),
		Time:     time.Now().Format("2006-01-02 15:04:05"),
		Status:   tgbot.LoginSuccess,
	})
	jsonObj(c, gin.H{"pending": false}, nil)
}

func telegramAuthRateLimitIP(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is6() {
		return netip.PrefixFrom(addr, 64).Masked().String()
	}
	return addr.String()
}

func (a *SettingController) telegramAuthSettingStatus(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		pureJsonMsg(c, http.StatusUnauthorized, false, telegramAuthError)
		return
	}
	jsonObj(c, gin.H{
		"available":      a.tgbot.IsRunning(),
		"linked":         user.TelegramID != 0,
		"telegramUserId": user.TelegramID,
		"botUsername":    a.tgbot.Username(),
	}, nil)
}

func (a *SettingController) telegramAuthCredentials(c *gin.Context) bool {
	var form telegramAuthCredentialsForm
	if err := c.ShouldBind(&form); err != nil || form.Password == "" {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return false
	}
	currentUser := session.GetLoginUser(c)
	if currentUser == nil {
		pureJsonMsg(c, http.StatusUnauthorized, false, telegramAuthError)
		return false
	}
	rateIP := telegramAuthRateLimitIP(getRemoteIp(c))
	if _, allowed := telegramAuthCredentialLimiter.allow(rateIP, currentUser.Username); !allowed {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return false
	}
	user, err := a.userService.CheckUser(currentUser.Username, form.Password, form.TwoFactorCode)
	if err != nil || user == nil || user.Id != currentUser.Id {
		telegramAuthCredentialLimiter.registerFailure(rateIP, currentUser.Username)
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return false
	}
	telegramAuthCredentialLimiter.registerSuccess(rateIP, currentUser.Username)
	return true
}

func (a *SettingController) telegramAuthLink(c *gin.Context) {
	if !a.telegramAuthCredentials(c) {
		return
	}
	if !a.tgbot.IsRunning() {
		pureJsonMsg(c, http.StatusOK, false, "Telegram bot is unavailable")
		return
	}
	user := session.GetLoginUser(c)
	code, expiresAt, err := telegramauth.Default.StartLink(user.Id)
	if err != nil {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return
	}
	jsonObj(c, gin.H{"code": code, "expiresAt": expiresAt.UnixMilli()}, nil)
}

func (a *SettingController) telegramAuthUnlink(c *gin.Context) {
	if !a.telegramAuthCredentials(c) {
		return
	}
	user := session.GetLoginUser(c)
	if err := telegramauth.Default.Unlink(user.Id); err != nil {
		pureJsonMsg(c, http.StatusOK, false, telegramAuthError)
		return
	}
	logger.Infof("Telegram login unlinked: username=%q", user.Username)
	jsonObj(c, gin.H{"linked": false}, nil)
}
