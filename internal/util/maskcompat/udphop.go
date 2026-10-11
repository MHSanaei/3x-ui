package maskcompat

import (
	"maps"
	"slices"
	"strings"
)

// UpgradeLegacyUDPHop moves the quicParams.udpHop xray-core 26.9.9 ignores into an intervalRemote
// "udphop" mask, rewriting only finalmask's own keys: a merged finalmask shares quicParams and udp.
func UpgradeLegacyUDPHop(finalmask any) bool {
	fm, _ := finalmask.(map[string]any)
	quicParams, _ := fm["quicParams"].(map[string]any)
	legacy, found := quicParams["udpHop"]
	if !found {
		return false
	}
	restQuic := maps.Clone(quicParams)
	delete(restQuic, "udpHop")
	if len(restQuic) == 0 {
		delete(fm, "quicParams")
	} else {
		fm["quicParams"] = restQuic
	}

	hop, _ := legacy.(map[string]any)
	ports := hop["ports"]
	if isEmptyHopValue(ports) || hasUDPHopMask(fm) {
		return true
	}
	settings := map[string]any{"mode": "intervalRemote", "remotePorts": ports}
	if interval := hop["interval"]; !isEmptyHopValue(interval) {
		settings["interval"] = interval
	}
	masks, _ := fm["udp"].([]any)
	fm["udp"] = append(slices.Clip(masks), map[string]any{"type": "udphop", "settings": settings})
	return true
}

func hasUDPHopMask(fm map[string]any) bool {
	masks, _ := fm["udp"].([]any)
	for _, entry := range masks {
		mask, _ := entry.(map[string]any)
		if maskType, _ := mask["type"].(string); strings.EqualFold(maskType, "udphop") {
			return true
		}
	}
	return false
}

func isEmptyHopValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case float64:
		return v == 0
	}
	return false
}
