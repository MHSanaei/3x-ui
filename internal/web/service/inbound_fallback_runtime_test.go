package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestBuildRuntimeInboundForAPI_InjectsFallbacks is the #5963 regression: the
// runtime inbound sent to nodes never carried the inbound_fallbacks rows, so
// fallbacks configured on the master silently vanished from every node.
func TestBuildRuntimeInboundForAPI_InjectsFallbacks(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "fb@x", ID: "11111111-1111-1111-1111-111111111111", Enable: true},
	}
	master := mkInbound(t, 30201, model.VLESS, clientsSettings(t, clients))
	master.StreamSettings = `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"s"}}`
	if err := db.Save(master).Error; err != nil {
		t.Fatalf("save master stream: %v", err)
	}

	fb := model.InboundFallback{MasterId: master.Id, ChildId: 0, Dest: "8081", Alpn: "h2", Path: "/fb", Xver: 1}
	if err := db.Create(&fb).Error; err != nil {
		t.Fatalf("seed fallback: %v", err)
	}

	runtimeIb, err := svc.buildInboundForLocalRuntime(db, master)
	if err != nil {
		t.Fatalf("buildInboundForLocalRuntime: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(runtimeIb.Settings), &settings); err != nil {
		t.Fatalf("runtime settings not valid json: %v", err)
	}
	fallbacks, ok := settings["fallbacks"].([]any)
	if !ok || len(fallbacks) != 1 {
		t.Fatalf("runtime settings must carry the fallback, got: %s", runtimeIb.Settings)
	}
	if !strings.Contains(runtimeIb.Settings, `"dest"`) || !strings.Contains(runtimeIb.Settings, "8081") {
		t.Fatalf("fallback dest missing from runtime settings: %s", runtimeIb.Settings)
	}
}

// A non-fallback-capable inbound (ws transport) must stay untouched.
func TestBuildRuntimeInboundForAPI_NoFallbacksOnWsInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "ws@x", ID: "22222222-2222-2222-2222-222222222222", Enable: true},
	}
	ib := mkInbound(t, 30202, model.VLESS, clientsSettings(t, clients))
	ib.StreamSettings = `{"network":"ws","security":"none"}`
	if err := db.Save(ib).Error; err != nil {
		t.Fatalf("save stream: %v", err)
	}
	fb := model.InboundFallback{MasterId: ib.Id, Dest: "8082"}
	if err := db.Create(&fb).Error; err != nil {
		t.Fatalf("seed fallback: %v", err)
	}

	runtimeIb, err := svc.buildInboundForLocalRuntime(db, ib)
	if err != nil {
		t.Fatalf("buildInboundForLocalRuntime: %v", err)
	}
	if strings.Contains(runtimeIb.Settings, "fallbacks") {
		t.Fatalf("ws inbound must not receive fallbacks: %s", runtimeIb.Settings)
	}
}

// #6835: a child that accepts PROXY protocol must get a header from its fallback, whichever
// was saved first; an explicit dest or a chosen xver stays the operator's.
func TestBuildFallbacksJSON_DerivesXverFromTheChild(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	master := mkInbound(t, 30301, model.VLESS, `{"clients":[]}`)

	cases := []struct {
		name   string
		stream string
		dest   string
		xver   int
		want   any
	}{
		{"raw transport flag", `{"network":"tcp","tcpSettings":{"acceptProxyProtocol":true}}`, "", 0, float64(1)},
		{"sockopt flag", `{"network":"grpc","sockopt":{"acceptProxyProtocol":true}}`, "", 0, float64(1)},
		{"websocket flag", `{"network":"ws","wsSettings":{"acceptProxyProtocol":true}}`, "", 0, float64(1)},
		{"stale flag of another transport", `{"network":"grpc","tcpSettings":{"acceptProxyProtocol":true}}`, "", 0, nil},
		{"child without PROXY protocol", `{"network":"tcp"}`, "", 0, nil},
		{"explicit dest keeps the stored xver", `{"network":"tcp","tcpSettings":{"acceptProxyProtocol":true}}`, "127.0.0.1:9000", 0, nil},
		{"chosen xver 2 stays", `{"network":"tcp","tcpSettings":{"acceptProxyProtocol":true}}`, "", 2, float64(2)},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Where("master_id = ?", master.Id).Delete(&model.InboundFallback{}).Error; err != nil {
				t.Fatal(err)
			}
			child := mkInboundStream(t, 30310+i, model.VLESS, `{"clients":[]}`, tc.stream)
			fb := model.InboundFallback{MasterId: master.Id, ChildId: child.Id, Dest: tc.dest, Xver: tc.xver}
			if err := db.Create(&fb).Error; err != nil {
				t.Fatal(err)
			}
			got, err := (&FallbackService{}).BuildFallbacksJSON(db, master.Id)
			if err != nil || len(got) != 1 {
				t.Fatalf("BuildFallbacksJSON = %v, %v", got, err)
			}
			raw, _ := json.Marshal(got[0])
			var entry map[string]any
			_ = json.Unmarshal(raw, &entry)
			if entry["xver"] != tc.want {
				t.Fatalf("xver = %v, want %v (entry %s)", entry["xver"], tc.want, raw)
			}
		})
	}
}

// The running master keeps the fallback it was built with, so turning PROXY protocol on in a
// linked child must rebuild the master too; an inbound no fallback derives from needs nothing.
func TestUpdateInbound_FallbackChildChangeRebuildsItsMaster(t *testing.T) {
	setupBulkDB(t)
	mgr := useTestRuntimeManager(t)
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	db := database.GetDB()
	master := mkInboundStream(t, 30401, model.VLESS, `{"clients":[]}`, `{"network":"tcp","security":"tls"}`)
	linked := mkInboundStream(t, 30402, model.VLESS, `{"clients":[]}`, `{"network":"tcp"}`)
	unlinked := mkInboundStream(t, 30403, model.VLESS, `{"clients":[]}`, `{"network":"tcp"}`)
	if err := db.Create(&model.InboundFallback{MasterId: master.Id, ChildId: linked.Id}).Error; err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		ib   *model.Inbound
		want bool
	}{{"linked child", linked, true}, {"unlinked inbound", unlinked, false}} {
		t.Run(tc.name, func(t *testing.T) {
			update := *tc.ib
			update.StreamSettings = `{"network":"tcp","tcpSettings":{"acceptProxyProtocol":true}}`
			_, needRestart, err := (&InboundService{}).UpdateInbound(&update)
			if err != nil {
				t.Fatalf("UpdateInbound: %v", err)
			}
			if needRestart != tc.want {
				t.Fatalf("needRestart = %v, want %v", needRestart, tc.want)
			}
		})
	}
}
