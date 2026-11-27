package service

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type SubLinkProvider interface {
	SubLinksForSubId(host, subId string) ([]string, error)
	LinksForClient(host string, inbound *model.Inbound, email string) []string
	LinksForInbounds(host string, inbounds []*model.Inbound) []string
}

// ClientBatchLinkProvider lets LinkProvider render all links for one client
// across multiple inbounds with one host preload and primed client metadata.
type ClientBatchLinkProvider interface {
	LinksForClientInbounds(host string, inbounds []*model.Inbound, rec *model.ClientRecord, flowByInbound map[int]string) []string
}

var (
	subLinkProviderMu         sync.RWMutex
	registeredSubLinkProvider SubLinkProvider
)

func RegisterSubLinkProvider(p SubLinkProvider) {
	subLinkProviderMu.Lock()
	defer subLinkProviderMu.Unlock()
	registeredSubLinkProvider = p
}

// subLinkProvider guards the registry: startup writes it while link requests read it.
func subLinkProvider() SubLinkProvider {
	subLinkProviderMu.RLock()
	defer subLinkProviderMu.RUnlock()
	return registeredSubLinkProvider
}

func (s *InboundService) GetSubLinks(host, subId string) ([]string, error) {
	provider := subLinkProvider()
	if provider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	return provider.SubLinksForSubId(host, subId)
}

func (s *InboundService) GetAllInboundLinks(host string, userId int) ([]string, error) {
	provider := subLinkProvider()
	if provider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	inbounds, err := s.GetInbounds(userId)
	if err != nil {
		return nil, err
	}
	return provider.LinksForInbounds(host, inbounds), nil
}

func (s *InboundService) GetAllClientLinks(host string, email string) ([]string, error) {
	if email == "" {
		return nil, common.NewError("client email is required")
	}
	provider := subLinkProvider()
	if provider == nil {
		return nil, common.NewError("sub link provider not registered")
	}
	rec, err := s.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	var linkRows []struct {
		InboundId    int
		FlowOverride string
	}
	if err := database.GetDB().Table("client_inbounds").
		Select("inbound_id, flow_override").
		Where("client_id = ?", rec.Id).
		Order("inbound_id ASC").
		Scan(&linkRows).Error; err != nil {
		return nil, err
	}
	if len(linkRows) == 0 {
		return nil, nil
	}
	inboundIds := make([]int, len(linkRows))
	flowByInbound := make(map[int]string, len(linkRows))
	for i, row := range linkRows {
		inboundIds[i] = row.InboundId
		if row.FlowOverride != "" {
			flowByInbound[row.InboundId] = row.FlowOverride
		}
	}

	metas, metaErrs := s.GetInboundMetaBatch(inboundIds)
	for _, ibId := range inboundIds {
		if mErr := metaErrs[ibId]; mErr != nil {
			return nil, mErr
		}
	}

	var tunnelIDs []int
	for _, ibId := range inboundIds {
		m := metas[ibId]
		if m != nil && (m.Protocol == model.WireGuard || m.Protocol == model.AmneziaWG) {
			tunnelIDs = append(tunnelIDs, ibId)
		}
	}
	var tunnelInbounds map[int]*model.Inbound
	if len(tunnelIDs) > 0 {
		var tErr error
		tunnelInbounds, tErr = s.GetInboundsByIDs(nil, tunnelIDs)
		if tErr != nil {
			return nil, tErr
		}
	}

	inbounds := make([]*model.Inbound, 0, len(inboundIds))
	for _, ibId := range inboundIds {
		if tunnelIb := tunnelInbounds[ibId]; tunnelIb != nil {
			inbounds = append(inbounds, tunnelIb)
			continue
		}
		inbounds = append(inbounds, metas[ibId].ToInboundShell())
	}

	if batchProvider, ok := provider.(ClientBatchLinkProvider); ok {
		return batchProvider.LinksForClientInbounds(host, inbounds, rec, flowByInbound), nil
	}

	var links []string
	for _, inbound := range inbounds {
		links = append(links, provider.LinksForClient(host, inbound, email)...)
	}
	return links, nil
}
