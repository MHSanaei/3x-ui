package controller

import (
	"encoding/base64"
	"strings"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type SUBController struct {
	BaseController

	subService service.SubService
}

func NewSUBController(g *gin.RouterGroup) *SUBController {
	a := &SUBController{}
	a.initRouter(g)
	return a
}

func (a *SUBController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/sub")

	g.GET("/:subid", a.subs)
}

func (a *SUBController) subs(c *gin.Context) {
	subId := c.Param("subid")
	host := strings.Split(c.Request.Host, ":")[0]
	subs, err := a.subService.GetSubs(subId, host)
	if err != nil {
		c.String(400, "Error!")
	} else {
		result := ""
		for _, sub := range subs {
			result += sub + "\n"
		}
		c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
	}
}
