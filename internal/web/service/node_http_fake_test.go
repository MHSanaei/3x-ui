package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// fakeNodeHTTP emulates a node panel over real HTTP, so the master's Remote
// (tag cache, list fetch, per-op RPC, timeouts) runs for real, not a stub.
type fakeNodeHTTP struct {
	srv  *httptest.Server
	mu   sync.Mutex
	tags map[string]int
	hits map[string]int
	// hold makes every non-list request block until the master gives up.
	hold    bool
	release chan struct{}
}

func newFakeNodeHTTP(t *testing.T) *fakeNodeHTTP {
	t.Helper()
	f := &fakeNodeHTTP{tags: map[string]int{}, hits: map[string]int{}, release: make(chan struct{})}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/inbounds/list") {
			f.mu.Lock()
			f.hits["list"]++
			type ent struct {
				Id  int    `json:"id"`
				Tag string `json:"tag"`
			}
			list := make([]ent, 0, len(f.tags))
			for tag, id := range f.tags {
				list = append(list, ent{Id: id, Tag: tag})
			}
			f.mu.Unlock()
			b, _ := json.Marshal(list)
			_, _ = w.Write([]byte(`{"success":true,"msg":"","obj":` + string(b) + `}`))
			return
		}
		f.mu.Lock()
		f.hits[r.URL.Path]++
		hold := f.hold
		f.mu.Unlock()
		if hold {
			// Drain first: the server only notices a client disconnect once the
			// body is consumed, and Close would otherwise wait on this forever.
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-f.release:
			}
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"msg":""}`))
	}))
	t.Cleanup(f.srv.Close)
	// Registered after Close, so it runs first and frees any held handler.
	t.Cleanup(func() { close(f.release) })
	return f
}

func (f *fakeNodeHTTP) setHold(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hold = v
}

// hitCount is how many requests whose path contains pathPart reached the node.
func (f *fakeNodeHTTP) hitCount(pathPart string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for path, c := range f.hits {
		if strings.Contains(path, pathPart) {
			n += c
		}
	}
	return n
}

// realNodeInbound creates a Node row pointing at the fake server plus one
// inbound on it, with NO runtime override so RuntimeFor builds a real Remote.
func realNodeInbound(t *testing.T, f *fakeNodeHTTP, port int, clients []model.Client) *model.Inbound {
	t.Helper()
	hostPart, portStr, _ := strings.Cut(strings.TrimPrefix(f.srv.URL, "http://"), ":")
	srvPort, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse fake node port: %v", err)
	}
	node := &model.Node{
		Name: fmt.Sprintf("%s-%d", t.Name(), port), Scheme: "http", Address: hostPart, Port: srvPort,
		BasePath: "/", ApiToken: "tok", Enable: true, Status: "online",
		AllowPrivateAddress: true, TlsVerifyMode: "verify",
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	ib := nodeInbound(t, node.Id, port, clients)
	f.mu.Lock()
	f.tags[ib.Tag] = 100 + port%100
	f.mu.Unlock()
	return ib
}
