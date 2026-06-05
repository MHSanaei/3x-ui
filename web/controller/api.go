package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// APIController handles the main API routes for the 3x-ui panel, including inbounds and server management.
type APIController struct {
	BaseController
	inboundController *InboundController
	serverController  *ServerController
	nodeController    *NodeController
	adminController   *AdminController
	settingService    service.SettingService
	userService       service.UserService
	walletService     service.WalletService
	apiTokenService   service.ApiTokenService
	Tgbot             service.Tgbot
}

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup, customGeo *service.CustomGeoService) *APIController {
	a := &APIController{}
	a.initRouter(g, customGeo)
	return a
}

func (a *APIController) checkAPIAuth(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		tok := after
		if a.apiTokenService.Match(tok) {
			if u, err := a.userService.GetFirstUser(); err == nil {
				session.SetAPIAuthUser(c, u)
			}
			c.Set("api_authed", true)
			c.Next()
			return
		}
	}
	if !session.IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			c.AbortWithStatus(http.StatusUnauthorized)
		} else {
			c.AbortWithStatus(http.StatusNotFound)
		}
		return
	}
	c.Next()
}

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup, customGeo *service.CustomGeoService) {
	// Main API group
	api := g.Group("/panel/api")
	api.Use(a.checkAPIAuth)
	api.Use(middleware.CSRFMiddleware())

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)

	clients := api.Group("/clients")
	NewClientController(clients)
	// Group (client tag) management is an admin-only feature; gate it so a
	// limited user cannot enumerate or mutate groups across other users.
	groupAdmin := clients.Group("")
	groupAdmin.Use(middleware.RequireAdmin())
	NewGroupController(groupAdmin)

	// Server API (system status/config) — admin only.
	server := api.Group("/server")
	server.Use(middleware.RequireAdmin())
	a.serverController = NewServerController(server)

	// Nodes API — multi-panel management; admin only.
	nodes := api.Group("/nodes")
	nodes.Use(middleware.RequireAdmin())
	a.nodeController = NewNodeController(nodes)

	// Custom geo asset management — admin only.
	customGeoGroup := api.Group("/custom-geo")
	customGeoGroup.Use(middleware.RequireAdmin())
	NewCustomGeoController(customGeoGroup, customGeo)

	// RBAC + wallet administration (admin-only; gated inside the controller).
	a.adminController = NewAdminController(api)

	// Identity + wallet snapshot for the current session (any logged-in user).
	api.GET("/me", a.me)
	// Self-service profile editing (any logged-in user; never admin-gated).
	api.POST("/profile", a.updateProfile)
	// Balance top-up via ZarinPal (any logged-in user).
	NewPaymentController(api)

	// Extra routes
	api.POST("/backuptotgbot", middleware.RequireAdmin(), a.BackuptoTgbot)
}

// me returns the current user's identity, role, balance and the per-client
// cost so the SPA can gate navigation, show the wallet and preview purchases.
func (a *APIController) me(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	balance, _ := a.walletService.GetBalance(user.Id)
	cost, _ := a.settingService.GetClientCost()
	costPerGB, _ := a.settingService.GetClientCostPerGB()
	zarinpalEnable, _ := a.settingService.GetZarinpalEnable()
	currency, _ := a.settingService.GetZarinpalCurrency()
	jsonObj(c, gin.H{
		"id":              user.Id,
		"username":        user.Username,
		"email":           user.Email,
		"role":            user.Role,
		"isAdmin":         user.IsAdmin(),
		"balance":         balance,
		"clientCost":      cost,
		"clientCostPerGB": costPerGB,
		"zarinpalEnable":  zarinpalEnable,
		"currency":        currency,
	}, nil)
}

type profileForm struct {
	CurrentPassword string `json:"currentPassword"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	NewPassword     string `json:"newPassword"`
}

// updateProfile lets the current user change their own username, email and
// password after confirming their current password. A password change bumps
// the login epoch (invalidating this session), so the response flags
// passwordChanged and the client redirects to login.
func (a *APIController) updateProfile(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var form profileForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	_, passwordChanged, err := a.userService.UpdateSelfProfile(user.Id, service.SelfProfileInput{
		CurrentPassword: form.CurrentPassword,
		Username:        form.Username,
		Email:           form.Email,
		NewPassword:     form.NewPassword,
	})
	if err != nil {
		if errors.Is(err, service.ErrWrongPassword) {
			pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.profile.toasts.wrongPassword"))
			return
		}
		if msg := adminUserErrorMessage(c, err); msg != "" {
			pureJsonMsg(c, http.StatusOK, false, msg)
			return
		}
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"passwordChanged": passwordChanged}, nil)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
