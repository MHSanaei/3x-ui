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

// NormalizeUDPHop keeps the last udphop mask alone at the end of udp: the core reverses the list
// and refuses ("incorrect index") a mask that dials its own socket anywhere but the socket end.
func NormalizeUDPHop(finalmask any) bool {
	fm, _ := finalmask.(map[string]any)
	masks, _ := fm["udp"].([]any)
	last, hops := -1, 0
	for i, mask := range masks {
		if isUDPHopMask(mask) {
			last, hops = i, hops+1
		}
	}
	if last < 0 || (hops == 1 && last == len(masks)-1) {
		return false
	}
	kept := make([]any, 0, len(masks)) // hops >= 1, so the re-appended hop still fits
	for _, mask := range masks {
		if !isUDPHopMask(mask) {
			kept = append(kept, mask)
		}
	}
	fm["udp"] = append(kept, masks[last])
	return true
}

// WithoutUDPHop returns finalmask minus its udphop masks, for a default that yields to a hop
// range already set; finalmask itself is left untouched.
func WithoutUDPHop(finalmask map[string]any) map[string]any {
	masks, _ := finalmask["udp"].([]any)
	if !slices.ContainsFunc(masks, isUDPHopMask) {
		return finalmask
	}
	out := maps.Clone(finalmask)
	out["udp"] = slices.DeleteFunc(slices.Clone(masks), isUDPHopMask)
	return out
}

// HasUDPHopMask reports whether finalmask already dials through a udphop mask.
func HasUDPHopMask(finalmask any) bool {
	fm, _ := finalmask.(map[string]any)
	return hasUDPHopMask(fm)
}

func hasUDPHopMask(fm map[string]any) bool {
	masks, _ := fm["udp"].([]any)
	return slices.ContainsFunc(masks, isUDPHopMask)
}

func isUDPHopMask(entry any) bool {
	mask, _ := entry.(map[string]any)
	maskType, _ := mask["type"].(string)
	return strings.EqualFold(maskType, "udphop")
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
