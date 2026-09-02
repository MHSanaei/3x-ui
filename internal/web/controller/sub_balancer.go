package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
// via ShouldBind, inboundIds as repeated keys. enabled is returned as *bool so
// Update can keep the stored value when the key is absent; a bad value is a 400.
func parseSubBalancerForm(c *gin.Context) (*model.SubBalancer, *bool, error) {
	form := struct {
		Remark    string `form:"remark"`
		Strategy  string `form:"strategy"`
		SortOrder int    `form:"sortOrder"`
	}{}
	if err := c.ShouldBind(&form); err != nil {
		return nil, nil, err
	}
	var enabled *bool
	if raw, ok := c.GetPostForm("enabled"); ok {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid enabled %q: %w", raw, err)
		}
		enabled = &v
	}
	balancer := &model.SubBalancer{
		Remark:    form.Remark,
		Strategy:  form.Strategy,
		SortOrder: form.SortOrder,
	}
	for _, raw := range c.PostFormArray("inboundIds") {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid inbound id %q: %w", raw, err)
		}
		balancer.InboundIds = append(balancer.InboundIds, id)
	}
	// Weights arrive as one JSON object ("memberWeights":{"3":0.5}); gin cannot
	// bind bracket-keyed maps from urlencoded forms, unlike repeated scalars.
	if raw, ok := c.GetPostForm("memberWeights"); ok && strings.TrimSpace(raw) != "" {
		weights := map[int]float64{}
		if err := json.Unmarshal([]byte(raw), &weights); err != nil {
			return nil, nil, fmt.Errorf("invalid memberWeights %q: %w", raw, err)
		}
		balancer.MemberWeights = weights
	}
	return balancer, enabled, nil
}

func (a *SubBalancerController) parseID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id %q", c.Param("id"))
	}
	return id, nil
}

func (a *SubBalancerController) list(c *gin.Context) {
	balancers, err := a.SubBalancerService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.list"), err)
		return
	}
	jsonObj(c, balancers, nil)
}

func (a *SubBalancerController) create(c *gin.Context) {
	balancer, enabled, err := parseSubBalancerForm(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.create"), err)
		return
	}
	balancer.Enabled = enabled == nil || *enabled
	created, err := a.SubBalancerService.Create(balancer)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.create"), err)
		return
	}
	jsonObj(c, created, nil)
}

func (a *SubBalancerController) update(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.invalidId"), err)
		return
	}
	balancer, enabled, err := parseSubBalancerForm(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.update"), err)
		return
	}
	updated, err := a.SubBalancerService.Update(id, balancer, enabled)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.update"), err)
		return
	}
	jsonObj(c, updated, nil)
}

func (a *SubBalancerController) del(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.invalidId"), err)
		return
	}
	if err := a.SubBalancerService.Delete(id); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.subBalancers.toasts.delete"), err)
		return
	}
	jsonObj(c, "", nil)
}
