package nodee2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"
)

// TestNodeSync walks one master/node pair per enrollment scope through every
// operation that must converge onto the node. Each subtest names its invariant.
func TestNodeSync(t *testing.T) {
	bin := panelBinary(t)
	for _, scope := range []string{"admin", "node-sync"} {
		t.Run("enrolled with "+scope+" token", func(t *testing.T) {
			t.Parallel()
			runNodeSyncScenarios(t, bin, scope)
		})
	}
}

type pair struct {
	t      *testing.T
	master *panel
	node   *panel
	nodeID int
}

func (pr *pair) nodeInbound(port int) (inboundView, bool) { return pr.node.inboundOnPort(port) }

func (pr *pair) masterInbound(port int) (inboundView, bool) {
	for _, ib := range pr.master.inbounds() {
		if ib.Port == port && ib.NodeID != nil && *ib.NodeID == pr.nodeID {
			return ib, true
		}
	}
	return inboundView{}, false
}

func (pr *pair) waitNode(what string, port int, ok func(inboundView) bool) {
	pr.t.Helper()
	eventually(pr.t, settleTimeout, what, func() (bool, string) {
		ib, found := pr.nodeInbound(port)
		if !found {
			return ok(inboundView{}) && false, fmt.Sprintf("node has no inbound on %d", port)
		}
		return ok(ib), fmt.Sprintf("node inbound %d: enable=%v remark=%q emails=%v", port, ib.Enable, ib.Remark, ib.emails())
	})
}

func (pr *pair) waitNodeAbsent(what string, port int) {
	pr.t.Helper()
	eventually(pr.t, settleTimeout, what, func() (bool, string) {
		ib, found := pr.nodeInbound(port)
		return !found, fmt.Sprintf("node still has inbound %d with %v", port, ib.emails())
	})
}

func (pr *pair) bulkAttach(emails []string, masterInboundID int) {
	pr.t.Helper()
	var res struct {
		Attached []string `json:"attached"`
		Errors   []string `json:"errors"`
	}
	obj := pr.master.call(http.MethodPost, "/panel/api/clients/bulkAttach", map[string]any{"emails": emails, "inboundIds": []int{masterInboundID}})
	if err := json.Unmarshal(obj, &res); err != nil {
		pr.t.Fatalf("decode bulkAttach: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Attached) != len(emails) {
		pr.t.Fatalf("bulkAttach attached=%d/%d errors=%v", len(res.Attached), len(emails), res.Errors)
	}
}

func emailRange(prefix string, from, to int) []string {
	out := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		out = append(out, prefix+strconv.Itoa(i))
	}
	return out
}

func hasAll(have []string, want ...string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

func runNodeSyncScenarios(t *testing.T, bin, scope string) {
	master := newPanel(t, bin, "master")
	node := newPanel(t, bin, "node")
	linkToken := node.mintToken("master-link", scope)
	master.start()
	node.start()
	pr := &pair{t: t, master: master, node: node}

	var (
		adoptedPort   = freePort(t)
		madePort      = freePort(t)
		lostPort      = freePort(t)
		droppedPort   = freePort(t)
		offlinePort   = freePort(t)
		unmanagedPort = freePort(t)
		localPort     = freePort(t)
	)
	node.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("pre-existing", adoptedPort, nil))

	local := master.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("local-pool", localPort, nil))
	var localIb inboundView
	_ = json.Unmarshal(local, &localIb)
	for _, email := range emailRange("p", 1, 45) {
		master.call(http.MethodPost, "/panel/api/clients/add", map[string]any{
			"client": map[string]any{"email": email, "enable": true}, "inboundIds": []int{localIb.Id},
		})
	}

	var nodeView struct {
		Id int `json:"id"`
	}
	obj := master.call(http.MethodPost, "/panel/api/nodes/add", map[string]any{
		"name": "n1", "scheme": "http", "address": "127.0.0.1", "port": node.port, "basePath": "/",
		"apiToken": linkToken, "enable": true, "allowPrivateAddress": true,
	})
	if err := json.Unmarshal(obj, &nodeView); err != nil || nodeView.Id == 0 {
		t.Fatalf("decode node add: %v (%s)", err, obj)
	}
	pr.nodeID = nodeView.Id

	var adoptedID, madeID int
	t.Run("an inbound already on the node is adopted by the master", func(t *testing.T) {
		pr.t = t
		eventually(t, settleTimeout, "master adopts the node inbound", func() (bool, string) {
			ib, ok := pr.masterInbound(adoptedPort)
			adoptedID = ib.Id
			return ok, "not adopted yet"
		})
	})
	if adoptedID == 0 {
		t.Fatal("no adopted inbound; later scenarios depend on it")
	}

	t.Run("an inbound created on the master for the node lands there with its clients", func(t *testing.T) {
		pr.t = t
		obj := master.call(http.MethodPost, "/panel/api/inbounds/add",
			vlessInbound("made-on-master", madePort, &pr.nodeID, vlessClient("m1"), vlessClient("m2")))
		var ib inboundView
		_ = json.Unmarshal(obj, &ib)
		madeID = ib.Id
		pr.waitNode("node holds the master-made inbound", madePort, func(ib inboundView) bool {
			return hasAll(ib.emails(), "m1", "m2")
		})
	})

	t.Run("editing the inbound on the master updates the node and keeps its clients", func(t *testing.T) {
		pr.t = t
		body := vlessInbound("renamed-on-master", madePort, &pr.nodeID)
		body["settings"] = `{"decryption":"none"}`
		master.call(http.MethodPost, "/panel/api/inbounds/update/"+strconv.Itoa(madeID), body)
		pr.waitNode("node shows the new remark with both clients", madePort, func(ib inboundView) bool {
			return ib.Remark == "renamed-on-master" && hasAll(ib.emails(), "m1", "m2")
		})
	})

	t.Run("attaching a few existing clients reaches the node", func(t *testing.T) {
		pr.t = t
		pr.bulkAttach([]string{"p1", "p2", "p3"}, madeID)
		pr.waitNode("node holds p1..p3", madePort, func(ib inboundView) bool {
			return hasAll(ib.emails(), "m1", "m2", "p1", "p2", "p3")
		})
	})

	t.Run("attaching more clients than the per-client push limit reaches the node", func(t *testing.T) {
		pr.t = t
		emails := emailRange("p", 4, 43)
		pr.bulkAttach(emails, adoptedID)
		pr.waitNode("node holds all 40", adoptedPort, func(ib inboundView) bool {
			return hasAll(ib.emails(), emails...)
		})
	})

	t.Run("disabling a client on the master disables it on the node", func(t *testing.T) {
		pr.t = t
		mib, _ := pr.masterInbound(madePort)
		entry := mib.client("p1")
		if entry == nil {
			t.Fatalf("master inbound has no p1: %v", mib.emails())
		}
		entry["enable"] = false
		master.call(http.MethodPost, "/panel/api/clients/update/p1", entry)
		pr.waitNode("node p1 disabled", madePort, func(ib inboundView) bool {
			c := ib.client("p1")
			return c != nil && c["enable"] == false
		})
	})

	t.Run("detaching a client from the node inbound removes it there", func(t *testing.T) {
		pr.t = t
		master.call(http.MethodPost, "/panel/api/clients/p2/detach", map[string]any{"inboundIds": []int{madeID}})
		pr.waitNode("node drops p2", madePort, func(ib inboundView) bool {
			return ib.client("p2") == nil && ib.client("p3") != nil
		})
	})

	t.Run("deleting a client on the master removes it from the node", func(t *testing.T) {
		pr.t = t
		master.call(http.MethodPost, "/panel/api/clients/del/p4", nil)
		pr.waitNode("node drops p4", adoptedPort, func(ib inboundView) bool {
			return ib.client("p4") == nil && ib.client("p5") != nil
		})
	})

	t.Run("switching the inbound off on the master switches it off on the node", func(t *testing.T) {
		pr.t = t
		master.call(http.MethodPost, "/panel/api/inbounds/setEnable/"+strconv.Itoa(madeID), map[string]any{"enable": false})
		pr.waitNode("node inbound disabled", madePort, func(ib inboundView) bool { return !ib.Enable })
	})

	t.Run("node traffic reaches the master and a master reset clears the node", func(t *testing.T) {
		pr.t = t
		node.call(http.MethodPost, "/panel/api/clients/updateTraffic/p5", map[string]any{"upload": 1000, "download": 2000})
		usage := func(p *panel) int64 {
			var tr struct{ Up, Down int64 }
			_ = json.Unmarshal(p.call(http.MethodGet, "/panel/api/clients/traffic/p5", nil), &tr)
			return tr.Up + tr.Down
		}
		eventually(t, settleTimeout, "master sees p5's node traffic", func() (bool, string) {
			u := usage(master)
			return u == 3000, fmt.Sprintf("master p5 usage %d", u)
		})
		master.call(http.MethodPost, "/panel/api/clients/resetTraffic/p5", nil)
		eventually(t, settleTimeout, "node p5 usage reset", func() (bool, string) {
			u := usage(node)
			return u == 0, fmt.Sprintf("node p5 usage %d", u)
		})
		time.Sleep(12 * time.Second)
		if u := usage(master); u != 0 {
			t.Fatalf("master p5 usage %d after reset settled, want 0", u)
		}
	})

	t.Run("an inbound deleted on the node is removed from the master too", func(t *testing.T) {
		pr.t = t
		master.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("deleted-on-node", lostPort, &pr.nodeID, vlessClient("l1")))
		pr.waitNode("node holds the inbound", lostPort, func(ib inboundView) bool { return ib.client("l1") != nil })
		nib, _ := pr.nodeInbound(lostPort)
		node.call(http.MethodPost, "/panel/api/inbounds/del/"+strconv.Itoa(nib.Id), nil)
		eventually(t, settleTimeout, "master mirrors the node-side delete (#6219)", func() (bool, string) {
			_, still := pr.masterInbound(lostPort)
			return !still, "master still has the inbound"
		})
	})

	t.Run("deleting the inbound on the master removes it from the node", func(t *testing.T) {
		pr.t = t
		obj := master.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("deleted-on-master", droppedPort, &pr.nodeID, vlessClient("d1")))
		var ib inboundView
		_ = json.Unmarshal(obj, &ib)
		pr.waitNode("node holds the inbound", droppedPort, func(ib inboundView) bool { return ib.client("d1") != nil })
		master.call(http.MethodPost, "/panel/api/inbounds/del/"+strconv.Itoa(ib.Id), nil)
		pr.waitNodeAbsent("node drops the inbound", droppedPort)
	})

	t.Run("a change made while the node is down reaches it once it is back", func(t *testing.T) {
		pr.t = t
		node.stop()
		pr.bulkAttach([]string{"p44"}, adoptedID)
		node.start()
		pr.waitNode("node holds p44 after restart", adoptedPort, func(ib inboundView) bool {
			return ib.client("p44") != nil
		})
	})

	t.Run("an inbound the node lost while down is re-created with the master's pending change", func(t *testing.T) {
		pr.t = t
		nib, ok := pr.nodeInbound(adoptedPort)
		if !ok {
			t.Fatal("node has no adopted inbound to lose")
		}
		node.stop()
		node.deleteInboundRow(nib.Id)
		pr.bulkAttach([]string{"p45"}, adoptedID)
		node.start()
		pr.waitNode("node re-creates the inbound with p44 and p45", adoptedPort, func(ib inboundView) bool {
			return ib.client("p44") != nil && ib.client("p45") != nil
		})
	})

	t.Run("an inbound created and edited while the node is down lands once it is back", func(t *testing.T) {
		pr.t = t
		node.stop()
		obj := master.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("made-while-down", offlinePort, &pr.nodeID, vlessClient("o1")))
		var ib inboundView
		_ = json.Unmarshal(obj, &ib)
		body := vlessInbound("edited-while-down", offlinePort, &pr.nodeID)
		body["settings"] = `{"decryption":"none"}`
		master.call(http.MethodPost, "/panel/api/inbounds/update/"+strconv.Itoa(ib.Id), body)
		node.start()
		pr.waitNode("node holds the edited inbound with o1", offlinePort, func(ib inboundView) bool {
			return ib.Remark == "edited-while-down" && ib.client("o1") != nil
		})
	})

	t.Run("a change made while the node is disabled on the master lands once it is re-enabled", func(t *testing.T) {
		pr.t = t
		nodePath := "/panel/api/nodes/setEnable/" + strconv.Itoa(pr.nodeID)
		master.call(http.MethodPost, nodePath, map[string]any{"enable": false})
		pr.bulkAttach([]string{"p6"}, madeID)
		time.Sleep(6 * time.Second)
		if ib, _ := pr.nodeInbound(madePort); ib.client("p6") != nil {
			t.Fatal("a disabled node received a push")
		}
		master.call(http.MethodPost, nodePath, map[string]any{"enable": true})
		pr.waitNode("node holds p6 after re-enable", madePort, func(ib inboundView) bool { return ib.client("p6") != nil })
	})

	t.Run("selected sync mode leaves the node's unselected inbounds alone", func(t *testing.T) {
		pr.t = t
		var selected []string
		for _, ib := range master.inbounds() {
			if ib.NodeID != nil && *ib.NodeID == pr.nodeID {
				selected = append(selected, ib.Tag)
			}
		}
		master.call(http.MethodPost, "/panel/api/nodes/update/"+strconv.Itoa(pr.nodeID), map[string]any{
			"name": "n1", "scheme": "http", "address": "127.0.0.1", "port": node.port, "basePath": "/",
			"enable": true, "allowPrivateAddress": true, "inboundSyncMode": "selected", "inboundTags": selected,
		})
		node.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("node-only", unmanagedPort, nil, vlessClient("u1")))
		pr.bulkAttach([]string{"p7"}, madeID)
		pr.waitNode("selected inbound still converges", madePort, func(ib inboundView) bool { return ib.client("p7") != nil })
		time.Sleep(12 * time.Second)
		if _, adopted := pr.masterInbound(unmanagedPort); adopted {
			t.Fatal("master adopted an unselected node inbound")
		}
		if ib, ok := pr.nodeInbound(unmanagedPort); !ok || ib.client("u1") == nil {
			t.Fatal("reconcile swept or rewrote an unselected node inbound")
		}
	})
}
