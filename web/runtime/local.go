package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// LocalDeps wires the runtime to the panel's xray process and the
// service.XrayService restart trigger via callbacks. We use callbacks
// (not an interface to *service.XrayService) because the runtime
// package would otherwise cycle-import service.
type LocalDeps struct {
	// APIPort returns the xray gRPC API port the local engine is
	// currently listening on. Returns 0 when xray isn't running yet —
	// callers should treat that as a transient error.
	APIPort func() int
	// SetNeedRestart trips the panel's "restart xray on next cron tick"
	// flag. Mirrors how InboundController.addInbound calls
	// xrayService.SetToNeedRestart() today.
	SetNeedRestart func()
}

// Local implements Runtime against the panel's own xray process. Each
// call follows the existing inbound.go pattern: open a gRPC client,
// run one operation, close. Per-call init keeps the connection state
// scoped so a stuck call can't leak across operations.
type Local struct {
	deps LocalDeps

	// Serialise gRPC operations — xray's HandlerService isn't documented
	// as concurrent-safe and the existing InboundService implicitly
	// runs one op at a time per request. This matches that.
	mu sync.Mutex
}

// NewLocal builds a Local runtime. deps.APIPort and deps.SetNeedRestart
// are required; callers that want a no-op restart can pass `func(){}`.
func NewLocal(deps LocalDeps) *Local {
	return &Local{deps: deps}
}

func (l *Local) Name() string { return "local" }

// withAPI runs fn against a freshly-initialised XrayAPI client and
// guarantees Close() afterwards. Returns an error if the gRPC port
// isn't available yet (xray still starting / stopped).
func (l *Local) withAPI(fn func(api *xray.XrayAPI) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	port := l.deps.APIPort()
	if port <= 0 {
		return errors.New("local xray is not running")
	}
	var api xray.XrayAPI
	if err := api.Init(port); err != nil {
		return err
	}
	defer api.Close()
	return fn(&api)
}

func (l *Local) AddInbound(_ context.Context, ib *model.Inbound) error {
	body, err := json.MarshalIndent(ib.GenXrayInboundConfig(), "", "  ")
	if err != nil {
		return err
	}
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.AddInbound(body)
	})
}

func (l *Local) DelInbound(_ context.Context, ib *model.Inbound) error {
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.DelInbound(ib.Tag)
	})
}

func (l *Local) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	// xray-core has no in-place inbound update — drop and re-add.
	// Matches what InboundService.UpdateInbound did inline.
	if err := l.DelInbound(ctx, oldIb); err != nil {
		// Best-effort: continue to AddInbound so a transient remove
		// failure (e.g. inbound already gone) doesn't strand us. The
		// caller's needRestart fallback will reconcile from config.
		_ = err
	}
	if !newIb.Enable {
		// Disabled inbounds aren't pushed to xray; we already removed
		// the old one above.
		return nil
	}
	return l.AddInbound(ctx, newIb)
}

func (l *Local) AddUser(_ context.Context, ib *model.Inbound, userMap map[string]any) error {
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.AddUser(string(ib.Protocol), ib.Tag, userMap)
	})
}

func (l *Local) RemoveUser(_ context.Context, ib *model.Inbound, email string) error {
	return l.withAPI(func(api *xray.XrayAPI) error {
		return api.RemoveUser(ib.Tag, email)
	})
}

func (l *Local) RestartXray(_ context.Context) error {
	if l.deps.SetNeedRestart != nil {
		l.deps.SetNeedRestart()
	}
	return nil
}

// Reset methods are intentional no-ops for Local. The central DB UPDATE
// that runs in InboundService.Reset* before this call has already zeroed
// the counters that xray reads; on the next stats poll the gRPC service
// will pick up matching values. Pre-Phase-1 the panel never issued an
// xrayApi reset call here either — keeping the same shape avoids a
// behaviour change for single-panel users.

func (l *Local) ResetClientTraffic(_ context.Context, _ *model.Inbound, _ string) error {
	return nil
}

func (l *Local) ResetInboundClientTraffics(_ context.Context, _ *model.Inbound) error {
	return nil
}

func (l *Local) ResetAllTraffics(_ context.Context) error {
	return nil
}
