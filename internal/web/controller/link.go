package controller

import (
	"errors"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// LinkController serves the panel-wide external link library: the shared rows
// every client, group or inbound binds to, and what one client ends up with.
type LinkController struct {
	clientService service.ClientService
}

func NewLinkController(g *gin.RouterGroup) *LinkController {
	a := &LinkController{}
	a.initRouter(g)
	return a
}

func (a *LinkController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.POST("/add", a.save)
	g.POST("/del/:id", a.delete)
	g.POST("/enable/:id", a.setEnable)
	g.POST("/reorder", a.reorder)
	g.GET("/targets/:id", a.targets)
	g.POST("/assign/:id", a.assign)
	g.POST("/unassign/:id", a.unassign)
	g.GET("/client/:clientId", a.clientLinks)
	g.POST("/refresh/:id", a.refresh)
}

func (a *LinkController) list(c *gin.Context) {
	rows, err := a.clientService.ExternalLinkLibraryList()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *LinkController) save(c *gin.Context) {
	var body model.ExternalLink
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.ExternalLinkLibrarySave(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, body, nil)
	notifyClientsChanged()
}

func (a *LinkController) delete(c *gin.Context) {
	id, err := linkIdParam(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.ExternalLinkLibraryDelete(id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"id": id}, nil)
	notifyClientsChanged()
}

type linkEnableBody struct {
	Enable bool `json:"enable"`
}

func (a *LinkController) setEnable(c *gin.Context) {
	id, err := linkIdParam(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var body linkEnableBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.ExternalLinkLibrarySetEnable(id, body.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"id": id, "enable": body.Enable}, nil)
	notifyClientsChanged()
}

type linkReorderBody struct {
	Ids []int `json:"ids"`
}

func (a *LinkController) reorder(c *gin.Context) {
	var body linkReorderBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.ExternalLinkLibraryReorder(body.Ids); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"ids": body.Ids}, nil)
	notifyClientsChanged()
}

func (a *LinkController) targets(c *gin.Context) {
	id, err := linkIdParam(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	rows, err := a.clientService.ExternalLinkTargets(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *LinkController) assign(c *gin.Context) {
	a.mutateTargets(c, true)
}

func (a *LinkController) unassign(c *gin.Context) {
	a.mutateTargets(c, false)
}

func (a *LinkController) mutateTargets(c *gin.Context, assign bool) {
	id, err := linkIdParam(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var body service.ExternalLinkAssignRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var affected int
	if assign {
		affected, err = a.clientService.ExternalLinkAssign(id, body)
	} else {
		affected, err = a.clientService.ExternalLinkUnassign(id, body)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	notifyClientsChanged()
}

// clientLinks returns what one client actually receives: the rows it owns plus
// the ones its group, its inbounds or the panel grant it.
func (a *LinkController) clientLinks(c *gin.Context) {
	clientId, err := strconv.Atoi(c.Param("clientId"))
	if err != nil || clientId <= 0 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), errLinkId)
		return
	}
	rows, err := a.clientService.ClientExternalLinkViews(clientId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, rows, nil)
}

var (
	errLinkId           = errors.New("link id must be a positive integer")
	errNotASubscription = errors.New("only an external subscription can be refreshed")
)

func linkIdParam(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, errLinkId
	}
	return id, nil
}

// refresh fetches a subscription row again on the operator's command, bypassing
// the cached copy. The cache lives in internal/sub, which owns every fetch.
func (a *LinkController) refresh(c *gin.Context) {
	id, err := linkIdParam(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	row, err := a.clientService.ExternalLinkLibraryGet(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if row.Kind != model.ExternalLinkKindSubscription {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), errNotASubscription)
		return
	}
	count, err := sub.RefreshExternalSubscription(row.Value, row.UserAgent, row.Headers, row.CacheTTL)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"count": count}, nil)
	notifyClientsChanged()
}
