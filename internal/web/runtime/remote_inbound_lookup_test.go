package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInboundLookupFailureDoesNotAcknowledgeMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"forbidden", http.StatusForbidden, "", "HTTP 403"},
		{"unavailable", http.StatusServiceUnavailable, "", "HTTP 503"},
		{"application error", http.StatusOK, `{"success":false,"msg":"node database unavailable"}`, "node database unavailable"},
		{"invalid catalog", http.StatusOK, `{"success":true,"obj":{"id":4}}`, "decode inbound list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mutations atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || !strings.HasSuffix(req.URL.Path, "/panel/api/inbounds/list") {
					mutations.Add(1)
					_, _ = w.Write([]byte(`{"success":true}`))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			remote := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
			inbound := &model.Inbound{Tag: "in-lookup", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
			if err := remote.DelInbound(context.Background(), inbound); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("DelInbound error = %v, want %q", err, tc.want)
			}
			if err := remote.UpdateInbound(context.Background(), inbound, inbound); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("UpdateInbound error = %v, want %q", err, tc.want)
			}
			if got := mutations.Load(); got != 0 {
				t.Errorf("mutation requests after failed lookup = %d, want 0", got)
			}
		})
	}
}

func TestInboundLookupPreservesCanceledContext(t *testing.T) {
	srv, _ := newCountingNodeServer(t, `{"success":true}`)
	remote := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inbound := &model.Inbound{Tag: "in-canceled", Protocol: model.VLESS}
	if err := remote.DelInbound(ctx, inbound); !errors.Is(err, context.Canceled) {
		t.Errorf("DelInbound error = %v, want canceled context", err)
	}
	if err := remote.UpdateInbound(ctx, inbound, inbound); !errors.Is(err, context.Canceled) {
		t.Errorf("UpdateInbound error = %v, want canceled context", err)
	}
}
