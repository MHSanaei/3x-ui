package xray

// Traffic represents network traffic statistics for Xray connections.
// It tracks upload and download bytes for inbound or outbound traffic.
type Traffic struct {
	IsInbound  bool   `json:"IsInbound" example:"true"`
	IsOutbound bool   `json:"IsOutbound" example:"false"`
	Tag        string `json:"Tag" example:"inbound-443"`
	Up         int64  `json:"Up" example:"1048576"`
	Down       int64  `json:"Down" example:"2097152"`
}
