package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

const visionFlow = "xtls-rprx-vision"

// restoreVisionFlowForEligibleInbound re-adds the XTLS Vision flow to a VLESS
// inbound's clients that lost it earlier.
//
// clientWithInboundFlow strips Vision from a client whenever the target inbound
// is not flow-eligible at write time (e.g. an XHTTP inbound before its vlessenc
// encryption is set). Nothing restored the flow when the inbound later became
// eligible — an inbound edit stores its settings verbatim and never re-gates the
// clients — so enabling encryption on an existing XHTTP inbound left every
// client without flow, and the share links/subscriptions dropped it.
//
// This runs on the now-final inbound settings: when the inbound IS flow-eligible
// it sets flow=Vision on each client that currently has no flow but whose
// intended flow (its flow_override on a sibling inbound, via EffectiveFlowsByEmails)
// is Vision. It never invents a flow for a client that has none anywhere, and it
// never overwrites an explicit non-empty flow. Returns the rewritten settings
// JSON and whether anything changed.
func (s *InboundService) restoreVisionFlowForEligibleInbound(tx *gorm.DB, settings, streamSettings string, protocol model.Protocol) (string, bool) {
	if protocol != model.VLESS {
		return settings, false
	}
	if !inboundCanEnableTlsFlow(string(protocol), streamSettings, settings) {
		return settings, false
	}
	return rewriteSettingsClients(settings, func(clients []any) bool {
		return s.restoreVisionFlow(tx, clients)
	})
}

func (s *InboundService) restoreVisionFlow(tx *gorm.DB, clients []any) bool {
	// Collect empty-flow clients, then resolve their intended flow in one query.
	emails := make([]string, 0, len(clients))
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if flow, _ := cm["flow"].(string); flow != "" {
			continue // respect an explicit flow (Vision or otherwise)
		}
		if email, _ := cm["email"].(string); email != "" {
			emails = append(emails, email)
		}
	}
	if len(emails) == 0 {
		return false
	}
	intended, err := s.clientService.EffectiveFlowsByEmails(tx, emails)
	if err != nil {
		return false
	}
	changed := false
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if flow, _ := cm["flow"].(string); flow != "" {
			continue
		}
		email, _ := cm["email"].(string)
		if intended[email] != visionFlow {
			continue
		}
		cm["flow"] = visionFlow
		changed = true
	}
	return changed
}

func stripClientFlows(settings string) (string, bool) {
	return rewriteSettingsClients(settings, clearClientFlows)
}

func clearClientFlows(clients []any) bool {
	changed := false
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if flow, _ := cm["flow"].(string); flow != "" {
			cm["flow"] = ""
			changed = true
		}
	}
	return changed
}
