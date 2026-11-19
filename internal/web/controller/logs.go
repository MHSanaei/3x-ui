package controller

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/logview"

	"github.com/gin-gonic/gin"
)

// LogController serves the read-only log viewer. It reads this panel's own
// log folder only, so it has no node dispatch to do.
type LogController struct {
	BaseController

	logs logview.Service
}

// NewLogController creates a LogController and registers its routes.
func NewLogController(g *gin.RouterGroup) *LogController {
	a := &LogController{}
	g.GET("/files", a.files)
	g.GET("/read", a.read)
	g.GET("/stats", a.stats)
	return a
}

func (a *LogController) files(c *gin.Context) {
	list, err := a.logs.ListFiles()
	jsonObj(c, list, err)
}

func (a *LogController) read(c *gin.Context) {
	var req logview.ReadRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		jsonObj(c, nil, logview.ErrInvalidFilter)
		return
	}
	page, err := a.logs.Read(c.Request.Context(), req)
	jsonObj(c, page, err)
}

func (a *LogController) stats(c *gin.Context) {
	var req logview.StatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		jsonObj(c, nil, logview.ErrInvalidFilter)
		return
	}
	stats, err := a.logs.Stats(c.Request.Context(), req)
	jsonObj(c, stats, err)
}
