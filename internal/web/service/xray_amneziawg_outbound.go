package service

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	json_util "github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// transformAmneziaWGOutbounds swaps each template "amneziawg" outbound for
// its socks bridge; unbridgeable entries fail generation rather than skip.
func transformAmneziaWGOutbounds(cfg *xray.Config) error {
	if len(cfg.OutboundConfigs) == 0 {
		return nil
	}
	var outbounds []json.RawMessage
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		return err
	}
	changed := false
	for i, raw := range outbounds {
		if !amneziawg.IsAmneziaWGOutbound(raw) {
			continue
		}
		var probe struct {
			Tag string `json:"tag"`
		}
		tagErr := json.Unmarshal(raw, &probe)
		replacement, ok := amneziawgnet.BuildSocksBridge(raw)
		if !ok {
			if tagErr != nil {
				return fmt.Errorf("amneziawg outbound %d: unreadable tag: %w", i, tagErr)
			}
			return fmt.Errorf("amneziawg outbound %d (%q): cannot bridge: tag must be a non-empty string", i, probe.Tag)
		}
		outbounds[i] = replacement
		changed = true
	}
	if !changed {
		return nil
	}
	bs, err := json.Marshal(outbounds)
	if err != nil {
		return err
	}
	cfg.OutboundConfigs = json_util.RawMessage(bs)
	return nil
}
