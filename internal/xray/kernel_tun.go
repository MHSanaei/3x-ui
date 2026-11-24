package xray

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// ForceUserspaceWireGuardTun sets noKernelTun on every WireGuard outbound and returns the tags
// it changed. Values stay raw so the template's other keys reach the core byte for byte.
func ForceUserspaceWireGuardTun(raw json_util.RawMessage) (json_util.RawMessage, []string) {
	var outbounds []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outbounds); err != nil {
		return raw, nil
	}
	var moved []string
	for _, ob := range outbounds {
		var protocol string
		if json.Unmarshal(ob["protocol"], &protocol) != nil || !strings.EqualFold(protocol, "wireguard") {
			continue
		}
		var settings map[string]json.RawMessage
		if json.Unmarshal(ob["settings"], &settings) != nil || settings == nil {
			continue
		}
		wasUserspace := string(settings["noKernelTun"]) == "true"
		settings["noKernelTun"] = json.RawMessage("true")
		rewritten, err := json.Marshal(settings)
		if err != nil {
			continue
		}
		ob["settings"] = rewritten
		if !wasUserspace {
			var tag string
			_ = json.Unmarshal(ob["tag"], &tag)
			moved = append(moved, tag)
		}
	}
	rewritten, err := json.Marshal(outbounds)
	if err != nil {
		return raw, nil
	}
	return rewritten, moved
}
