package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type LinkProvider struct {
	settingService service.SettingService
}

func NewLinkProvider() *LinkProvider {
	return &LinkProvider{}
}

func (p *LinkProvider) build(host string) *SubService {
	remarkTemplate, _ := p.settingService.GetRemarkTemplate()
	svc := NewSubService(remarkTemplate)
	svc.PrepareForRequest(host)
	return svc
}

func (p *LinkProvider) SubLinksForSubId(host, subId string) ([]string, error) {
	svc := p.build(host)
	links, _, _, _, err := svc.GetSubs(subId, host)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, splitLinkLines(l)...)
	}
	return out, nil
}

func (p *LinkProvider) LinksForClient(host string, inbound *model.Inbound, email string) []string {
	svc := p.build(host)
	return p.linksForClientWithService(svc, inbound, email)
}

func (p *LinkProvider) LinksForClientInbounds(host string, inbounds []*model.Inbound, rec *model.ClientRecord, flowByInbound map[int]string) []string {
	if len(inbounds) == 0 || rec == nil {
		return nil
	}
	svc := p.build(host)
	_ = svc.loadSubscriptionHosts(inbounds)
	baseClient := rec.ToClient()
	for _, inbound := range inbounds {
		if inbound.Protocol != model.WireGuard && inbound.Protocol != model.AmneziaWG {
			c := *baseClient
			c.Flow = flowByInbound[inbound.Id]
			svc.primeLinkClients(inbound.Id, []model.Client{c}, true)
		}
	}
	var out []string
	for _, inbound := range inbounds {
		out = append(out, p.linksForClientWithService(svc, inbound, rec.Email)...)
	}
	return out
}

func (p *LinkProvider) linksForClientWithService(svc *SubService, inbound *model.Inbound, email string) []string {
	svc.projectThroughFallbackMaster(inbound)
	baseEps, closed := svc.hostEndpoints(inbound, "raw")
	if closed {
		return nil
	}
	if client, ok := svc.clientForLink(inbound, email); ok {
		effectiveEps, bound := svc.selectHostEndpoints(inbound, client, "raw", baseEps)
		if bound && len(effectiveEps) == 0 {
			return nil
		}
		if len(effectiveEps) > 0 {
			if link := svc.linkFromHosts(inbound, client, effectiveEps); link != "" {
				return splitLinkLines(link)
			}
		}
	}
	return splitLinkLines(svc.GetLink(inbound, email))
}

func (p *LinkProvider) LinksForInbounds(host string, inbounds []*model.Inbound) []string {
	svc := p.build(host)
	var out []string
	for _, inbound := range inbounds {
		out = append(out, svc.inboundLinks(inbound)...)
	}
	return out
}

func splitLinkLines(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
