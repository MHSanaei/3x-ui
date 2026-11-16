package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestLocalInboundMutationsPassThroughTCPProtection(t *testing.T) {
	refused := errors.New("firewall refused")
	called := 0
	local := NewLocal(LocalDeps{
		APIPort: func() int {
			t.Fatal("the listener must not be opened before protection succeeds")
			return 0
		},
		WithTCPConnectionLimit: func(func() error) error {
			called++
			return refused
		},
	})
	inbound := &model.Inbound{Tag: "test", Protocol: model.VLESS, Port: 443, Settings: `{"clients":[]}`}
	if err := local.AddInbound(context.Background(), inbound); !errors.Is(err, refused) {
		t.Fatalf("AddInbound error = %v", err)
	}
	if err := local.DelInbound(context.Background(), inbound); !errors.Is(err, refused) {
		t.Fatalf("DelInbound error = %v", err)
	}
	if err := local.UpdateInbound(context.Background(), inbound, &model.Inbound{Enable: false}); !errors.Is(err, refused) {
		t.Fatalf("UpdateInbound disable error = %v", err)
	}
	if called != 3 {
		t.Fatalf("protection hook calls = %d", called)
	}
}
