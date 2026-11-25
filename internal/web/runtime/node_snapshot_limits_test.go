package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRemoteSnapshotCapsAndRecovers(t *testing.T) {
	list := func(item string, count int) string { return "[" + strings.Repeat(item+",", count-1) + item + "]" }
	for _, tc := range []struct{ name, field, value string }{
		{"catalog", "structure", `{"inbounds":` + list("{}", 65537) + `,"attachments":[]}`},
		{"embedded clients", "structure", `{"inbounds":[{"id":1,"CLIENTSTATS":` + list("{}", (1<<20)+1) + `}],"attachments":[]}`},
		{"attachments", "structure", `{"inbounds":[],"attachments":` + list("{}", (1<<20)+1) + `}`},
		{"clients", "clients", list("{}", (1<<20)+1)},
		{"inbound counters", "inboundTraffic", list("{}", 65537)},
		{"online emails", "onlineEmails", list(`""`, (1<<20)+1)},
		{"online tree", "onlineTree", `{"guid":` + list(`""`, 1<<20) + `}`},
		{"active tree", "activeInboundTree", `{"guid":` + list(`""`, 1<<20) + `}`},
		{"last online", "lastOnlineMap", `{"guid":` + list("0", 1<<20) + `}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			valid := `{"version":1,"revision":"one","structure":{"inbounds":[],"attachments":[]},"clients":[],"inboundTraffic":[]}`
			tick, legacy := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
				if req.URL.Path != "/panel/api/inbounds/trafficSnapshot" {
					legacy++
					t.Errorf("unexpected fallback: %s", req.URL.Path)
				}
				tick++
				body := valid
				if tick == 2 {
					body = `{"version":1,"revision":"one","structure":{"inbounds":[],"attachments":[]},"clients":[],"inboundTraffic":[],"` + tc.field + `":` + tc.value + `}`
				}
				if tick == 3 && req.URL.Query().Get("revision") != "" {
					t.Error("invalid snapshot retained history")
				}
				fmt.Fprint(w, `{"success":true,"obj":`+body+`}`)
			}))
			defer server.Close()
			remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
			remote.recordCaps(http.Header{"X-3x-Node-Caps": []string{"traffic-snapshot-v1"}})
			first, err := remote.FetchTrafficSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			bad, err := remote.FetchTrafficSnapshot(context.Background())
			if bad != nil || !errors.Is(err, errRemoteListTooLong) {
				t.Fatalf("oversize %s: snapshot=%v error=%v, want list limit", tc.name, bad, err)
			}
			last, err := remote.FetchTrafficSnapshot(context.Background())
			if err != nil || last == nil || last.StructuralRevision == first.StructuralRevision || legacy != 0 {
				t.Fatalf("failed closed recovery: %+v %v legacy=%d", last, err, legacy)
			}
		})
	}
}

func TestSnapshotDecodeCollectionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		field string
	}{
		{"clients", maxNodeSnapshotClients, "clients"},
		{"attachments", maxNodeSnapshotClients, "attachments"},
		{"inbound counters", maxNodeInbounds, "inboundTraffic"},
		{"catalog", maxNodeInbounds, "inbounds"},
		{"online emails", maxNodeMapValues, "onlineEmails"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := "{}"
			if tc.field == "onlineEmails" {
				item = `""`
			}
			list := "[" + strings.Repeat(item+",", tc.count-1) + item + "]"
			raw := `{"` + tc.field + `":` + list + `}`
			if tc.field == "inbounds" || tc.field == "attachments" {
				raw = `{"structure":` + raw + `}`
			}
			wire, err := decodeNodeTrafficSnapshot(json.RawMessage(raw))
			if err != nil {
				t.Fatalf("at-limit payload rejected: %v", err)
			}
			got := 0
			switch tc.field {
			case "clients":
				got = len(wire.Clients)
			case "attachments":
				got = len(wire.Structure.Attachments)
			case "inboundTraffic":
				got = len(wire.InboundTraffic)
			case "inbounds":
				got = len(wire.Structure.Inbounds)
			case "onlineEmails":
				got = len(wire.OnlineEmails)
			}
			if got != tc.count {
				t.Fatalf("decoded %d entries, want %d", got, tc.count)
			}
		})
	}
	for _, raw := range []string{`{"clients":[}`, `{"clients":{}}`, `{"structure":{"inbounds":[{"clientStats":{}}]}}`, `{"onlineTree":[]}`, `{"lastOnlineMap":{"guid":"bad"}}`, `{"clients":[]} trailing`} {
		if _, err := decodeNodeTrafficSnapshot(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted malformed compact payload: %s", raw)
		}
	}
}

func TestRemoteSnapshotLargeValidDeployment(t *testing.T) {
	wire := NodeTrafficSnapshotV1{Version: 1, Revision: "large", Structure: &NodeSnapshotStructure{Inbounds: []*model.Inbound{{Id: 1, Tag: "one"}, {Id: 2, Tag: "two"}}, Attachments: []NodeSnapshotAttachment{}}, InboundTraffic: []NodeInboundTraffic{{ID: 1}, {ID: 2}}}
	for id := 1; id <= 20000; id++ {
		email := fmt.Sprintf("client-%d", id)
		wire.Clients = append(wire.Clients, NodeClientTraffic{ID: id, InboundID: 1, Email: email, Enable: true, Total: 1000})
		wire.Structure.Attachments = append(wire.Structure.Attachments, NodeSnapshotAttachment{InboundID: 1, Email: email}, NodeSnapshotAttachment{InboundID: 2, Email: email})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
		if req.URL.Path != "/panel/api/inbounds/trafficSnapshot" {
			t.Errorf("legacy fallback: %s", req.URL.Path)
		}
		response := wire
		if req.URL.Query().Get("revision") == wire.Revision {
			response.Structure = nil
		}
		response.Clients[0].Up++
		if err := json.NewEncoder(w).Encode(map[string]any{"success": true, "obj": response}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	remote := NewRemote(nodeForPlainServer(t, server, "verify", "token"), nil)
	remote.recordCaps(http.Header{"X-3x-Node-Caps": []string{"traffic-snapshot-v1"}})
	for tick := 1; tick <= 3; tick++ {
		snap, err := remote.FetchTrafficSnapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Inbounds) != 2 || len(snap.Inbounds[0].ClientStats) != 20000 || len(snap.Inbounds[1].ClientStats) != 20000 || snap.Inbounds[1].ClientStats[0].Up != int64(tick) {
			t.Fatalf("large deployment lost counters or attachments on tick %d", tick)
		}
	}
}

func BenchmarkCompactSnapshotDecode(b *testing.B) {
	wire := NodeTrafficSnapshotV1{Version: 1, Revision: "benchmark", InboundTraffic: []NodeInboundTraffic{}}
	for id := 1; id <= 20000; id++ {
		wire.Clients = append(wire.Clients, NodeClientTraffic{ID: id, InboundID: 1, Email: fmt.Sprintf("client-%d", id), Up: 100, Total: 1000, Enable: true})
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		b.Fatal(err)
	}
	for _, mode := range []string{"unbounded-baseline", "bounded"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(raw)))
			for range b.N {
				var decoded NodeTrafficSnapshotV1
				var err error
				if mode == "bounded" {
					decoded, err = decodeNodeTrafficSnapshot(raw)
				} else {
					err = json.Unmarshal(raw, &decoded)
				}
				if err != nil || len(decoded.Clients) != 20000 {
					b.Fatalf("decode: %v", err)
				}
			}
		})
	}
}
