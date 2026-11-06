package nodee2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestTunnelNodeSync(t *testing.T) {
	bin := panelBinary(t)
	master := newPanel(t, bin, "tunnel-master")
	node := newPanel(t, bin, "tunnel-node")
	master.start()
	node.start()
	target, _ := url.Parse(node.url())
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	defer proxy.Close()
	address, _ := url.Parse(proxy.URL)
	port, _ := strconv.Atoi(address.Port())
	fingerprint := sha256.Sum256(proxy.Certificate().Raw)
	token := node.mintToken("tunnel-master", "node-sync")
	var enrolled struct {
		ID int `json:"id"`
	}
	raw := master.call(http.MethodPost, "/panel/api/nodes/add", map[string]any{"name": "tunnel node", "scheme": "https", "address": "127.0.0.1", "port": port, "basePath": "/", "apiToken": token, "enable": true, "allowPrivateAddress": true, "tlsVerifyMode": "pin", "pinnedCertSha256": hex.EncodeToString(fingerprint[:]), "inboundSyncMode": "selected", "inboundTags": []string{}})
	if err := json.Unmarshal(raw, &enrolled); err != nil || enrolled.ID == 0 {
		t.Fatalf("enroll node: %v %s", err, raw)
	}
	request := service.TunnelMutation{Name: "before offline", NodeID: &enrolled.ID, Secret: "e2e-private-key", Config: wattunnel.Config{Role: "server", Carrier: "tcp", Endpoint: "192.0.2.1", Port: 54444, Ports: []wattunnel.Port{{Listen: 443, Target: 443, Protocol: "tcp"}}}}
	raw = master.call(http.MethodPost, "/panel/api/tunnels/add", request)
	var view service.TunnelView
	if err := json.Unmarshal(raw, &view); err != nil || view.ID == 0 {
		t.Fatalf("create tunnel: %v %s", err, raw)
	}
	list := func() []service.TunnelView {
		var views []service.TunnelView
		raw := node.call(http.MethodGet, "/panel/api/tunnels/list", nil)
		if strings.Contains(string(raw), "e2e-private-key") {
			t.Fatal("node list leaked key")
		}
		if err := json.Unmarshal(raw, &views); err != nil {
			t.Fatal(err)
		}
		return views
	}
	eventually(t, settleTimeout, "stopped tunnel reaches node", func() (bool, string) {
		rows := list()
		return len(rows) == 1 && rows[0].Managed && !rows[0].Enabled, fmt.Sprintf("node rows: %+v", rows)
	})
	node.stop()
	request.Name = "changed while offline"
	request.Secret = ""
	request.Revision = view.Revision
	raw = master.call(http.MethodPost, "/panel/api/tunnels/update/"+strconv.Itoa(view.ID), request)
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	node.start()
	eventually(t, settleTimeout, "offline change reconciles", func() (bool, string) {
		rows := list()
		return len(rows) == 1 && rows[0].Name == request.Name, fmt.Sprintf("node rows: %+v", rows)
	})
	node.stop()
	master.call(http.MethodPost, "/panel/api/tunnels/del/"+strconv.Itoa(view.ID), nil)
	master.stop()
	master.start()
	node.start()
	eventually(t, settleTimeout, "deletion survives master restart and node outage", func() (bool, string) { rows := list(); return len(rows) == 0, fmt.Sprintf("node rows: %+v", rows) })
}
