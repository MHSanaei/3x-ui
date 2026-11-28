package service

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

type addOrphanReads struct {
	selects, probes, maxEmails int
}

func trackAddOrphanReads(t testing.TB) *addOrphanReads {
	t.Helper()
	reads := &addOrphanReads{}
	callbacks := database.GetDB().Callback()
	const name = "test:add_orphan_reads"
	unquote := strings.NewReplacer("\"", "", "`", "")
	count := func(tx *gorm.DB) {
		sql := tx.Statement.SQL.String()
		if !strings.HasPrefix(sql, "SELECT ") {
			return
		}
		reads.selects++
		if strings.HasPrefix(unquote.Replace(sql), "SELECT email FROM clients WHERE email IN (") {
			reads.probes++
			reads.maxEmails = max(reads.maxEmails, len(tx.Statement.Vars))
		}
	}
	if err := callbacks.Query().After("gorm:query").Register(name, count); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := callbacks.Query().Remove(name); err != nil {
			t.Error(err)
		}
	})
	if err := callbacks.Row().After("gorm:row").Register(name, count); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := callbacks.Row().Remove(name); err != nil {
			t.Error(err)
		}
	})
	return reads
}

func orphanTestClient(email string) model.Client {
	return model.Client{ID: uuid.NewString(), Email: email, SubID: uuid.NewString()}
}

func addOrphanTestPayload(t testing.TB, id int, clients ...model.Client) *model.Inbound {
	t.Helper()
	return &model.Inbound{Id: id, Settings: clientsSettings(t, clients)}
}

func TestAddInboundClientBoundedOrphanCleanup(t *testing.T) {
	setupBulkDB(t)
	testAddInboundClientBoundedOrphanCleanup(t)
}

func TestAddInboundClientBoundedOrphanCleanupPostgres(t *testing.T) {
	setupScaleDB(t)
	testAddInboundClientBoundedOrphanCleanup(t)
}

func testAddInboundClientBoundedOrphanCleanup(t *testing.T) {
	t.Helper()
	ds := seedScaleDataset(t, 1000, 1)
	id := ds.inboundIds[0]
	live := ds.perInbound[0][0]
	stale := orphanTestClient("recent@orphan-add")
	old := orphanTestClient("old@orphan-add")
	entries := append([]model.Client{stale}, ds.perInbound[0]...)
	entries = append(entries, old)
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", id).
		Update("settings", clientsSettings(t, entries)).Error; err != nil {
		t.Fatal(err)
	}
	tombstoneClientEmails([]string{live.Email, stale.Email})
	t.Cleanup(func() { withdrawClientTombstones(live.Email, stale.Email) })

	reads := trackAddOrphanReads(t)
	cs, is := &ClientService{}, &InboundService{}
	want := emailsOf(ds.perInbound[0])
	for i := range 4 {
		newClient := orphanTestClient(fmt.Sprintf("new-%d@orphan-add", i))
		want = append(want, newClient.Email)
		*reads = addOrphanReads{}
		if _, err := cs.AddInboundClient(is, addOrphanTestPayload(t, id, live, newClient)); err != nil {
			t.Fatal(err)
		}
		if reads.probes > 1 || reads.maxEmails > 400 {
			t.Errorf("add %d issued %d orphan SELECTs with up to %d emails, want at most one query of at most 400 emails", i, reads.probes, reads.maxEmails)
		}
		if i == 0 {
			emails := settingsClientEmails(t, id)
			if !slices.Contains(emails, old.Email) && !slices.Contains(emails, stale.Email) {
				t.Error("a single add swept orphans at both ends of the inbound")
			}
		}
	}
	emails := settingsClientEmails(t, id)
	if !slices.Equal(emails, want) {
		t.Fatalf("after rotating through the inbound, got %d entries, want %d with both orphans removed and every live email kept once in order", len(emails), len(want))
	}
}

func TestAddInboundClientOrphanProbePreservesConcurrentRecreation(t *testing.T) {
	setupBulkDB(t)
	testAddInboundClientOrphanProbePreservesConcurrentRecreation(t)
}

func TestAddInboundClientOrphanProbePreservesConcurrentRecreationPostgres(t *testing.T) {
	setupScaleDB(t)
	testAddInboundClientOrphanProbePreservesConcurrentRecreation(t)
}

func testAddInboundClientOrphanProbePreservesConcurrentRecreation(t *testing.T) {
	t.Helper()
	db := database.GetDB()
	resetScaleTables(t, db, "inbounds", "clients", "client_inbounds", "client_traffics")
	recreated := orphanTestClient("recreated@orphan-add")
	target := model.Inbound{Tag: "target-orphan", Protocol: model.VLESS, Port: 44001, Settings: clientsSettings(t, []model.Client{recreated})}
	other := model.Inbound{Tag: "other-orphan", Protocol: model.VLESS, Port: 44002, Settings: clientsSettings(t, nil)}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	newClient := orphanTestClient("concurrent@orphan-add")
	data := addOrphanTestPayload(t, target.Id, newClient)
	cs := &ClientService{}
	commitTickBetweenReadAndWrite(t, func(tx *gorm.DB) error {
		return cs.ApplyInboundClientDelta(tx, other.Id, []model.Client{recreated}, nil)
	}, func() {
		if _, err := cs.AddInboundClient(&InboundService{}, data); err != nil {
			t.Error(err)
		}
	})
	if got, want := settingsClientEmails(t, target.Id), []string{recreated.Email, newClient.Email}; !slices.Equal(got, want) {
		t.Fatalf("settings after recreation committed first = %v, want %v", got, want)
	}
}

func TestNodeAddInboundClientReconcilesOrphanRemoval(t *testing.T) {
	setupBulkDB(t)
	stale := orphanTestClient("stale@node-orphan")
	added := orphanTestClient("added@node-orphan")
	added.Enable = true
	var adds, updates atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/api/clients/add":
			adds.Add(1)
		case "/panel/api/inbounds/update/7":
			settings := r.FormValue("settings")
			if !strings.Contains(settings, added.Email) || strings.Contains(settings, stale.Email) {
				t.Errorf("node update settings = %s, want only the added client", settings)
			}
			updates.Add(1)
		default:
			t.Errorf("unexpected node call: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	host, portString, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	db := database.GetDB()
	node := &model.Node{
		Name: "orphan-node", Scheme: "http", Address: host, Port: port, BasePath: "/",
		ApiToken: "test", Enable: true, Status: "online", AllowPrivateAddress: true,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{
		Tag: "orphan-inbound", NodeID: &node.Id, Port: 44003, Protocol: model.VLESS,
		Settings: clientsSettings(t, []model.Client{stale}),
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	remote := runtime.NewRemote(node, nil)
	useTestRuntimeManager(t).SetRuntimeOverride(node.Id, remote)
	remote.AdoptInboundAlias(ib, runtime.RemoteInboundOption{Id: 7, Tag: ib.Tag})
	svc := &InboundService{}
	if _, err := (&ClientService{}).AddInboundClient(svc, addOrphanTestPayload(t, ib.Id, added)); err != nil {
		t.Fatal(err)
	}
	saved, err := svc.GetInbound(ib.Id)
	if err != nil {
		t.Fatal(err)
	}
	pushed, err := remote.ReconcileInbound(t.Context(), saved, true)
	if err != nil {
		t.Fatal(err)
	}
	if !pushed || adds.Load() != 1 || updates.Load() != 1 {
		t.Fatalf("got %d client adds and %d inbound updates (pushed=%t), want one of each", adds.Load(), updates.Load(), pushed)
	}
}

func BenchmarkAddInboundClientOrphanCleanup(b *testing.B) {
	setupScaleDB(b)
	for _, n := range []int{1000, 10000, 50000, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			ds := seedScaleDataset(b, n, 1)
			id := ds.inboundIds[0]
			reads := trackAddOrphanReads(b)
			cs, is := &ClientService{}, &InboundService{}
			warmup := addOrphanTestPayload(b, id, orphanTestClient("warm@orphan-bench"))
			if _, err := cs.AddInboundClient(is, warmup); err != nil {
				b.Fatal(err)
			}
			*reads = addOrphanReads{}
			var samples []time.Duration
			for b.Loop() {
				b.StopTimer()
				client := orphanTestClient(fmt.Sprintf("add-%d@orphan-bench", len(samples)))
				data := addOrphanTestPayload(b, id, client)
				b.StartTimer()
				start := time.Now()
				if _, err := cs.AddInboundClient(is, data); err != nil {
					b.Fatal(err)
				}
				samples = append(samples, time.Since(start))
			}
			slices.Sort(samples)
			b.ReportMetric(float64(samples[(len(samples)-1)/2].Microseconds())/1000, "p50-ms")
			b.ReportMetric(float64(samples[(95*len(samples)+99)/100-1].Microseconds())/1000, "p95-ms")
			b.ReportMetric(float64(reads.selects)/float64(b.N), "selects/op")
			b.ReportMetric(float64(reads.probes)/float64(b.N), "orphan-selects/op")
		})
	}
}
