package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoteSnapshotMalformedResponseClearsHistory(t *testing.T) {
	valid := `{"version":1,"revision":"one","structure":{"inbounds":[],"attachments":[]},"clients":[],"inboundTraffic":[]}`
	for _, bad := range []string{
		`{"version":2,"revision":"one","clients":[],"inboundTraffic":[]}`,
		`{"version":1,"revision":"one","clients":null,"inboundTraffic":[]}`,
		`{"version":1,"revision":"one","clients":{},"inboundTraffic":[]}`,
		`{"version":1,"revision":"one","clients":[null],"inboundTraffic":[]}`,
		`{"version":1,"revision":"one","clients":[],"inboundTraffic":[],"onlineTree":[]}`,
		`{"version":1,"revision":"one","structure":{"inbounds":null,"attachments":[]},"clients":[],"inboundTraffic":[]}`,
		`{"version":1,"revision":"one","clients":[}`,
	} {
		t.Run(bad, func(t *testing.T) {
			tick := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-3x-Node-Caps", "traffic-snapshot-v1")
				if req.URL.Path != "/panel/api/inbounds/trafficSnapshot" {
					t.Errorf("malformed response triggered legacy fallback: %s", req.URL.Path)
				}
				tick++
				body := valid
				if tick == 2 {
					body = bad
				}
				if tick == 3 && req.URL.Query().Get("revision") != "" {
					t.Error("malformed response retained revision")
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
			if snap, err := remote.FetchTrafficSnapshot(context.Background()); err == nil || snap != nil {
				t.Fatalf("accepted malformed response: %+v %v", snap, err)
			}
			last, err := remote.FetchTrafficSnapshot(context.Background())
			if err != nil || last == nil || last.StructuralRevision == first.StructuralRevision {
				t.Fatalf("failed recovery: %+v %v", last, err)
			}
		})
	}
}

func TestSnapshotDecodeMapValueBoundary(t *testing.T) {
	values := `[` + strings.Repeat(`"",`, maxNodeMapValues-3) + `""]`
	for _, field := range []string{"onlineTree", "activeInboundTree"} {
		wire, err := decodeNodeTrafficSnapshot([]byte(`{"` + field + `":{"guid":` + values + `}}`))
		if err != nil {
			t.Fatalf("at-limit %s rejected: %v", field, err)
		}
		tree := wire.OnlineTree
		if field == "activeInboundTree" {
			tree = wire.ActiveInboundTree
		}
		if len(tree["guid"]) != maxNodeMapValues-2 {
			t.Fatalf("lost %s values: %d", field, len(tree["guid"]))
		}
	}
}
