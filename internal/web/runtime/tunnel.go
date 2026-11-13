package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
)

type TunnelRuntime interface {
	ApplyTunnel(context.Context, wattunnel.Spec) (wattunnel.Status, error)
	TunnelCapabilities(context.Context) (wattunnel.Capabilities, error)
}

func (m *Manager) TunnelRuntimeFor(nodeID *int) (TunnelRuntime, error) {
	rt, err := m.RuntimeFor(nodeID)
	if err != nil {
		return nil, err
	}
	tr, ok := rt.(TunnelRuntime)
	if !ok {
		return nil, errors.New("runtime does not support tunnels")
	}
	return tr, nil
}

func (l *Local) ApplyTunnel(ctx context.Context, s wattunnel.Spec) (wattunnel.Status, error) {
	err := wattunnel.GetManager().Apply(ctx, s)
	return wattunnel.GetManager().Status(s.GUID), err
}

func (l *Local) TunnelCapabilities(_ context.Context) (wattunnel.Capabilities, error) {
	return wattunnel.GetCapabilities(), nil
}

func (r *Remote) tunnelTransport() error {
	if r.node.Scheme != "https" || r.node.TlsVerifyMode == "skip" {
		return errors.New("tunnel synchronization requires verified HTTPS, certificate pinning, or mTLS")
	}
	return nil
}

func (r *Remote) ApplyTunnel(ctx context.Context, s wattunnel.Spec) (wattunnel.Status, error) {
	if err := r.tunnelTransport(); err != nil {
		return wattunnel.Status{}, err
	}
	// Remote errors can echo request bodies; never propagate their diagnostic text for secrets.
	env, err := r.do(ctx, http.MethodPost, "panel/api/tunnels/sync", s)
	if err != nil {
		return wattunnel.Status{}, errors.New("tunnel synchronization failed; check node availability, permissions and panel version")
	}
	var status wattunnel.Status
	if err = json.Unmarshal(env.Obj, &status); err != nil {
		return status, errors.New("invalid tunnel status from node")
	}
	return status, nil
}

func (r *Remote) TunnelCapabilities(ctx context.Context) (wattunnel.Capabilities, error) {
	if err := r.tunnelTransport(); err != nil {
		return wattunnel.Capabilities{}, err
	}
	env, err := r.do(ctx, http.MethodGet, "panel/api/tunnels/capabilities", nil)
	if err != nil {
		return wattunnel.Capabilities{}, errors.New("node unavailable or does not support tunnels")
	}
	var caps wattunnel.Capabilities
	err = json.Unmarshal(env.Obj, &caps)
	return caps, err
}
