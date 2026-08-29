package amneziawgnet

import "encoding/json"

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
	settings := map[string]any{
		"address": "127.0.0.1",
		"port":    EgressBasePort,
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
	return out, true
}
