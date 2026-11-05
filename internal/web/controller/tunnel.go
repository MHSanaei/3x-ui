package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type TunnelController struct{ service service.TunnelService }

func NewTunnelController(g *gin.RouterGroup) *TunnelController {
	a := &TunnelController{}
	g.GET("/list", a.list)
	g.GET("/capabilities", a.capabilities)
	g.POST("/add", a.save)
	g.POST("/update/:id", a.save)
	g.POST("/start/:id", a.start)
	g.POST("/stop/:id", a.stop)
	g.POST("/restart/:id", a.restart)
	g.POST("/del/:id", a.remove)
	g.POST("/export/:id", a.export)
	g.POST("/sync", a.sync)
	return a
}

func (a *TunnelController) list(c *gin.Context) { out, err := a.service.List(); jsonObj(c, out, err) }

func (a *TunnelController) capabilities(c *gin.Context) {
	var id *int
	if raw := c.Query("nodeId"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			jsonObj(c, nil, errors.New("invalid node ID"))
			return
		}
		id = &n
	}
	out, err := a.service.Capabilities(c.Request.Context(), id)
	jsonObj(c, out, err)
}

func tunnelID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid tunnel ID")
	}
	return id, nil
}

func (a *TunnelController) save(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req service.TunnelMutation
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonObj(c, nil, errors.New("invalid tunnel request"))
		return
	}
	id := 0
	if c.Param("id") != "" {
		var err error
		id, err = tunnelID(c)
		if err != nil {
			jsonObj(c, nil, err)
			return
		}
	}
	out, err := a.service.Save(c.Request.Context(), id, req)
	jsonObj(c, out, err)
}

func (a *TunnelController) action(c *gin.Context, action string) {
	id, err := tunnelID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	out, err := a.service.Action(c.Request.Context(), id, action)
	jsonObj(c, out, err)
}
func (a *TunnelController) start(c *gin.Context)   { a.action(c, "start") }
func (a *TunnelController) stop(c *gin.Context)    { a.action(c, "stop") }
func (a *TunnelController) restart(c *gin.Context) { a.action(c, "restart") }
func (a *TunnelController) remove(c *gin.Context)  { a.action(c, "delete") }
func (a *TunnelController) export(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, err := tunnelID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	out, err := a.service.Export(id)
	jsonObj(c, out, err)
}

func (a *TunnelController) sync(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var spec wattunnel.Spec
	if err := c.ShouldBindJSON(&spec); err != nil {
		jsonObj(c, nil, errors.New("invalid tunnel synchronization request"))
		return
	}
	out, err := a.service.Sync(c.Request.Context(), spec, c.GetString("api_identity"))
	jsonObj(c, out, err)
}
