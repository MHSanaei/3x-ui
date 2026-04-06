package ctx

import (
	"context"

	"github.com/xtls/xray-core/proxy/hysteria/account"
)

type key int

const (
	requireDatagram key = iota
	validator
)

func ContextWithRequireDatagram(ctx context.Context, udp bool) context.Context {
	if !udp {
		return ctx
	}
	return context.WithValue(ctx, requireDatagram, struct{}{})
}

func RequireDatagramFromContext(ctx context.Context) bool {
	_, ok := ctx.Value(requireDatagram).(struct{})
	return ok
}

func ContextWithValidator(ctx context.Context, v *account.Validator) context.Context {
	return context.WithValue(ctx, validator, v)
}

func ValidatorFromContext(ctx context.Context) *account.Validator {
	v, _ := ctx.Value(validator).(*account.Validator)
	return v
}
