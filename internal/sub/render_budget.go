package sub

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// maxRenderedBytes bounds what one subscription response renders: every client and
// inbound of the subId repeats the remark and endpoints, so the size is a product.
const maxRenderedBytes = 8 << 20

// renderBudget counts rendered bytes against maxRenderedBytes. The zero value
// is a fresh budget.
type renderBudget int

// take books n more bytes and reports whether they still fit. The caller drops
// what does not fit and stops rendering: a spent budget admits nothing else.
func (b *renderBudget) take(n int) bool {
	if b.spent() {
		return false
	}
	*b += renderBudget(n)
	if b.spent() {
		logger.Warningf("sub: rendering stopped at %d bytes, the remaining links are dropped", maxRenderedBytes)
		return false
	}
	return true
}

func (b *renderBudget) spent() bool {
	return *b > maxRenderedBytes
}

// keepLink books a rendered raw link, and its line break, against the request's budget.
func (s *SubService) keepLink(link string) bool {
	return s.rendered.take(len(link) + 1)
}

// budgetedConfig renders a client's JSON configs, keeping those the request's budget admits.
func (s *SubJsonService) budgetedConfig(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
	if subReq.rendered.spent() {
		return nil
	}
	configs := s.getConfig(subReq, inbound, client, host)
	for i, config := range configs {
		if !subReq.rendered.take(len(config)) {
			return configs[:i]
		}
	}
	return configs
}

// budgetedProxies renders a client's Clash proxies, keeping those the request's budget admits;
// the YAML is written only at the end, and its JSON form is about as long.
func (s *SubClashService) budgetedProxies(subReq *SubService, inbound *model.Inbound, client model.Client, host string) []map[string]any {
	if subReq.rendered.spent() {
		return nil
	}
	proxies := s.getProxies(subReq, inbound, client, host)
	for i, proxy := range proxies {
		text, _ := json.Marshal(proxy)
		if !subReq.rendered.take(len(text)) {
			return proxies[:i]
		}
	}
	return proxies
}
