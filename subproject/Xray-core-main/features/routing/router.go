package routing

import (
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/features"
)

// Router is a feature to choose an outbound tag for the given request.
//
// xray:api:stable
type Router interface {
	features.Feature

	// PickRoute returns a route decision based on the given routing context.
	PickRoute(ctx Context) (Route, error)
	AddRule(config *serial.TypedMessage, shouldAppend bool) error
	RemoveRule(tag string) error
	ListRule() []Route
}

// Route is the routing result of Router feature.
//
// xray:api:stable
type Route interface {
	// A Route is also a routing context.
	Context

	// GetOutboundGroupTags returns the detoured outbound group tags in sequence before a final outbound is chosen.
	GetOutboundGroupTags() []string

	// GetOutboundTag returns the tag of the outbound the connection was dispatched to.
	GetOutboundTag() string

	// GetRuleTag returns the matching rule tag for debugging if exists
	GetRuleTag() string
}

// RouterType return the type of Router interface. Can be used to implement common.HasType.
//
// xray:api:stable
func RouterType() interface{} {
	return (*Router)(nil)
}

// DefaultRouter is an implementation of Router, which always returns ErrNoClue for routing decisions.
type DefaultRouter struct{}

// Type implements common.HasType.
func (DefaultRouter) Type() interface{} {
	return RouterType()
}

// PickRoute implements Router.
func (DefaultRouter) PickRoute(ctx Context) (Route, error) {
	return nil, common.ErrNoClue
}

// AddRule implements Router.
func (DefaultRouter) AddRule(config *serial.TypedMessage, shouldAppend bool) error {
	return common.ErrNoClue
}

// RemoveRule implements Router.
func (DefaultRouter) RemoveRule(tag string) error {
	return common.ErrNoClue
}

// ListRule implements Router.
func (DefaultRouter) ListRule() []Route {
	return nil
}

// Start implements common.Runnable.
func (DefaultRouter) Start() error {
	return nil
}

// Close implements common.Closable.
func (DefaultRouter) Close() error {
	return nil
}
