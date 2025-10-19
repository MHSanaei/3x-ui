package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// APIController handles the main API routes for the 3x-ui panel, including inbounds and server management.
type APIController struct {
	BaseController
	inboundController     *InboundController
	serverController      *ServerController
	multiServerController *MultiServerController
	Tgbot                 service.Tgbot
}

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

// checkAPIAuth is a middleware that returns 404 for unauthenticated API requests
// to hide the existence of API endpoints from unauthorized users
// have second type of authentication - APIKEY for remote and multiserver access
func (a *APIController) checkAPIAuth(c *gin.Context) {
	if !session.IsLogin(c) {
		apiKey := c.GetHeader("X-API-KEY")
		if apiKey == "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		settingService := service.SettingService{}
		panelAPIKey, err := settingService.GetAPIKey()
		if err != nil || panelAPIKey == "" || apiKey != panelAPIKey {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}  
	}

	c.Next()
}

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup) {
	// Main API group
	api := g.Group("/panel/api")
	api.Use(a.checkAPIAuth)

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)

	// Server API
	server := api.Group("/server")
	a.serverController = NewServerController(server)

	// Servers api
	servers := api.Group("/servers")
	a.multiServerController = NewMultiServerController(servers)

	// Extra routes
	api.GET("/backuptotgbot", a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
