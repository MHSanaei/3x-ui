package core

import "context"

// Type identifies the proxy engine used by the panel.
type Type string

const (
	Xray    Type = "xray"
	SingBox  Type = "sing-box"
)

// Valid reports whether the core type is supported by this build.
func (t Type) Valid() bool {
	return t == Xray || t == SingBox
}

// Runtime is the common lifecycle contract for panel-managed proxy cores.
// Configuration translation, statistics and protocol-specific operations are
// intentionally kept out of this interface; they belong to the selected core
// implementation and will be added behind capability-specific interfaces.
type Runtime interface {
	Type() Type
	Version(ctx context.Context) (string, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	IsRunning() bool
	ValidateConfig(ctx context.Context) error
}
