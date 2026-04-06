//go:build !linux && !windows && !android && !darwin

package tun

import (
	"github.com/xtls/xray-core/common/errors"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type DefaultTun struct {
}

// DefaultTun implements Tun
var _ Tun = (*DefaultTun)(nil)

// DefaultTun implements GVisorTun
var _ GVisorTun = (*DefaultTun)(nil)

// NewTun builds new tun interface handler
func NewTun(options TunOptions) (Tun, error) {
	return nil, errors.New("Tun is not supported on your platform")
}

func (t *DefaultTun) Start() error {
	return errors.New("Tun is not supported on your platform")
}

func (t *DefaultTun) Close() error {
	return errors.New("Tun is not supported on your platform")
}

func (t *DefaultTun) newEndpoint() (stack.LinkEndpoint, error) {
	return nil, errors.New("Tun is not supported on your platform")
}
