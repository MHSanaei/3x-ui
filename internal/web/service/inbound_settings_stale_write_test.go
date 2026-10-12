package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// commitTickBetweenReadAndWrite parks the serial writer, lets op read the
// inbound and queue its transaction, then commits tick ahead of that transaction.
func commitTickBetweenReadAndWrite(t *testing.T, tick func(tx *gorm.DB) error, op func()) {
	t.Helper()
	resetTrafficWriterForTest(t)
	StartTrafficWriter()

	parked := make(chan struct{})
	release := make(chan struct{})
	tickErr := make(chan error, 1)
	go func() {
		tickErr <- submitTrafficWrite(func() error {
			close(parked)
			<-release
			return database.GetDB().Transaction(tick)
		})
	}()
	<-parked

	opDone := make(chan struct{})
	go func() {
		defer close(opDone)
		op()
	}()
	waitTrafficWriterQueued(t)
	close(release)
	if err := <-tickErr; err != nil {
		t.Fatalf("tick: %v", err)
	}
	<-opDone
}

// seedRenewableNeighbour builds an inbound holding a healthy client X and a
// quota-disabled client Y whose auto-renew is due, as the traffic job sees them.
func seedRenewableNeighbour(t *testing.T, port int, nodeID *int) *model.Inbound {
	t.Helper()
	past := time.Now().Add(-time.Hour).UnixMilli()
	future := time.Now().Add(30 * 24 * time.Hour).UnixMilli()
	clients := []model.Client{
		{Email: "x@stale", ID: "aaaaaaaa-0000-0000-0000-00000000000a", SubID: "sub-x", Enable: true, ExpiryTime: future},
		{Email: "y@stale", ID: "aaaaaaaa-0000-0000-0000-00000000000b", SubID: "sub-y", Enable: false, Reset: 30, ExpiryTime: past, TotalGB: 1000},
	}
	ib := &model.Inbound{
		Tag: "stale-" + strconv.Itoa(port), Enable: true, Port: port, Protocol: model.VLESS,
		Settings: clientsSettings(t, clients), NodeID: nodeID,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	rows := []xray.ClientTraffic{
		{InboundId: ib.Id, Email: "x@stale", Enable: true, ExpiryTime: future},
		{InboundId: ib.Id, Email: "y@stale", Enable: false, Up: 600, Down: 400, Total: 1000, Reset: 30, ExpiryTime: past},
	}
	if err := database.GetDB().Create(&rows).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	return ib
}

func autoRenewTick(tx *gorm.DB) error {
	_, _, err := (&InboundService{}).autoRenewClients(tx, newTrafficMutationBatch())
	return err
}

// renewYTick writes the renewal autoRenewClients would commit for y@stale; it
// skips clients hosted only on a node, so the node case applies it directly.
func renewYTick(inboundId int) func(tx *gorm.DB) error {
	return func(tx *gorm.DB) error {
		var ib model.Inbound
		if err := tx.First(&ib, inboundId).Error; err != nil {
			return err
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			return err
		}
		for _, c := range settings["clients"].([]any) {
			if m := c.(map[string]any); m["email"] == "y@stale" {
				m["enable"] = true
				m["expiryTime"] = time.Now().Add(30 * 24 * time.Hour).UnixMilli()
			}
		}
		b, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return err
		}
		return tx.Model(&model.Inbound{}).Where("id = ?", inboundId).Update("settings", string(b)).Error
	}
}

func settingsClient(t *testing.T, inboundId int, email string) (model.Client, bool) {
	t.Helper()
	ib, err := (&InboundService{}).GetInbound(inboundId)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	clients, err := (&InboundService{}).GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	for _, c := range clients {
		if c.Email == email {
			return c, true
		}
	}
	return model.Client{}, false
}

func requireNeighbourRenewed(t *testing.T, inboundId int) model.Client {
	t.Helper()
	y, ok := settingsClient(t, inboundId, "y@stale")
	if !ok {
		t.Fatal("neighbour y@stale missing from settings")
	}
	if now := time.Now().UnixMilli(); !y.Enable || y.ExpiryTime <= now {
		t.Fatalf("renewed neighbour rolled back in settings: enable=%v expiryTime=%d (now %d)", y.Enable, y.ExpiryTime, now)
	}
	return y
}

type staleClientOp struct {
	name string
	// advancesNodeFingerprint: on a node inbound the op pushes per client and
	// then advances the reconcile-skip fingerprint.
	advancesNodeFingerprint bool
	run                     func(t *testing.T, ib *model.Inbound) error
}

var staleClientOps = []staleClientOp{
	{"edit", true, func(t *testing.T, ib *model.Inbound) error {
		rec := lookupClientRecord(t, "x@stale")
		edited := rec.ToClient()
		edited.Comment = "edited"
		_, err := (&ClientService{}).UpdateInboundClient(&InboundService{}, &model.Inbound{
			Id: ib.Id, Settings: clientsSettings(t, []model.Client{*edited}),
		}, "x@stale")
		return err
	}},
	{"add", true, func(t *testing.T, ib *model.Inbound) error {
		_, err := (&ClientService{}).AddInboundClient(&InboundService{}, &model.Inbound{
			Id: ib.Id, Settings: clientsSettings(t, []model.Client{{Email: "z@stale", ID: "aaaaaaaa-0000-0000-0000-00000000000c", Enable: true}}),
		})
		return err
	}},
	{"delete", true, func(t *testing.T, ib *model.Inbound) error {
		_, err := (&ClientService{}).DelInboundClientByEmail(&InboundService{}, ib.Id, "x@stale", false, true)
		return err
	}},
	{"bulk detach", true, func(t *testing.T, ib *model.Inbound) error {
		_, _, err := (&ClientService{}).BulkDetach(&InboundService{}, []string{"x@stale"}, []int{ib.Id})
		return err
	}},
	{"bulk adjust", false, func(t *testing.T, ib *model.Inbound) error {
		_, _, err := (&ClientService{}).BulkAdjust(&InboundService{}, []string{"x@stale"}, 1, 0, "", nil, "")
		return err
	}},
	{"bulk delete", false, func(t *testing.T, ib *model.Inbound) error {
		_, _, err := (&ClientService{}).BulkDelete(&InboundService{}, []string{"x@stale"}, false)
		return err
	}},
	{"bulk set enable", true, func(t *testing.T, ib *model.Inbound) error {
		_, _, err := (&ClientService{}).BulkSetEnable(&InboundService{}, []string{"x@stale"}, false)
		return err
	}},
}

// Each client op reads the inbound before queueing its write; a renewal the
// traffic writer commits in between must not be reverted to enable=false.
func TestClientOpsKeepNeighbourRenewedMidOp(t *testing.T) {
	for i, op := range staleClientOps {
		t.Run(op.name, func(t *testing.T) {
			setupBulkDB(t)
			ib := seedRenewableNeighbour(t, 23101+i, nil)
			commitTickBetweenReadAndWrite(t, autoRenewTick, func() {
				if err := op.run(t, ib); err != nil {
					t.Errorf("%s: %v", op.name, err)
				}
			})
			requireNeighbourRenewed(t, ib.Id)
		})
	}
}

// An op on the renewed client itself keeps the fields it did not change.
func TestBulkAdjustOnRenewedClientKeepsRenewal(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23120, nil)
	commitTickBetweenReadAndWrite(t, autoRenewTick, func() {
		if _, _, err := (&ClientService{}).BulkAdjust(&InboundService{}, []string{"y@stale"}, 0, 500, "", nil, ""); err != nil {
			t.Errorf("BulkAdjust: %v", err)
		}
	})
	if y := requireNeighbourRenewed(t, ib.Id); y.TotalGB != 1500 {
		t.Fatalf("y@stale totalGB = %d, want 1500 (the adjust itself was lost)", y.TotalGB)
	}
}

// The node got only the per-client push, so the skip fingerprint must not claim
// it also holds the renewal the traffic writer committed mid-op.
func TestNodeClientOpsMidRenewalStillReconcileRenewal(t *testing.T) {
	for i, op := range staleClientOps {
		if !op.advancesNodeFingerprint {
			continue
		}
		t.Run(op.name, func(t *testing.T) {
			setupBulkDB(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			t.Cleanup(srv.Close)
			u, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatalf("parse url: %v", err)
			}
			port, _ := strconv.Atoi(u.Port())
			node := &model.Node{
				Name: "stale-node", Scheme: "http", Address: u.Hostname(), Port: port, BasePath: "/",
				ApiToken: "tok", Enable: true, Status: "online", AllowPrivateAddress: true,
			}
			if err := database.GetDB().Create(node).Error; err != nil {
				t.Fatalf("create node: %v", err)
			}
			remote := runtime.NewRemote(node, nil)
			useTestRuntimeManager(t).SetRuntimeOverride(node.Id, remote)

			ib := seedRenewableNeighbour(t, 23131+i, &node.Id)
			remote.AdoptInboundAlias(ib, runtime.RemoteInboundOption{Id: 7, Tag: ib.Tag})

			commitTickBetweenReadAndWrite(t, renewYTick(ib.Id), func() {
				if err := op.run(t, ib); err != nil {
					t.Errorf("%s: %v", op.name, err)
				}
			})
			requireNeighbourRenewed(t, ib.Id)

			saved, err := (&InboundService{}).GetInbound(ib.Id)
			if err != nil {
				t.Fatalf("GetInbound: %v", err)
			}
			pushed, err := remote.ReconcileInbound(context.Background(), saved, true)
			if err != nil {
				t.Fatalf("ReconcileInbound: %v", err)
			}
			if !pushed {
				t.Fatal("reconcile skipped the inbound: the node never receives y@stale's renewal")
			}
		})
	}
}

func TestRebaseClientSettings(t *testing.T) {
	const base = `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":false,"expiryTime":1}]}`
	cases := []struct {
		name, ours, current, want string
	}{
		{
			name:    "untouched client takes the committed version",
			ours:    `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":2},{"email":"b","enable":false,"expiryTime":1}]}`,
			current: `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":true,"expiryTime":9}]}`,
			want:    `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":2},{"email":"b","enable":true,"expiryTime":9}]}`,
		},
		{
			name:    "edited client keeps committed changes to fields the op left alone",
			ours:    `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":false,"expiryTime":1,"comment":"x"}]}`,
			current: `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":true,"expiryTime":9}]}`,
			want:    `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":true,"expiryTime":9,"comment":"x"}]}`,
		},
		{
			name:    "client the op removed stays removed",
			ours:    `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1}]}`,
			current: `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":true,"expiryTime":9}]}`,
			want:    `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1}]}`,
		},
		{
			name:    "client committed after the read is kept",
			ours:    `{"decryption":"none","clients":[{"email":"a","enable":false,"totalGB":1},{"email":"b","enable":false,"expiryTime":1}]}`,
			current: `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":false,"expiryTime":1},{"email":"c"}]}`,
			want:    `{"decryption":"none","clients":[{"email":"a","enable":false,"totalGB":1},{"email":"b","enable":false,"expiryTime":1},{"email":"c"}]}`,
		},
		{
			name:    "untouched client removed after the read stays removed",
			ours:    `{"decryption":"none","clients":[{"email":"a","enable":false,"totalGB":1},{"email":"b","enable":false,"expiryTime":1}]}`,
			current: `{"decryption":"none","clients":[{"email":"a","enable":true,"totalGB":1}]}`,
			want:    `{"decryption":"none","clients":[{"email":"a","enable":false,"totalGB":1}]}`,
		},
		{
			name:    "top-level key follows whichever side changed it",
			ours:    `{"decryption":"none","testseed":[1],"clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":false,"expiryTime":1}]}`,
			current: `{"decryption":"mlkem","clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":false,"expiryTime":1}]}`,
			want:    `{"decryption":"mlkem","testseed":[1],"clients":[{"email":"a","enable":true,"totalGB":1},{"email":"b","enable":false,"expiryTime":1}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rebaseClientSettings(base, tc.ours, tc.current)
			if err != nil {
				t.Fatalf("rebaseClientSettings: %v", err)
			}
			var gotV, wantV any
			if err := json.Unmarshal([]byte(got), &gotV); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.want), &wantV); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}
			if !reflect.DeepEqual(gotV, wantV) {
				t.Fatalf("rebase = %s\nwant    %s", got, tc.want)
			}
		})
	}
}
