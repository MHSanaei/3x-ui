package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	json_util "github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// transformAmneziaWGOutbounds rewrites every "amneziawg" outbound in the
// generated config into a plain socks outbound with the same tag, pointed at
// the panel's own loopback SOCKS5 egress server (internal/amneziawgnet's
// GetEgressServer, authenticating the outbound tag as username). Xray-core
// has no amneziawg proxy -- OutboundConfigs is handed to it verbatim --
// while the embedded amneziawg-go client device lives in THIS process; the
// socks swap is what routes Xray's traffic through that device. The stored
// template is never touched: this runs on the freshly-unmarshalled copy
// inside GetXrayConfig, so hot-diff sees only real, appliable changes.
func transformAmneziaWGOutbounds(cfg *xray.Config) {
	if len(cfg.OutboundConfigs) == 0 {
		return
	}
	var outbounds []json.RawMessage
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		return
	}
	changed := false
	for i, raw := range outbounds {
		var probe struct {
			Protocol string `json:"protocol"`
			Tag      string `json:"tag"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil || probe.Protocol != "amneziawg" {
			continue
		}
		settings := map[string]any{
			"addr": "127.0.0.1",
			"port": amneziawgnet.EgressBasePort,
			"user": probe.Tag,
			"pass": amneziawgnet.SocksPassword(),
		}
		bs, err := json.Marshal(settings)
		if err != nil {
			continue
		}
		replacement, err := json.Marshal(map[string]any{
			"protocol": "socks",
			"tag":      probe.Tag,
			"settings": json.RawMessage(bs),
		})
		if err != nil {
			continue
		}
		outbounds[i] = replacement
		changed = true
	}
	if !changed {
		return
	}
	bs, err := json.Marshal(outbounds)
	if err != nil {
		return
	}
	cfg.OutboundConfigs = json_util.RawMessage(bs)
}
