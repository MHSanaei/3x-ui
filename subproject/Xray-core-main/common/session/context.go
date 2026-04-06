package session

import (
	"context"
	_ "unsafe"

	"github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/routing"
)

//go:linkname IndependentCancelCtx context.newCancelCtx
func IndependentCancelCtx(parent context.Context) context.Context

const (
	inboundSessionKey         ctx.SessionKey = 1
	outboundSessionKey        ctx.SessionKey = 2
	contentSessionKey         ctx.SessionKey = 3
	isReverseMuxKey           ctx.SessionKey = 4  // is reverse mux
	sockoptSessionKey         ctx.SessionKey = 5  // used by dokodemo to only receive sockopt.Mark
	trackedConnectionErrorKey ctx.SessionKey = 6  // used by observer to get outbound error
	dispatcherKey             ctx.SessionKey = 7  // used by ss2022 inbounds to get dispatcher
	timeoutOnlyKey            ctx.SessionKey = 8  // mux context's child contexts to only cancel when its own traffic times out
	allowedNetworkKey         ctx.SessionKey = 9  // muxcool server control incoming request tcp/udp
	fullHandlerKey            ctx.SessionKey = 10 // outbound gets full handler
	mitmAlpn11Key             ctx.SessionKey = 11 // used by TLS dialer
	mitmServerNameKey         ctx.SessionKey = 12 // used by TLS dialer
)

func ContextWithInbound(ctx context.Context, inbound *Inbound) context.Context {
	return context.WithValue(ctx, inboundSessionKey, inbound)
}

func InboundFromContext(ctx context.Context) *Inbound {
	if inbound, ok := ctx.Value(inboundSessionKey).(*Inbound); ok {
		return inbound
	}
	return nil
}

func ContextWithOutbounds(ctx context.Context, outbounds []*Outbound) context.Context {
	return context.WithValue(ctx, outboundSessionKey, outbounds)
}

func SubContextFromMuxInbound(ctx context.Context) context.Context {
	newOutbounds := []*Outbound{{}}

	content := ContentFromContext(ctx)
	newContent := Content{}
	if content != nil {
		newContent = *content
		if content.Attributes != nil {
			panic("content.Attributes != nil")
		}
	}
	return ContextWithContent(ContextWithOutbounds(ctx, newOutbounds), &newContent)
}

func OutboundsFromContext(ctx context.Context) []*Outbound {
	if outbounds, ok := ctx.Value(outboundSessionKey).([]*Outbound); ok {
		return outbounds
	}
	return nil
}

func ContextWithContent(ctx context.Context, content *Content) context.Context {
	return context.WithValue(ctx, contentSessionKey, content)
}

func ContentFromContext(ctx context.Context) *Content {
	if content, ok := ctx.Value(contentSessionKey).(*Content); ok {
		return content
	}
	return nil
}

func ContextWithIsReverseMux(ctx context.Context, isReverseMux bool) context.Context {
	return context.WithValue(ctx, isReverseMuxKey, isReverseMux)
}

func IsReverseMuxFromContext(ctx context.Context) bool {
	if val, ok := ctx.Value(isReverseMuxKey).(bool); ok {
		return val
	}
	return false
}

func ContextWithSockopt(ctx context.Context, s *Sockopt) context.Context {
	return context.WithValue(ctx, sockoptSessionKey, s)
}

func SockoptFromContext(ctx context.Context) *Sockopt {
	if sockopt, ok := ctx.Value(sockoptSessionKey).(*Sockopt); ok {
		return sockopt
	}
	return nil
}

func GetForcedOutboundTagFromContext(ctx context.Context) string {
	if ContentFromContext(ctx) == nil {
		return ""
	}
	return ContentFromContext(ctx).Attribute("forcedOutboundTag")
}

func SetForcedOutboundTagToContext(ctx context.Context, tag string) context.Context {
	if contentFromContext := ContentFromContext(ctx); contentFromContext == nil {
		ctx = ContextWithContent(ctx, &Content{})
	}
	ContentFromContext(ctx).SetAttribute("forcedOutboundTag", tag)
	return ctx
}

type TrackedRequestErrorFeedback interface {
	SubmitError(err error)
}

func SubmitOutboundErrorToOriginator(ctx context.Context, err error) {
	if errorTracker := ctx.Value(trackedConnectionErrorKey); errorTracker != nil {
		errorTracker := errorTracker.(TrackedRequestErrorFeedback)
		errorTracker.SubmitError(err)
	}
}

func TrackedConnectionError(ctx context.Context, tracker TrackedRequestErrorFeedback) context.Context {
	return context.WithValue(ctx, trackedConnectionErrorKey, tracker)
}

func ContextWithDispatcher(ctx context.Context, dispatcher routing.Dispatcher) context.Context {
	return context.WithValue(ctx, dispatcherKey, dispatcher)
}

func DispatcherFromContext(ctx context.Context) routing.Dispatcher {
	if dispatcher, ok := ctx.Value(dispatcherKey).(routing.Dispatcher); ok {
		return dispatcher
	}
	return nil
}

func ContextWithTimeoutOnly(ctx context.Context, only bool) context.Context {
	return context.WithValue(ctx, timeoutOnlyKey, only)
}

func TimeoutOnlyFromContext(ctx context.Context) bool {
	if val, ok := ctx.Value(timeoutOnlyKey).(bool); ok {
		return val
	}
	return false
}

func ContextWithAllowedNetwork(ctx context.Context, network net.Network) context.Context {
	return context.WithValue(ctx, allowedNetworkKey, network)
}

func AllowedNetworkFromContext(ctx context.Context) net.Network {
	if val, ok := ctx.Value(allowedNetworkKey).(net.Network); ok {
		return val
	}
	return net.Network_Unknown
}

func ContextWithFullHandler(ctx context.Context, handler outbound.Handler) context.Context {
	return context.WithValue(ctx, fullHandlerKey, handler)
}

func FullHandlerFromContext(ctx context.Context) outbound.Handler {
	if val, ok := ctx.Value(fullHandlerKey).(outbound.Handler); ok {
		return val
	}
	return nil
}

func ContextWithMitmAlpn11(ctx context.Context, alpn11 bool) context.Context {
	return context.WithValue(ctx, mitmAlpn11Key, alpn11)
}

func MitmAlpn11FromContext(ctx context.Context) bool {
	if val, ok := ctx.Value(mitmAlpn11Key).(bool); ok {
		return val
	}
	return false
}

func ContextWithMitmServerName(ctx context.Context, serverName string) context.Context {
	return context.WithValue(ctx, mitmServerNameKey, serverName)
}

func MitmServerNameFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(mitmServerNameKey).(string); ok {
		return val
	}
	return ""
}
