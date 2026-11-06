package nodee2e

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestNodeSnapshotVersionCompatibility(t *testing.T) {
	current := panelBinary(t)
	old := os.Getenv("XUI_NODE_E2E_OLD_BINARY")
	for _, version := range []struct {
		name         string
		master, node string
		compact      bool
	}{
		{"new-master-new-node", current, current, true},
		{"new-master-old-node", current, old, false},
		{"old-master-new-node", old, current, false},
		{"old-master-old-node", old, old, false},
	} {
		t.Run(version.name, func(t *testing.T) {
			if version.master == "" || version.node == "" {
				t.Skip("set XUI_NODE_E2E_OLD_BINARY to a baseline panel binary")
			}
			t.Parallel()
			master := newPanel(t, version.master, "master")
			node := newPanel(t, version.node, "node")
			token := node.mintToken("snapshot-link", "node-sync")
			master.start()
			node.start()
			var lists, compact, full, unchanged atomic.Int32
			target, err := url.Parse(node.url())
			if err != nil {
				t.Fatal(err)
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ModifyResponse = func(response *http.Response) error {
				if response.Request.URL.Path != "/panel/api/inbounds/trafficSnapshot" || response.StatusCode != http.StatusOK {
					return nil
				}
				data, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil {
					return err
				}
				response.Body = io.NopCloser(bytes.NewReader(data))
				if response.Header.Get("Content-Encoding") == "gzip" {
					reader, err := gzip.NewReader(bytes.NewReader(data))
					if err != nil {
						return err
					}
					decoded, readErr := io.ReadAll(reader)
					_ = reader.Close()
					if readErr != nil {
						return readErr
					}
					data = decoded
				}
				var env struct {
					Obj struct {
						Structure json.RawMessage `json:"structure"`
					} `json:"obj"`
				}
				if err := json.Unmarshal(data, &env); err != nil {
					return err
				}
				if len(env.Obj.Structure) > 0 && string(env.Obj.Structure) != "null" {
					full.Add(1)
				} else {
					unchanged.Add(1)
				}
				return nil
			}
			front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch req.URL.Path {
				case "/panel/api/inbounds/list":
					lists.Add(1)
				case "/panel/api/inbounds/trafficSnapshot":
					compact.Add(1)
				}
				proxy.ServeHTTP(w, req)
			}))
			t.Cleanup(front.Close)
			address, err := url.Parse(front.URL)
			if err != nil {
				t.Fatal(err)
			}
			port, err := strconv.Atoi(address.Port())
			if err != nil {
				t.Fatal(err)
			}
			inboundPort := freePort(t)
			var source inboundView
			if err := json.Unmarshal(node.call(http.MethodPost, "/panel/api/inbounds/add", vlessInbound("matrix", inboundPort, nil, vlessClient("matrix-first"))), &source); err != nil {
				t.Fatal(err)
			}
			master.call(http.MethodPost, "/panel/api/nodes/add", map[string]any{"name": "matrix-node", "scheme": "http", "address": address.Hostname(), "port": port, "basePath": "/", "apiToken": token, "enable": true, "allowPrivateAddress": true})
			eventually(t, settleTimeout, "version pair synchronizes and reaches steady state", func() (bool, string) {
				ib, ok := master.inboundOnPort(inboundPort)
				ready := lists.Load() >= 3
				if version.compact {
					ready = unchanged.Load() >= 2
				}
				return ok && hasAll(ib.emails(), "matrix-first") && ready, fmt.Sprintf("adopted=%v lists=%d compact=%d unchanged=%d", ok, lists.Load(), compact.Load(), unchanged.Load())
			})
			if !version.compact && compact.Load() != 0 {
				t.Fatalf("legacy peer received compact calls: %d", compact.Load())
			}
			before := full.Load()
			node.call(http.MethodPost, "/panel/api/clients/add", map[string]any{"client": map[string]any{"email": "matrix-added", "enable": true}, "inboundIds": []int{source.Id}})
			eventually(t, settleTimeout, "node edit after unchanged polls reaches master", func() (bool, string) {
				ib, ok := master.inboundOnPort(inboundPort)
				return ok && hasAll(ib.emails(), "matrix-first", "matrix-added"), fmt.Sprint(ib.emails())
			})
			if version.compact && full.Load() <= before {
				t.Fatal("node edit did not refresh catalog")
			}
			if version.compact {
				before = full.Load()
				node.stop()
				node.start()
				eventually(t, settleTimeout, "node restart invalidates the old catalog", func() (bool, string) { return full.Load() > before, fmt.Sprint(full.Load()) })
				before = full.Load()
				master.stop()
				master.start()
				eventually(t, settleTimeout, "master restart reacquires complete structure", func() (bool, string) { return full.Load() > before, fmt.Sprint(full.Load()) })
			}
		})
	}
}
