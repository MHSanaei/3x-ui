package controller

import (
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// ResellerController manages reseller accounts. Only the panel admin may
// create/list/update/delete resellers and assign inbounds to them; a reseller
// may view their own account, usage and assigned inbounds via /resellers/me.
type ResellerController struct {
	resellerService panel.ResellerService
	inboundService  service.InboundService
}

func NewResellerController(g *gin.RouterGroup) *ResellerController {
	a := &ResellerController{}
	a.initRouter(g)
	return a
}

func (a *ResellerController) initRouter(g *gin.RouterGroup) {
	resellers := g.Group("/resellers")
	resellers.GET("/list", a.requireAdmin, a.list)
	resellers.POST("/add", a.requireAdmin, a.add)
	resellers.POST("/update/:id", a.requireAdmin, a.update)
	resellers.POST("/del/:id", a.requireAdmin, a.del)

	// A reseller (or admin) inspects their own account.
	resellers.GET("/me", a.me)

	// Assigned inbounds: admin manages any reseller's, a reseller sees their own.
	resellers.GET("/:id/inbounds", a.inbounds)
	resellers.POST("/:id/inbounds", a.requireAdmin, a.assignInbounds)

	// Assign existing clients to a reseller (admin only).
	resellers.POST("/:id/clients/assign", a.requireAdmin, a.assignClients)
}

func (a *ResellerController) loginUser(c *gin.Context) *model.User {
	return session.GetLoginUser(c)
}

func (a *ResellerController) requireAdmin(c *gin.Context) {
	user := a.loginUser(c)
	if user == nil || user.Role != model.UserRoleAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, entity.Msg{Success: false, Msg: "admin access required"})
		return
	}
	c.Next()
}

// list returns the admin's resellers with their current usage.
func (a *ResellerController) list(c *gin.Context) {
	user := a.loginUser(c)
	rows, err := a.resellerService.ListResellers(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, rows, nil)
}

// add creates a new reseller.
func (a *ResellerController) add(c *gin.Context) {
	user := a.loginUser(c)
	var input panel.ResellerCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	created, err := a.resellerService.CreateReseller(user.Id, input)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, created, nil)
}

// update updates a reseller's quotas and credentials.
func (a *ResellerController) update(c *gin.Context) {
	user := a.loginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var input panel.ResellerUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.resellerService.UpdateReseller(id, user.Id, input); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
}

// del removes a reseller that has no clients.
func (a *ResellerController) del(c *gin.Context) {
	user := a.loginUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.resellerService.DeleteReseller(id, user.Id); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
}

// me returns the logged-in user's own account. A reseller sees their usage and
// assigned inbounds; an admin sees their admin record.
func (a *ResellerController) me(c *gin.Context) {
	user := a.loginUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, entity.Msg{Success: false, Msg: "not authenticated"})
		return
	}
	out := gin.H{"user": gin.H{
		"id":       user.Id,
		"username": user.Username,
		"role":     user.Role,
		"parentId": user.ParentId,
		"totalGB":  user.TotalGB,
		"usedGB":   user.UsedGB,
		"maxClients": user.MaxClients,
		"enable":   user.Enable,
	}}
	if user.Role == model.UserRoleReseller {
		out["usage"] = a.resellerService.UsageFor(user.Id)
		assigned, err := a.resellerService.AssignedInbounds(user.Id)
		if err != nil {
			assigned = nil
		}
		out["assignedInbounds"] = assigned
	}
	jsonObj(c, out, nil)
}

// inbounds returns the inbound options available to a reseller (their assigned
// inbounds). An admin may query any reseller; a reseller only their own.
func (a *ResellerController) inbounds(c *gin.Context) {
	requester := a.loginUser(c)
	resellerId, err := strconv.Atoi(c.Param("id"))
	if err != nil || resellerId < 1 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if requester.Role == model.UserRoleReseller && requester.Id != resellerId {
		c.AbortWithStatusJSON(http.StatusForbidden, entity.Msg{Success: false, Msg: "forbidden"})
		return
	}
	assigned, err := a.resellerService.AssignedInbounds(resellerId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if len(assigned) == 0 {
		jsonObj(c, []any{}, nil)
		return
	}
	inbounds, err := a.inboundService.GetInboundsSlim(0)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	allowed := make(map[int]struct{}, len(assigned))
	for _, id := range assigned {
		allowed[id] = struct{}{}
	}
	out := make([]any, 0, len(assigned))
	for _, ib := range inbounds {
		if _, ok := allowed[ib.Id]; ok {
			out = append(out, ib)
		}
	}
	jsonObj(c, out, nil)
}

// assignInbounds replaces a reseller's assigned inbound set.
func (a *ResellerController) assignInbounds(c *gin.Context) {
	user := a.loginUser(c)
	resellerId, err := strconv.Atoi(c.Param("id"))
	if err != nil || resellerId < 1 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var body struct {
		InboundIds []int `json:"inboundIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.resellerService.AssignInbounds(resellerId, body.InboundIds); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
}

// assignClients moves the given clients under the ownership of the reseller.
func (a *ResellerController) assignClients(c *gin.Context) {
	resellerId, err := strconv.Atoi(c.Param("id"))
	if err != nil || resellerId < 1 {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var body struct {
		ClientEmails []string `json:"clientEmails"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.resellerService.AssignClientsToReseller(resellerId, body.ClientEmails); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.resellers.clientsAssigned"), nil)
}
