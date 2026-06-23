package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
// intended flow (its flow_override on a sibling inbound, via EffectiveFlowByEmail)
// is Vision. It never invents a flow for a client that has none anywhere, and it
// never overwrites an explicit non-empty flow. Returns the rewritten settings
// JSON and whether anything changed.
func (s *InboundService) restoreVisionFlowForEligibleInbound(settings, streamSettings string, protocol model.Protocol) (string, bool) {
	if protocol != model.VLESS {
		return settings, false
	}
	if !inboundCanEnableTlsFlow(string(protocol), streamSettings, settings) {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	clients, ok := parsed["clients"].([]any)
	if !ok || len(clients) == 0 {
		return settings, false
	}
	changed := false
	for i := range clients {
		cm, ok := clients[i].(map[string]any)
		if !ok {
			continue
		}
		if flow, _ := cm["flow"].(string); flow != "" {
			continue // respect an explicit flow (Vision or otherwise)
		}
		email, _ := cm["email"].(string)
		if email == "" {
			continue
		}
		intended, err := s.clientService.EffectiveFlowByEmail(nil, email)
		if err != nil || intended != visionFlow {
			continue
		}
		cm["flow"] = visionFlow
		clients[i] = cm
		changed = true
	}
	if !changed {
		return settings, false
	}
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}
