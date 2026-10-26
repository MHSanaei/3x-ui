package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wirecodec"
)

func TestAWGPingRemoteCapabilityNeverSilentlyDropsRights(t *testing.T) {
	for _, caps := range []string{"", wirecodec.CapZstd, wirecodec.NodeCaps} {
		t.Run(caps, func(t *testing.T) {
			pushes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(wirecodec.CapsHeader, caps)
				if r.Method == http.MethodPost {
					pushes++
				}
				_, _ = w.Write([]byte(`{"success":true,"obj":[]}`))
			}))
			defer server.Close()
			remote := NewRemote(nodeForPlainServer(t, server, "verify", "synthetic"), nil)
			ib := &model.Inbound{Protocol: model.AmneziaWG, Tag: "synthetic-awg", Settings: `{"server":{},"clients":[{"email":"synthetic","awgPing":{"enabled":true,"allow":["192.0.2.0/24"],"deny":[]}}]}`}
			err := remote.AddInbound(context.Background(), ib)
			if caps != wirecodec.NodeCaps {
				if err == nil || !strings.Contains(err.Error(), "does not support AWG IPv4 ping") || pushes != 0 {
					t.Fatalf("old node received enabled policy: pushes %d err %v", pushes, err)
				}
			} else if err != nil || pushes != 1 {
				t.Fatalf("capable node rejected: pushes %d err %v", pushes, err)
			}
		})
	}
}
