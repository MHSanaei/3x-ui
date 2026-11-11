package service

import (
	"slices"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

var chainProxyTypes = map[string]struct{}{
	"socks": {},
	"http":  {},
}

const (
	chainProxyRemarkMax = 256
	chainProxyServerMax = 253
	chainProxyCredMax   = 256
)

// ChainProxyService manages the upstream SOCKS/HTTP proxies a subscriber's
// client can dial before this panel's inbound. Rows are read per subscription
// render by internal/sub, so mutations need no xray restart.
type ChainProxyService struct{}

func (s *ChainProxyService) validate(proxy *model.ChainProxy) error {
	proxy.Remark = strings.TrimSpace(proxy.Remark)
	if proxy.Remark == "" {
		return common.NewError("chain proxy remark is required")
	}
	if len(proxy.Remark) > chainProxyRemarkMax {
		return common.NewError("chain proxy remark too long (max 256)")
	}
	proxy.Type = strings.ToLower(strings.TrimSpace(proxy.Type))
	if proxy.Type == "" {
		proxy.Type = "socks"
	}
	if _, ok := chainProxyTypes[proxy.Type]; !ok {
		return common.NewError("invalid chain proxy type:", proxy.Type)
	}
	proxy.Server = normalizeChainProxyServer(proxy.Server)
	if proxy.Server == "" {
		return common.NewError("chain proxy server is required")
	}
	if len(proxy.Server) > chainProxyServerMax {
		return common.NewError("chain proxy server too long (max 253)")
	}
	if strings.ContainsAny(proxy.Server, " \t\r\n") {
		return common.NewError("chain proxy server must be a bare host or IP")
	}
	if proxy.Port < 1 || proxy.Port > 65535 {
		return common.NewError("chain proxy port must be between 1 and 65535")
	}
	// xray's socks outbound has no TLS — an HTTPS proxy is only representable
	// over the http outbound, so reject the combination instead of dropping it.
	if proxy.TLS && proxy.Type != "http" {
		return common.NewError("chain proxy tls is only supported for http proxies")
	}
	if strings.ContainsAny(proxy.Username, " \t\r\n") {
		return common.NewError("chain proxy username must not contain whitespace")
	}
	if len(proxy.Username) > chainProxyCredMax || len(proxy.Password) > chainProxyCredMax {
		return common.NewError("chain proxy credentials too long (max 256)")
	}
	if strings.ContainsAny(proxy.Password, "\r\n") {
		return common.NewError("chain proxy password must not contain a newline")
	}
	return nil
}

// normalizeChainProxyServer accepts what an operator pastes: a bare host, or a
// full URL whose scheme, credentials, port and brackets the outbound rejects.
// The separate port field stays authoritative, so a pasted port is dropped.
func normalizeChainProxyServer(raw string) string {
	server := strings.TrimSpace(raw)
	if idx := strings.Index(server, "://"); idx >= 0 {
		server = server[idx+3:]
	}
	if idx := strings.LastIndex(server, "@"); idx >= 0 {
		server = server[idx+1:]
	}
	if idx := strings.IndexAny(server, "/?#"); idx >= 0 {
		server = server[:idx]
	}
	if strings.HasPrefix(server, "[") {
		if end := strings.Index(server, "]"); end > 0 {
			return server[1:end]
		}
	}
	// A single colon separates host:port; several mean an unbracketed IPv6.
	if strings.Count(server, ":") == 1 {
		host, port, _ := strings.Cut(server, ":")
		if isAllDigits(port) {
			return host
		}
	}
	return server
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *ChainProxyService) List() ([]*model.ChainProxy, error) {
	var proxies []*model.ChainProxy
	err := database.GetDB().Order("id asc").Find(&proxies).Error
	return proxies, err
}

func (s *ChainProxyService) Get(id int) (*model.ChainProxy, error) {
	var proxy model.ChainProxy
	if err := database.GetDB().First(&proxy, id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (s *ChainProxyService) Create(proxy *model.ChainProxy) (*model.ChainProxy, error) {
	if err := s.validate(proxy); err != nil {
		return nil, err
	}
	if err := database.GetDB().Create(proxy).Error; err != nil {
		return nil, err
	}
	InvalidateSubData()
	return proxy, nil
}

func (s *ChainProxyService) Update(id int, proxy *model.ChainProxy, enabled *bool) (*model.ChainProxy, error) {
	if err := s.validate(proxy); err != nil {
		return nil, err
	}
	current, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	current.Remark = proxy.Remark
	current.Type = proxy.Type
	current.Server = proxy.Server
	current.Port = proxy.Port
	current.Username = proxy.Username
	current.Password = proxy.Password
	current.TLS = proxy.TLS
	if enabled != nil {
		current.Enabled = *enabled
	}
	if err := database.GetDB().Save(current).Error; err != nil {
		return nil, err
	}
	InvalidateSubData()
	return current, nil
}

// Delete removes a hop and every reference to it: selectors lose the member and
// an inbound assigned this single proxy falls back to a direct connection.
func (s *ChainProxyService) Delete(id int) error {
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.ChainProxy{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return common.NewError("chain proxy not found")
		}
		var selectors []model.ChainSelector
		if err := tx.Find(&selectors).Error; err != nil {
			return err
		}
		for i := range selectors {
			if !slices.Contains(selectors[i].ProxyIds, id) {
				continue
			}
			selectors[i].ProxyIds = slices.DeleteFunc(selectors[i].ProxyIds, func(pid int) bool { return pid == id })
			if err := tx.Save(&selectors[i]).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Inbound{}).Where("chain_proxy_id = ?", id).
			Update("chain_proxy_id", 0).Error
	})
	if err != nil {
		return err
	}
	InvalidateSubData()
	return nil
}

// ChainSelectorService manages groups of chain proxies a client picks between
// by strategy; it is assigned to an inbound as an alternative to a single hop.
type ChainSelectorService struct{}

func (s *ChainSelectorService) validate(selector *model.ChainSelector) error {
	selector.Remark = strings.TrimSpace(selector.Remark)
	if selector.Remark == "" {
		return common.NewError("chain selector remark is required")
	}
	if len(selector.Remark) > chainProxyRemarkMax {
		return common.NewError("chain selector remark too long (max 256)")
	}
	if selector.Strategy == "" {
		selector.Strategy = "random"
	}
	if _, ok := subBalancerStrategies[selector.Strategy]; !ok {
		return common.NewError("invalid chain selector strategy:", selector.Strategy)
	}
	proxyIds := make([]int, 0, len(selector.ProxyIds))
	for _, id := range selector.ProxyIds {
		if id <= 0 || slices.Contains(proxyIds, id) {
			continue
		}
		proxyIds = append(proxyIds, id)
	}
	if len(proxyIds) == 0 {
		return common.NewError("chain selector must contain at least one proxy")
	}
	var found int64
	if err := database.GetDB().Model(&model.ChainProxy{}).Where("id IN ?", proxyIds).Count(&found).Error; err != nil {
		return err
	}
	if int(found) != len(proxyIds) {
		return common.NewError("chain selector references an unknown chain proxy")
	}
	selector.ProxyIds = proxyIds
	return nil
}

func (s *ChainSelectorService) List() ([]*model.ChainSelector, error) {
	var selectors []*model.ChainSelector
	err := database.GetDB().Order("id asc").Find(&selectors).Error
	return selectors, err
}

func (s *ChainSelectorService) Get(id int) (*model.ChainSelector, error) {
	var selector model.ChainSelector
	if err := database.GetDB().First(&selector, id).Error; err != nil {
		return nil, err
	}
	return &selector, nil
}

func (s *ChainSelectorService) Create(selector *model.ChainSelector) (*model.ChainSelector, error) {
	if err := s.validate(selector); err != nil {
		return nil, err
	}
	if err := database.GetDB().Create(selector).Error; err != nil {
		return nil, err
	}
	InvalidateSubData()
	return selector, nil
}

func (s *ChainSelectorService) Update(id int, selector *model.ChainSelector, enabled *bool) (*model.ChainSelector, error) {
	if err := s.validate(selector); err != nil {
		return nil, err
	}
	current, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	current.Remark = selector.Remark
	current.Strategy = selector.Strategy
	current.ProxyIds = selector.ProxyIds
	if enabled != nil {
		current.Enabled = *enabled
	}
	if err := database.GetDB().Save(current).Error; err != nil {
		return nil, err
	}
	InvalidateSubData()
	return current, nil
}

// Delete removes a selector and clears the assignment of every inbound using it.
func (s *ChainSelectorService) Delete(id int) error {
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.ChainSelector{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return common.NewError("chain selector not found")
		}
		return tx.Model(&model.Inbound{}).Where("chain_selector_id = ?", id).
			Update("chain_selector_id", 0).Error
	})
	if err != nil {
		return err
	}
	InvalidateSubData()
	return nil
}

// validateInboundChain rejects an assignment the subscription could never
// honor: both kinds at once, a missing row, or a protocol whose client config
// (and share links) have nowhere to carry a hop.
func (s *InboundService) validateInboundChain(inbound *model.Inbound) error {
	if inbound.ChainProxyId < 0 || inbound.ChainSelectorId < 0 {
		return common.NewError("chain proxy ids must not be negative")
	}
	if inbound.ChainProxyId > 0 && inbound.ChainSelectorId > 0 {
		return common.NewError("an inbound takes a chain proxy or a chain selector, not both")
	}
	if inbound.ChainProxyId == 0 && inbound.ChainSelectorId == 0 {
		return nil
	}
	if !chainableInboundProtocol(inbound.Protocol) {
		return common.NewError("chain proxies apply to vmess, vless, trojan and shadowsocks inbounds only")
	}
	db := database.GetDB()
	if inbound.ChainProxyId > 0 {
		var count int64
		if err := db.Model(&model.ChainProxy{}).Where("id = ?", inbound.ChainProxyId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return common.NewError("chain proxy not found")
		}
		return nil
	}
	var count int64
	if err := db.Model(&model.ChainSelector{}).Where("id = ?", inbound.ChainSelectorId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return common.NewError("chain selector not found")
	}
	return nil
}

// chainableInboundProtocol mirrors internal/sub's chainableProtocol; the panel
// cannot import the subscription package, so the two lists are kept in sync.
func chainableInboundProtocol(protocol model.Protocol) bool {
	switch protocol {
	case model.VMESS, model.VLESS, model.Trojan, model.Shadowsocks:
		return true
	default:
		return false
	}
}
