package amneziawgnet

import (
	"encoding/json"
	"sync/atomic"
)

// bridgedPort is the port the last generated socks bridge dials; 0 before any.
var bridgedPort atomic.Int64

// BuildSocksBridge swaps an "amneziawg" outbound for its loopback socks
// form, preserving sibling keys; false = unbridgeable, fail loudly upstream.
func BuildSocksBridge(raw []byte) ([]byte, bool) {
	var ob map[string]any
	if err := json.Unmarshal(raw, &ob); err != nil {
		return nil, false
	}
	tag, _ := ob["tag"].(string)
	if tag == "" {
		return nil, false
	}
	port := EgressPort()
	settings := map[string]any{
		"address": "127.0.0.1",
		"port":    port,
		"user":    tag,
		"pass":    SocksPassword(),
	}
	bs, err := json.Marshal(settings)
	if err != nil {
		return nil, false
	}
	ob["protocol"] = "socks"
	ob["settings"] = json.RawMessage(bs)
	out, err := json.Marshal(ob)
	if err != nil {
		return nil, false
	}
	bridgedPort.Store(int64(port))
	return out, true
}

// BridgesStale reports that the egress listener holds another port than the last
// generated socks bridge dials, so Xray has to regenerate its config.
func BridgesStale() bool {
	bound, listening := GetEgressServer().boundPort()
	bridged := int(bridgedPort.Load())
	return listening && bridged != 0 && bridged != bound
}
