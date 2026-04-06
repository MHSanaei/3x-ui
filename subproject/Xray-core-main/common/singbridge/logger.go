package singbridge

import (
	"context"

	"github.com/sagernet/sing/common/logger"
	"github.com/xtls/xray-core/common/errors"
)

var _ logger.ContextLogger = (*XrayLogger)(nil)

type XrayLogger struct {
	newError func(values ...any) *errors.Error
}

func NewLogger(newErrorFunc func(values ...any) *errors.Error) *XrayLogger {
	return &XrayLogger{
		newErrorFunc,
	}
}

func (l *XrayLogger) Trace(args ...any) {
}

func (l *XrayLogger) Debug(args ...any) {
	errors.LogDebug(context.Background(), args...)
}

func (l *XrayLogger) Info(args ...any) {
	errors.LogInfo(context.Background(), args...)
}

func (l *XrayLogger) Warn(args ...any) {
	errors.LogWarning(context.Background(), args...)
}

func (l *XrayLogger) Error(args ...any) {
	errors.LogError(context.Background(), args...)
}

func (l *XrayLogger) Fatal(args ...any) {
}

func (l *XrayLogger) Panic(args ...any) {
}

func (l *XrayLogger) TraceContext(ctx context.Context, args ...any) {
}

func (l *XrayLogger) DebugContext(ctx context.Context, args ...any) {
	errors.LogDebug(ctx, args...)
}

func (l *XrayLogger) InfoContext(ctx context.Context, args ...any) {
	errors.LogInfo(ctx, args...)
}

func (l *XrayLogger) WarnContext(ctx context.Context, args ...any) {
	errors.LogWarning(ctx, args...)
}

func (l *XrayLogger) ErrorContext(ctx context.Context, args ...any) {
	errors.LogError(ctx, args...)
}

func (l *XrayLogger) FatalContext(ctx context.Context, args ...any) {
}

func (l *XrayLogger) PanicContext(ctx context.Context, args ...any) {
}
