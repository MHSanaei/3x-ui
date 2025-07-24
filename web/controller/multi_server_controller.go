package controller

import (
	"strconv"

	"x-ui/database/model"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type MultiServerController struct {
	multiServerService service.MultiServerService
}

func NewMultiServerController(g *gin.RouterGroup) *MultiServerController {
	c := &MultiServerController{}
	c.initRouter(g)
	return c
}

func (c *MultiServerController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/server")

	g.GET("/list", c.getServers)
	g.POST("/add", c.addServer)
	g.POST("/del/:id", c.delServer)
	g.POST("/update/:id", c.updateServer)
}

func (c *MultiServerController) getServers(ctx *gin.Context) {
	servers, err := c.multiServerService.GetServers()
	if err != nil {
		jsonMsg(ctx, "Failed to get servers", err)
		return
	}
	jsonObj(ctx, servers, nil)
}

func (c *MultiServerController) addServer(ctx *gin.Context) {
	server := &model.Server{}
	err := ctx.ShouldBind(server)
	if err != nil {
		jsonMsg(ctx, "Invalid data", err)
		return
	}
	err = c.multiServerService.AddServer(server)
	if err != nil {
		jsonMsg(ctx, "Failed to add server", err)
		return
	}
	jsonMsg(ctx, "Server added successfully", nil)
}

func (c *MultiServerController) delServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid ID", err)
		return
	}
	err = c.multiServerService.DeleteServer(id)
	if err != nil {
		jsonMsg(ctx, "Failed to delete server", err)
		return
	}
	jsonMsg(ctx, "Server deleted successfully", nil)
}

func (c *MultiServerController) updateServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid ID", err)
		return
	}
	server := &model.Server{
		Id: id,
	}
	err = ctx.ShouldBind(server)
	if err != nil {
		jsonMsg(ctx, "Invalid data", err)
		return
	}
	err = c.multiServerService.UpdateServer(server)
	if err != nil {
		jsonMsg(ctx, "Failed to update server", err)
		return
	}
	jsonMsg(ctx, "Server updated successfully", nil)
}
