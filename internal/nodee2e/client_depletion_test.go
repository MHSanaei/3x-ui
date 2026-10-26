package nodee2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeDepletionTimestampAndRecovery(t *testing.T) {
	bin := panelBinary(t)
	master := newPanel(t, bin, "depletion-master")
	node := newPanel(t, bin, "depletion-node")
	master.start()
	node.start()
	client := vlessClient("depletion-e2e")
	client["totalGB"] = 100
	node.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("depletion", freePort(t), nil, client))
	enrollTrafficNode(t, master, node)
	read := func(p *panel) (xray.ClientTraffic, bool) {
		t.Helper()
		env, err := p.try(http.MethodGet, "/panel/api/clients/traffic/depletion-e2e", nil)
		var tr xray.ClientTraffic
		if err != nil || !env.Success {
			return tr, false
		}
		if err := json.Unmarshal(env.Obj, &tr); err != nil {
			t.Fatal(err)
		}
		return tr, true
	}
	eventually(t, settleTimeout, "master adopts zero-usage client", func() (bool, string) {
		tr, ok := read(master)
		return ok && tr.Total == 100 && tr.Up == 0, fmt.Sprintf("master traffic: %+v", tr)
	})
	node.call(http.MethodPost, "/panel/api/clients/updateTraffic/depletion-e2e", map[string]any{"upload": 100, "download": 0})
	var origin int64
	eventually(t, settleTimeout, "node records traffic depletion", func() (bool, string) {
		tr, ok := read(node)
		origin = tr.DepletedAt
		return ok && !tr.Enable && origin > 0 && tr.DepletedReason == "traffic", fmt.Sprintf("node traffic: %+v", tr)
	})
	eventually(t, settleTimeout, "master preserves node timestamp", func() (bool, string) {
		tr, ok := read(master)
		return ok && tr.DepletedAt == origin && tr.DepletedReason == "traffic", fmt.Sprintf("master traffic: %+v, origin=%d", tr, origin)
	})
	master.stop()
	master.start()
	if tr, ok := read(master); !ok || tr.DepletedAt != origin || tr.DepletedReason != "traffic" {
		t.Fatalf("restart lost node depletion: %+v", tr)
	}
	master.call(http.MethodPost, "/panel/api/clients/resetTraffic/depletion-e2e", nil)
	for _, p := range []*panel{node, master} {
		eventually(t, settleTimeout, p.name+" clears recovered depletion", func() (bool, string) {
			tr, ok := read(p)
			return ok && tr.Up == 0 && tr.Down == 0 && tr.DepletedAt == 0 && tr.DepletedReason == "", fmt.Sprintf("traffic after reset: %+v", tr)
		})
	}
}
