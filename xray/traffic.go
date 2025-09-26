package xray

type Traffic struct {
	IsInbound  bool
	IsOutbound bool
	Tag        string
	Up         int64
	Down       int64
}
