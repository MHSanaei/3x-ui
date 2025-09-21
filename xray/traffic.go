package xray

// Traffic represents network traffic statistics for Xray connections.
// It tracks upload and download bytes for inbound or outbound traffic.
type Traffic struct {
	IsInbound  bool
	IsOutbound bool
	Tag        string
	Up         int64
	Down       int64
}
