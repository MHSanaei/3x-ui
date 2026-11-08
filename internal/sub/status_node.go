package sub

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

type statusNode struct {
	remark  string
	blocked bool
}

func (n statusNode) endpoint() (string, int) {
	if n.blocked {
		// A rejected device must not reach an existing local SOCKS proxy.
		return "192.0.2.1", 1
	}
	return "127.0.0.1", 1080
}

func (n statusNode) link() string {
	address, port := n.endpoint()
	return fmt.Sprintf("socks://%s:%d#%s", address, port, encodeURIComponent(n.remark))
}

func (n statusNode) clashProxy() map[string]any {
	address, port := n.endpoint()
	return map[string]any{
		"name": n.remark, "type": "socks5", "server": address, "port": port,
	}
}

func (n statusNode) outbound() json_util.RawMessage {
	outbound := map[string]any{
		"tag": "proxy", "protocol": "blackhole", "settings": map[string]any{},
	}
	if !n.blocked {
		address, port := n.endpoint()
		outbound["protocol"] = "socks"
		outbound["settings"] = map[string]any{
			"servers": []any{map[string]any{"address": address, "port": port}},
		}
	}
	raw, _ := json.Marshal(outbound)
	return raw
}
