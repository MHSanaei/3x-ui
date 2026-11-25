package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRemoteSnapshotNegotiatesUnchangedStructure(t *testing.T) {
	full, compact := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
		switch r.URL.Path {
		case "/panel/api/inbounds/list":
			full++
			fmt.Fprint(w, `{"success":true,"obj":[{"id":7,"tag":"edge","settings":{"clients":[]},"clientStats":[]}]}`)
		case "/panel/api/inbounds/trafficSnapshot":
			compact++
			structure := `{"inbounds":[{"id":7,"tag":"edge","settings":{"clients":[]}}],"attachments":[]}`
			if r.URL.Query().Get("revision") == "revision1" {
				structure = "null"
			}
			fmt.Fprintf(w, `{"success":true,"obj":{"version":1,"revision":"revision1","structure":%s,"inboundTraffic":[{"id":7,"up":%d,"down":0}],"clients":[],"lastOnlineMap":{},"onlineTree":{},"activeInboundTree":{}}}`, structure, compact)
		default:
			fmt.Fprint(w, `{"success":true,"obj":{}}`)
		}
	}))
	defer server.Close()
	remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
	for i := range 3 {
		snapshot, err := remote.FetchTrafficSnapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Inbounds) != 1 || snapshot.Inbounds[0].Tag != "edge" {
			t.Fatalf("lost structure: %+v", snapshot)
		}
		if i > 0 && snapshot.Inbounds[0].Up != int64(i) {
			t.Fatalf("stale counters: %+v", snapshot.Inbounds[0])
		}
		snapshot.Inbounds[0].Tag = "caller-modified"
	}
	if full != 1 || compact != 2 {
		t.Fatalf("full=%d compact=%d, want 1 full then 2 compact snapshots", full, compact)
	}
}

func TestRemoteSnapshotOnlyUnsupportedFallsBack(t *testing.T) {
	for _, code := range []int{404, 405, 401, 403, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			full, compact := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
				switch req.URL.Path {
				case "/panel/api/inbounds/trafficSnapshot":
					compact++
					http.Error(w, "blocked", code)
				case "/panel/api/inbounds/list":
					full++
					fmt.Fprint(w, `{"success":true,"obj":[]}`)
				default:
					fmt.Fprint(w, `{"success":true,"obj":{}}`)
				}
			}))
			defer server.Close()
			remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
			remote.recordCaps(http.Header{"X-3x-Node-Caps": []string{"traffic-snapshot-v1"}})
			for range 3 {
				snap, err := remote.FetchTrafficSnapshot(context.Background())
				if code == 404 || code == 405 {
					if err != nil || len(snap.Inbounds) != 0 {
						t.Fatalf("fallback: %v %+v", err, snap)
					}
				} else {
					var status *remoteHTTPError
					if snap != nil || !errors.As(err, &status) || status.code != code {
						t.Fatalf("masked error: %+v %v", snap, err)
					}
				}
			}
			if code == 404 || code == 405 {
				if compact != 1 || full != 3 {
					t.Fatalf("404 storm or no fallback: compact=%d full=%d", compact, full)
				}
			} else if full != 0 || compact != 3 {
				t.Fatalf("auth/server error downgraded: compact=%d full=%d", compact, full)
			}
		})
	}
}

func TestRemoteSnapshotRejectsUnknownPartialAndRecovers(t *testing.T) {
	var tick atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
		switch tick.Add(1) {
		case 1:
			fmt.Fprint(w, `{"success":true,"obj":{"version":1,"revision":"one","structure":{"inbounds":[],"attachments":[]},"clients":[],"inboundTraffic":[]}}`)
		case 2:
			fmt.Fprint(w, `{"success":true,"obj":{"version":1,"revision":"unknown","clients":[],"inboundTraffic":[]}}`)
		default:
			if req.URL.Query().Get("revision") != "" {
				t.Error("failed snapshot retained revision")
			}
			fmt.Fprint(w, `{"success":true,"obj":{"version":1,"revision":"two","structure":{"inbounds":[],"attachments":[]},"clients":[],"inboundTraffic":[]}}`)
		}
	}))
	defer server.Close()
	remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
	remote.recordCaps(http.Header{"X-3x-Node-Caps": []string{"traffic-snapshot-v1"}})
	first, err := remote.FetchTrafficSnapshot(context.Background())
	if err != nil || first.Inbounds == nil || len(first.Inbounds) != 0 {
		t.Fatalf("valid empty rejected: %+v %v", first, err)
	}
	bad, err := remote.FetchTrafficSnapshot(context.Background())
	if bad != nil || err == nil || !strings.Contains(err.Error(), "no matching structure") {
		t.Fatalf("unknown partial trusted: %+v %v", bad, err)
	}
	last, err := remote.FetchTrafficSnapshot(context.Background())
	if err != nil || last.Inbounds == nil || last.StructuralRevision == first.StructuralRevision {
		t.Fatalf("recovery failed: %+v %v", last, err)
	}
}

func TestRemoteSnapshotConcurrentWaitHonorsDeadline(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)
		<-release
		fmt.Fprint(w, `{"success":true,"obj":{"version":1,"revision":"one","structure":{"inbounds":[],"attachments":[]},"clients":[],"inboundTraffic":[]}}`)
	}))
	defer server.Close()
	remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
	remote.recordCaps(http.Header{"X-3x-Node-Caps": []string{"traffic-snapshot-v1"}})
	done := make(chan error, 1)
	go func() { _, err := remote.FetchTrafficSnapshot(context.Background()); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	snap, err := remote.FetchTrafficSnapshot(ctx)
	close(release)
	if snap != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait ignored cancellation: %+v %v", snap, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotExpansionRequiresCompleteCounters(t *testing.T) {
	structure := &NodeSnapshotStructure{Inbounds: []*model.Inbound{{Id: 1}, {Id: 2}}, Attachments: []NodeSnapshotAttachment{{InboundID: 2, Email: "shared"}}}
	wire := NodeTrafficSnapshotV1{InboundTraffic: []NodeInboundTraffic{{ID: 1}}, Clients: []NodeClientTraffic{}}
	if snap, err := wire.expand(structure); snap != nil || err == nil || err.Error() != "missing snapshot inbound counters" {
		t.Fatalf("incomplete accepted: %+v %v", snap, err)
	}
	wire.InboundTraffic = append(wire.InboundTraffic, NodeInboundTraffic{ID: 2})
	wire.Clients = []NodeClientTraffic{{InboundID: 1, Email: "shared", Up: 50}, {InboundID: 1, Email: ""}, {InboundID: 2, Email: ""}}
	snap, err := wire.expand(structure)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Inbounds[0].ClientStats) != 2 || len(snap.Inbounds[1].ClientStats) != 2 || snap.Inbounds[1].ClientStats[1].Up != 50 {
		t.Fatalf("lost shared/orphan stats: %+v", snap.Inbounds)
	}
}

func TestRemoteSnapshotRejectsTruncatedUnchangedCounters(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
		if calls.Add(1) == 1 {
			fmt.Fprint(w, `{"success":true,"obj":{"version":1,"revision":"one","structure":{"inbounds":[{"id":1,"tag":"edge","settings":{"clients":[]}}],"attachments":[]},"inboundTraffic":[{"id":1}],"clients":[{"id":1,"inboundId":1,"email":"one","up":100}]}}`)
		} else {
			fmt.Fprint(w, `{"success":true,"obj":{"version":1,"revision":"one","inboundTraffic":[{"id":1}],"clients":[]}}`)
		}
	}))
	defer server.Close()
	remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
	remote.recordCaps(http.Header{"X-3x-Node-Caps": []string{"traffic-snapshot-v1"}})
	first, err := remote.FetchTrafficSnapshot(context.Background())
	if err != nil || len(first.Inbounds[0].ClientStats) != 1 {
		t.Fatalf("initial counters: %+v %v", first, err)
	}
	partial, err := remote.FetchTrafficSnapshot(context.Background())
	if partial != nil || err == nil || err.Error() != "traffic snapshot omitted client counters" {
		t.Fatalf("partial counters could reach full merge: %+v %v", partial, err)
	}
}
