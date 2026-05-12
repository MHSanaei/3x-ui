package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

type LocalDeps struct {
	APIPort        func() int
	SetNeedRestart func()
}

type Local struct {
	deps LocalDeps
	mu   sync.Mutex
}

func NewLocal(deps LocalDeps) *Local {
	return &Local{deps: deps}
}

func (l *Local) Name() string { return "local" }

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
	_ = l.DelInbound(ctx, oldIb)
	if !newIb.Enable {
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

func (l *Local) ResetClientTraffic(_ context.Context, _ *model.Inbound, _ string) error {
	return nil
}

func (l *Local) ResetInboundClientTraffics(_ context.Context, _ *model.Inbound) error {
	return nil
}

func (l *Local) ResetAllTraffics(_ context.Context) error {
	return nil
}
