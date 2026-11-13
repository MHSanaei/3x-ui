package controller

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ChainProxyController manages the upstream SOCKS/HTTP proxies a subscriber's
// client hops through before this panel's inbound, plus their selectors.
type ChainProxyController struct {
	ChainProxyService    service.ChainProxyService
	ChainSelectorService service.ChainSelectorService
}

func NewChainProxyController(g *gin.RouterGroup) *ChainProxyController {
	a := &ChainProxyController{}
	proxies := g.Group("/chain-proxies")
	proxies.GET("", a.listProxies)
	proxies.POST("", a.createProxy)
	proxies.POST("/:id", a.updateProxy)
	proxies.DELETE("/:id", a.delProxy)
	proxies.POST("/:id/del", a.delProxy)

	selectors := g.Group("/chain-selectors")
	selectors.GET("", a.listSelectors)
	selectors.POST("", a.createSelector)
	selectors.POST("/:id", a.updateSelector)
	selectors.DELETE("/:id", a.delSelector)
	selectors.POST("/:id/del", a.delSelector)
	return a
}

// parseChainProxyForm reads the urlencoded form (HttpUtil default). enabled is
// returned as *bool so Update can keep the stored value when the key is absent;
// a bad value is a 400 rather than a silent default.
func parseChainProxyForm(c *gin.Context) (*model.ChainProxy, *bool, error) {
	form := struct {
		Remark   string `form:"remark"`
		Type     string `form:"type"`
		Server   string `form:"server"`
		Port     int    `form:"port"`
		Username string `form:"username"`
		Password string `form:"password"`
		TLS      bool   `form:"tls"`
	}{}
	if err := c.ShouldBind(&form); err != nil {
		return nil, nil, err
	}
	enabled, err := parseOptionalBool(c, "enabled")
	if err != nil {
		return nil, nil, err
	}
	return &model.ChainProxy{
		Remark:   form.Remark,
		Type:     form.Type,
		Server:   form.Server,
		Port:     form.Port,
		Username: form.Username,
		Password: form.Password,
		TLS:      form.TLS,
	}, enabled, nil
}

func parseChainSelectorForm(c *gin.Context) (*model.ChainSelector, *bool, error) {
	form := struct {
		Remark   string `form:"remark"`
		Strategy string `form:"strategy"`
	}{}
	if err := c.ShouldBind(&form); err != nil {
		return nil, nil, err
	}
	enabled, err := parseOptionalBool(c, "enabled")
	if err != nil {
		return nil, nil, err
	}
	selector := &model.ChainSelector{Remark: form.Remark, Strategy: form.Strategy}
	for _, raw := range c.PostFormArray("proxyIds") {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid proxy id %q: %w", raw, err)
		}
		selector.ProxyIds = append(selector.ProxyIds, id)
	}
	return selector, enabled, nil
}

func parseOptionalBool(c *gin.Context, field string) (*bool, error) {
	raw, ok := c.GetPostForm(field)
	if !ok {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", field, raw, err)
	}
	return &value, nil
}

func (a *ChainProxyController) parseID(c *gin.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id %q", c.Param("id"))
	}
	return id, nil
}

func (a *ChainProxyController) listProxies(c *gin.Context) {
	proxies, err := a.ChainProxyService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.list"), err)
		return
	}
	jsonObj(c, proxies, nil)
}

func (a *ChainProxyController) createProxy(c *gin.Context) {
	proxy, enabled, err := parseChainProxyForm(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.create"), err)
		return
	}
	proxy.Enabled = enabled == nil || *enabled
	created, err := a.ChainProxyService.Create(proxy)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.create"), err)
		return
	}
	jsonObj(c, created, nil)
}

func (a *ChainProxyController) updateProxy(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.invalidId"), err)
		return
	}
	proxy, enabled, err := parseChainProxyForm(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.update"), err)
		return
	}
	updated, err := a.ChainProxyService.Update(id, proxy, enabled)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.update"), err)
		return
	}
	jsonObj(c, updated, nil)
}

func (a *ChainProxyController) delProxy(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.invalidId"), err)
		return
	}
	if err := a.ChainProxyService.Delete(id); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainProxies.toasts.delete"), err)
		return
	}
	jsonObj(c, "", nil)
}

func (a *ChainProxyController) listSelectors(c *gin.Context) {
	selectors, err := a.ChainSelectorService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.list"), err)
		return
	}
	jsonObj(c, selectors, nil)
}

func (a *ChainProxyController) createSelector(c *gin.Context) {
	selector, enabled, err := parseChainSelectorForm(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.create"), err)
		return
	}
	selector.Enabled = enabled == nil || *enabled
	created, err := a.ChainSelectorService.Create(selector)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.create"), err)
		return
	}
	jsonObj(c, created, nil)
}

func (a *ChainProxyController) updateSelector(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.invalidId"), err)
		return
	}
	selector, enabled, err := parseChainSelectorForm(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.update"), err)
		return
	}
	updated, err := a.ChainSelectorService.Update(id, selector, enabled)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.update"), err)
		return
	}
	jsonObj(c, updated, nil)
}

func (a *ChainProxyController) delSelector(c *gin.Context) {
	id, err := a.parseID(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.invalidId"), err)
		return
	}
	if err := a.ChainSelectorService.Delete(id); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.chainSelectors.toasts.delete"), err)
		return
	}
	jsonObj(c, "", nil)
}
