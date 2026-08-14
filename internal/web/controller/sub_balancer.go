package controller

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// SubBalancerController manages client-side JSON-subscription balancers.
type SubBalancerController struct {
	SubBalancerService service.SubBalancerService
}

func NewSubBalancerController(g *gin.RouterGroup) *SubBalancerController {
	a := &SubBalancerController{}
	g = g.Group("/sub-balancers")
	g.GET("", a.list)
	g.POST("", a.create)
	g.POST("/:id", a.update)
	g.DELETE("/:id", a.del)
	g.POST("/:id/del", a.del)
	return a
}

// parseSubBalancerForm reads the urlencoded form (HttpUtil default): scalars
// via ShouldBind, inboundIds as repeated keys (inboundIds=1&inboundIds=2).
func parseSubBalancerForm(c *gin.Context) (*model.SubBalancer, error) {
	form := struct {
		Remark    string `form:"remark"`
		Strategy  string `form:"strategy"`
		SortOrder int    `form:"sortOrder"`
	}{}
	if err := c.ShouldBind(&form); err != nil {
		return nil, err
	}
	balancer := &model.SubBalancer{
		Remark:    form.Remark,
		Strategy:  form.Strategy,
		SortOrder: form.SortOrder,
		Enabled:   c.PostForm("enabled") != "false",
	}
	for _, raw := range c.PostFormArray("inboundIds") {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid inbound id %q: %w", raw, err)
		}
		balancer.InboundIds = append(balancer.InboundIds, id)
	}
	return balancer, nil
}

func (a *SubBalancerController) parseID(c *gin.Context) (int, error) {
	var id int
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
		return 0, err
	}
	return id, nil
}

func (a *SubBalancerController) list(c *gin.Context) {
	balancers, err := a.SubBalancerService.List()
	if err != nil {
		jsonMsg(c, "Failed to list subscription balancers", err)
		return
	}
	jsonObj(c, balancers, nil)
}

func (a *SubBalancerController) create(c *gin.Context) {
	balancer, err := parseSubBalancerForm(c)
	if err != nil {
		jsonMsg(c, "Failed to create subscription balancer", err)
		return
	}
	created, err := a.SubBalancerService.Create(balancer)
	if err != nil {
		jsonMsg(c, "Failed to create subscription balancer", err)
		return
	}
	jsonObj(c, created, nil)
}

func (a *SubBalancerController) update(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	balancer, err := parseSubBalancerForm(c)
	if err != nil {
		jsonMsg(c, "Failed to update subscription balancer", err)
		return
	}
	updated, err := a.SubBalancerService.Update(id, balancer)
	if err != nil {
		jsonMsg(c, "Failed to update subscription balancer", err)
		return
	}
	jsonObj(c, updated, nil)
}

func (a *SubBalancerController) del(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	if err := a.SubBalancerService.Delete(id); err != nil {
		jsonMsg(c, "Failed to delete subscription balancer", err)
		return
	}
	jsonObj(c, "", nil)
}
