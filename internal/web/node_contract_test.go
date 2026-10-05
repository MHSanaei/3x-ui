package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// nodeUnderContract serves the production router as a node and records every
// request the node refused for auth or scope.
type nodeUnderContract struct {
	srv     *httptest.Server
	mu      sync.Mutex
	refused []string
}

func startContractNode(t *testing.T) *nodeUnderContract {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	prevMgr := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(prevMgr) })

	previous := global.GetWebServer()
	s := NewServer()
	s.cron = cron.New(cron.WithLocation(time.Local), cron.WithSeconds())
	global.SetWebServer(s)
	t.Cleanup(func() {
		s.cancel()
		global.SetWebServer(previous)
	})
	engine, err := s.initRouter()
	if err != nil {
		t.Fatalf("initRouter: %v", err)
	}
	n := &nodeUnderContract{}
	n.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		engine.ServeHTTP(rec, r)
		if rec.status == http.StatusUnauthorized || rec.status == http.StatusForbidden {
			n.mu.Lock()
			n.refused = append(n.refused, r.Method+" "+r.URL.Path+" -> "+strconv.Itoa(rec.status))
			n.mu.Unlock()
		}
	}))
	t.Cleanup(n.srv.Close)
	return n
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (n *nodeUnderContract) takeRefused() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.refused
	n.refused = nil
	return out
}

func (n *nodeUnderContract) masterWithToken(t *testing.T, scope string) *runtime.Remote {
	t.Helper()
	token := "contract-" + scope
	if err := database.GetDB().Create(&model.ApiToken{
		Name: "master-" + scope, Token: crypto.HashTokenSHA256(token), Enabled: true, Scope: scope,
	}).Error; err != nil {
		t.Fatalf("seed %s token: %v", scope, err)
	}
	u, _ := url.Parse(n.srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return runtime.NewRemote(&model.Node{
		Id: 1, Name: "contract-node", Scheme: "http", Address: u.Hostname(), Port: port,
		BasePath: "/", ApiToken: token, Enable: true, AllowPrivateAddress: true,
	}, nil)
}

func nodeRow(t *testing.T, tag string) (*model.Inbound, bool) {
	t.Helper()
	var ib model.Inbound
	err := database.GetDB().Where("tag = ?", tag).First(&ib).Error
	return &ib, err == nil
}

func nodeTraffic(t *testing.T, email string) int64 {
	t.Helper()
	var ct xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&ct).Error; err != nil {
		t.Fatalf("client_traffics %s: %v", email, err)
	}
	return ct.Up + ct.Down
}

func seedNodeTraffic(t *testing.T, emails ...string) {
	t.Helper()
	for _, e := range emails {
		if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", e).
			Updates(map[string]any{"up": 100, "down": 200}).Error; err != nil {
			t.Fatalf("seed traffic %s: %v", e, err)
		}
	}
	if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", contractTag).
		Updates(map[string]any{"up": 100, "down": 200}).Error; err != nil {
		t.Fatalf("seed inbound traffic: %v", err)
	}
}

const contractTag = "in-51001-tcp"

func masterInbound(remark string, enable bool, clients ...string) *model.Inbound {
	entries := make([]string, 0, len(clients))
	for i, email := range clients {
		entries = append(entries, `{"email":"`+email+`","enable":true,"subId":"s-`+email+
			`","id":"0b6d5c2e-7c1a-4f4e-9d3b-00000000000`+strconv.Itoa(i)+`"}`)
	}
	return &model.Inbound{
		Tag: contractTag, Remark: remark, Enable: enable, Port: 51001, Protocol: model.VLESS,
		Settings:       `{"clients":[` + strings.Join(entries, ",") + `],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
		Sniffing:       `{}`,
	}
}

func nodeEmails(t *testing.T) []string {
	t.Helper()
	ib, ok := nodeRow(t, contractTag)
	if !ok {
		t.Fatal("node has no contract inbound")
	}
	clients, err := (&service.InboundService{}).GetClients(ib)
	if err != nil {
		t.Fatalf("parse node clients: %v", err)
	}
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		emails = append(emails, c.Email)
	}
	return emails
}

// TestMasterNodeContract sends every node call the master makes through the production
// router, once per enrollment scope; UpdatePanel is excluded from node-sync on purpose.
func TestMasterNodeContract(t *testing.T) {
	for _, scope := range []string{model.ApiScopeAdmin, model.ApiScopeNodeSync} {
		t.Run(scope, func(t *testing.T) {
			node := startContractNode(t)
			master := node.masterWithToken(t, scope)
			ctx := context.Background()

			cells := []struct {
				name   string
				covers []string
				run    func() error
				check  func(t *testing.T)
			}{
				{"AddInbound creates the inbound with its clients", []string{"AddInbound"}, func() error {
					return master.AddInbound(ctx, masterInbound("added", true, "c0", "c1"))
				}, func(t *testing.T) {
					if got := nodeEmails(t); strings.Join(got, ",") != "c0,c1" {
						t.Fatalf("node clients = %v, want c0,c1", got)
					}
				}},
				{"UpdateInbound applies remark, clients and enable", []string{"UpdateInbound", "AddUser", "RemoveUser", "ReconcileInbound"}, func() error {
					ib := masterInbound("updated", false, "c0", "c1", "c2")
					if err := master.AddUser(ctx, ib, nil); err != nil {
						return err
					}
					if err := master.RemoveUser(ctx, ib, ""); err != nil {
						return err
					}
					if _, err := master.ReconcileInbound(ctx, ib, true); err != nil {
						return err
					}
					return master.UpdateInbound(ctx, ib, ib)
				}, func(t *testing.T) {
					ib, _ := nodeRow(t, contractTag)
					if ib.Remark != "updated" || ib.Enable {
						t.Fatalf("node remark=%q enable=%v, want updated/false", ib.Remark, ib.Enable)
					}
					if got := nodeEmails(t); strings.Join(got, ",") != "c0,c1,c2" {
						t.Fatalf("node clients = %v, want c0,c1,c2", got)
					}
				}},
				{"SetInboundSubSortIndex reaches the node", []string{"SetInboundSubSortIndex"}, func() error {
					return master.SetInboundSubSortIndex(ctx, masterInbound("updated", false), 7)
				}, func(t *testing.T) {
					if ib, _ := nodeRow(t, contractTag); ib.SubSortIndex != 7 {
						t.Fatalf("node subSortIndex = %d, want 7", ib.SubSortIndex)
					}
				}},
				{"AddClient attaches one client", []string{"AddClient"}, func() error {
					return master.AddClient(ctx, masterInbound("updated", false), model.Client{
						Email: "c3", ID: "0b6d5c2e-7c1a-4f4e-9d3b-000000000003", SubID: "s-c3", Enable: true,
					})
				}, func(t *testing.T) {
					if got := nodeEmails(t); !strings.Contains(strings.Join(got, ","), "c3") {
						t.Fatalf("node clients = %v, want c3 among them", got)
					}
				}},
				{"UpdateUser changes the client's limits", []string{"UpdateUser"}, func() error {
					return master.UpdateUser(ctx, masterInbound("updated", false), "c3", model.Client{
						Email: "c3", ID: "0b6d5c2e-7c1a-4f4e-9d3b-000000000003", SubID: "s-c3", Enable: true, TotalGB: 5 << 30,
					})
				}, func(t *testing.T) {
					var ct xray.ClientTraffic
					database.GetDB().Where("email = ?", "c3").First(&ct)
					if ct.Total != 5<<30 {
						t.Fatalf("node c3 total = %d, want %d", ct.Total, int64(5<<30))
					}
				}},
				{"ResetClientTraffic zeroes one client", []string{"ResetClientTraffic"}, func() error {
					seedNodeTraffic(t, "c0")
					return master.ResetClientTraffic(ctx, nil, "c0")
				}, func(t *testing.T) {
					if u := nodeTraffic(t, "c0"); u != 0 {
						t.Fatalf("node c0 usage = %d, want 0", u)
					}
				}},
				{"ResetClientTraffics zeroes several clients", []string{"ResetClientTraffics"}, func() error {
					seedNodeTraffic(t, "c1", "c2")
					return master.ResetClientTraffics(ctx, []string{"c1", "c2"})
				}, func(t *testing.T) {
					if u := nodeTraffic(t, "c1") + nodeTraffic(t, "c2"); u != 0 {
						t.Fatalf("node c1+c2 usage = %d, want 0", u)
					}
				}},
				{"ResetInboundTraffic zeroes the inbound", []string{"ResetInboundTraffic"}, func() error {
					seedNodeTraffic(t)
					return master.ResetInboundTraffic(ctx, masterInbound("updated", false))
				}, func(t *testing.T) {
					if ib, _ := nodeRow(t, contractTag); ib.Up+ib.Down != 0 {
						t.Fatalf("node inbound usage = %d, want 0", ib.Up+ib.Down)
					}
				}},
				{"ResetAllTraffics zeroes every inbound's counters", []string{"ResetAllTraffics"}, func() error {
					seedNodeTraffic(t)
					return master.ResetAllTraffics(ctx)
				}, func(t *testing.T) {
					if ib, _ := nodeRow(t, contractTag); ib.Up+ib.Down != 0 {
						t.Fatalf("node inbound usage = %d, want 0", ib.Up+ib.Down)
					}
				}},
				{"FetchTrafficSnapshot reads every part of the snapshot", []string{"FetchTrafficSnapshot"}, func() error {
					_, err := master.FetchTrafficSnapshot(ctx)
					return err
				}, nil},
				{"PushGlobalClientTraffics is accepted", []string{"PushGlobalClientTraffics"}, func() error {
					return master.PushGlobalClientTraffics(ctx, "master-guid", []*xray.ClientTraffic{{Email: "c0", Up: 1, Down: 2}})
				}, nil},
				{"client IP sync is accepted both ways", []string{"FetchAllClientIps", "PushAllClientIps", "FetchClientIpsByGuid"}, func() error {
					ips, err := master.FetchAllClientIps(ctx)
					if err != nil {
						return err
					}
					if err := master.PushAllClientIps(ctx, ips); err != nil {
						return err
					}
					_, err = master.FetchClientIpsByGuid(ctx)
					return err
				}, nil},
				{"host groups, descendants and web cert files are readable", []string{"FetchHostGroups", "GetDescendants", "GetWebCertFiles", "ListInboundOptions", "ListRemoteTags"}, func() error {
					if _, err := master.FetchHostGroups(ctx); err != nil {
						return err
					}
					if _, err := master.GetDescendants(ctx); err != nil {
						return err
					}
					if _, err := master.GetWebCertFiles(ctx); err != nil {
						return err
					}
					if _, err := master.ListInboundOptions(ctx); err != nil {
						return err
					}
					_, err := master.ListRemoteTags(ctx)
					return err
				}, nil},
				{"RestartXray is accepted by the node", []string{"RestartXray"}, func() error {
					// No core binary here: only the node's own restart failure may come back.
					if err := master.RestartXray(ctx); err != nil && !strings.Contains(err.Error(), "rebooting the Xray") {
						return err
					}
					return nil
				}, nil},
				{"DeleteUser detaches the client from the inbound", []string{"DeleteUser"}, func() error {
					return master.DeleteUser(ctx, masterInbound("updated", false), "c3")
				}, func(t *testing.T) {
					if got := nodeEmails(t); strings.Contains(strings.Join(got, ","), "c3") {
						t.Fatalf("node clients = %v, want c3 gone", got)
					}
				}},
				{"DeleteClient removes the client everywhere", []string{"DeleteClient"}, func() error {
					return master.DeleteClient(ctx, "c2")
				}, func(t *testing.T) {
					if got := nodeEmails(t); strings.Contains(strings.Join(got, ","), "c2") {
						t.Fatalf("node clients = %v, want c2 gone", got)
					}
				}},
				{"DelInbound removes the inbound", []string{"DelInbound"}, func() error {
					return master.DelInbound(ctx, masterInbound("updated", false))
				}, func(t *testing.T) {
					if _, ok := nodeRow(t, contractTag); ok {
						t.Fatal("node still has the inbound")
					}
				}},
			}
			covered := map[string]bool{}
			for _, c := range cells {
				for _, m := range c.covers {
					covered[m] = true
				}
			}
			assertEveryRemoteCallCovered(t, covered)
			for _, c := range cells {
				t.Run(c.name, func(t *testing.T) {
					node.takeRefused()
					if err := c.run(); err != nil {
						t.Fatalf("master call failed: %v", err)
					}
					if refused := node.takeRefused(); len(refused) != 0 {
						t.Fatalf("node refused master requests: %v", refused)
					}
					if c.check != nil {
						c.check(t)
					}
				})
			}
		})
	}
}

// Remote methods that never reach the node, or that this table must not run.
var remoteMethodsOutsideContract = map[string]string{
	"Name":                  "local label",
	"RecordAdoptedInbound":  "local fingerprint bookkeeping",
	"AdoptInboundAlias":     "local alias bookkeeping",
	"AdoptedInboundAliases": "local alias bookkeeping",
	"AdvancePushedInbound":  "local fingerprint bookkeeping",
	"ForgetPushedInbound":   "local fingerprint bookkeeping",
	"UpdatePanel":           "replaces the node binary; node-sync is denied it on purpose (#6201)",
}

// A Remote method with no cell is how activeInbounds and bulkResetTraffic
// drifted out of the node-sync allowlist unnoticed.
func assertEveryRemoteCallCovered(t *testing.T, covered map[string]bool) {
	t.Helper()
	rt := reflect.TypeOf(&runtime.Remote{})
	for i := 0; i < rt.NumMethod(); i++ {
		name := rt.Method(i).Name
		if _, skip := remoteMethodsOutsideContract[name]; skip {
			continue
		}
		if !covered[name] {
			t.Errorf("runtime.Remote.%s has no cell in TestMasterNodeContract", name)
		}
	}
}
