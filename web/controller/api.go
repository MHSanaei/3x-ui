package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type APIController struct {
	BaseController
	inbounds           *InboundController
	Tgbot               service.Tgbot
	server              *ServerController
}

func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	a.initApiV2Router(g)
	return a
}

func (controller *APIController) initRouter(router *gin.RouterGroup) {
    apiV1 := router.Group("/panel/api")
    apiV1.Use(controller.checkLogin)

    inboundsApiGroup := apiV1.Group("/inbounds")
	controller.inbounds = NewInboundController(inboundsApiGroup)

	inboundRoutes := []struct {
		Method  string
		Path    string
		Handler gin.HandlerFunc
	}{
		{"GET", "/createbackup", controller.createBackup},
		{"GET", "/list", controller.inbounds.getInbounds},
		{"GET", "/get/:id", controller.inbounds.getInbound},
		{"GET", "/getClientTraffics/:email", controller.inbounds.getClientTraffics},
		{"GET", "/getClientTrafficsById/:id", controller.inbounds.getClientTrafficsById},
		{"POST", "/add", controller.inbounds.addInbound},
		{"POST", "/del/:id", controller.inbounds.delInbound},
		{"POST", "/update/:id", controller.inbounds.updateInbound},
		{"POST", "/clientIps/:email", controller.inbounds.getClientIps},
		{"POST", "/clearClientIps/:email", controller.inbounds.clearClientIps},
		{"POST", "/addClient", controller.inbounds.addInboundClient},
		{"POST", "/:id/delClient/:clientId", controller.inbounds.delInboundClient},
		{"POST", "/updateClient/:clientId", controller.inbounds.updateInboundClient},
		{"POST", "/:id/resetClientTraffic/:email", controller.inbounds.resetClientTraffic},
		{"POST", "/resetAllTraffics", controller.inbounds.resetAllTraffics},
		{"POST", "/resetAllClientTraffics/:id", controller.inbounds.resetAllClientTraffics},
		{"POST", "/delDepletedClients/:id", controller.inbounds.delDepletedClients},
		{"POST", "/onlines", controller.inbounds.onlines},
	}

	for _, route := range inboundRoutes {
		inboundsApiGroup.Handle(route.Method, route.Path, route.Handler)
	}
}

func (a *APIController) createBackup(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}


func (controller *APIController) initApiV2Router(router *gin.RouterGroup) {
    apiV2 := router.Group("/api/v2")
    apiV2.Use(controller.apiTokenGuard)

    serverApiGroup := apiV2.Group("/server")
    inboundsApiGroup := apiV2.Group("/inbounds")

	controller.inbounds = NewInboundController(inboundsApiGroup)
	controller.server = NewServerController(serverApiGroup)

    /**
    * Inbounds
    */
    inboundsApiGroup.GET("/", controller.inbounds.getInbounds)
    inboundsApiGroup.DELETE("/traffic", controller.inbounds.resetAllClientTraffics)

    /**
    * Inbound
    */
    inboundsApiGroup.POST("/", controller.inbounds.addInbound)
    inboundsApiGroup.GET("/:id", controller.inbounds.getInbound)
    inboundsApiGroup.DELETE("/:id", controller.inbounds.delInbound)
    inboundsApiGroup.PUT("/:id", controller.inbounds.updateInbound)

    inboundsApiGroup.DELETE("/:id/traffic", controller.inbounds.delInbound)
    inboundsApiGroup.DELETE("/:id/depleted-clients", controller.inbounds.delDepletedClients)

   /**
    * Inbound clients
    */
    inboundsApiGroup.GET("/:id/clients/", controller.inbounds.getInboundClients)

    /**
    * Inbound client
    */
    inboundsApiGroup.POST("/:id/clients", controller.inbounds.addInboundClient)
    inboundsApiGroup.GET("/:id/clients/:clientId", controller.inbounds.getClientById)
    inboundsApiGroup.PUT("/:id/clients/:clientId", controller.inbounds.updateInboundClient)
    inboundsApiGroup.DELETE("/:id/clients/:clientId", controller.inbounds.delInboundClient)

    inboundsApiGroup.GET("/:id/clients/:clientId/traffic", controller.inbounds.getClientTrafficsById)
    // TODO: get client ips by ID
    // TODO: clear client ips by ID
    // TODO: reset client traffic by ID

    /**
    * Inbound client by email
    */
    inboundsApiGroup.GET("/:id/clients/email/:email", controller.inbounds.getClientByEmail)
    // TODO: update client by Email
    // TODO: delete client by Email

    inboundsApiGroup.GET("/:id/clients/email/:email/ips", controller.inbounds.getClientIps)
    inboundsApiGroup.DELETE("/:id/clients/email/:email/ips", controller.inbounds.clearClientIps)

    inboundsApiGroup.GET("/:id/clients/email/:email/traffic", controller.inbounds.getClientTraffics)
    inboundsApiGroup.DELETE("/:id/clients/email/:email/traffic", controller.inbounds.resetClientTraffic)

    /**
    * Other
    */
    inboundsApiGroup.GET("/create-backup", controller.createBackup)
    inboundsApiGroup.GET("/online", controller.inbounds.onlines)

    /**
    * Server
    */
    serverApiGroup.GET("/status", controller.server.status)
}