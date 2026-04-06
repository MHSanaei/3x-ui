package extension

import "context"

type ContextReceiver interface {
	InjectContext(ctx context.Context)
}
