package nodee2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodePanelRestartPreservesNodeAndMaster(t *testing.T) {
	bin := panelBinary(t)
	master := newPanel(t, bin, "restart-master")
	node := newPanel(t, bin, "restart-node")
	linkToken := node.mintToken("restart-link", "node-admin")
	master.start()
	node.start()
	port := freePort(t)
	node.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("survives-restart", port, nil))
	var target struct {
		Id int `json:"id"`
	}
	obj := master.call(http.MethodPost, "/panel/api/nodes/add", map[string]any{
		"name": "restart-target", "scheme": "http", "address": "127.0.0.1", "port": node.port,
		"basePath": "/", "apiToken": linkToken, "enable": true, "allowPrivateAddress": true,
	})
	if err := json.Unmarshal(obj, &target); err != nil || target.Id <= 0 {
		t.Fatalf("node enrollment = %s: %v", obj, err)
	}
	master.call(http.MethodPost, fmt.Sprintf("/panel/api/nodes/restartPanel/%d", target.Id), nil)
	eventually(t, settleTimeout, "the selected node completes its panel restart", func() (bool, string) {
		log, err := os.ReadFile(filepath.Join(node.dir, "stdout.log"))
		if err != nil {
			return false, err.Error()
		}
		if !strings.Contains(string(log), "Sub server restarted successfully.") {
			return false, "node restart not completed"
		}
		env, err := node.try(http.MethodGet, "/panel/api/server/status", nil)
		return err == nil && env.Success, fmt.Sprintf("status=%v error=%v", env, err)
	})
	if ib, ok := node.inboundOnPort(port); !ok || ib.Remark != "survives-restart" {
		t.Fatalf("node inbound after restart = %+v, found=%v", ib, ok)
	}
	master.call(http.MethodGet, "/panel/api/server/status", nil)
	log, err := os.ReadFile(filepath.Join(master.dir, "stdout.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(log), "Web server restarted successfully.") {
		t.Fatal("restarting the node also restarted the master")
	}
}
