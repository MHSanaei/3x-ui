package model

// ClientInboundTraffic stores per-attachment (one client × one inbound) traffic
// for the Traffic Multiplier feature. Up/Down are Real bytes metered for this
// inbound alone — made possible by the per-attachment accounting identity
// ("<inboundId>::<email>") that Xray meters separately. BilledUp/BilledDown are
// those same bytes after the inbound's Traffic Multiplier, accrued at the
// multiplier in force when the traffic happened (non-retroactive), so a later
// multiplier change never re-bills the past.
//
// The per-client aggregate (Real up/down + billed_up/billed_down) lives on
// client_traffics, which is what quota enforcement reads; this table is the
// per-tunnel breakdown shown in the client modal and the basis for exact
// attribution when a client is detached from one inbound. Rows are retained on
// detach (no refund) and cleared on traffic reset/renew.
type ClientInboundTraffic struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" gorm:"uniqueIndex:idx_client_inbound_traffic,priority:1;not null;index"`
	Email      string `json:"email" gorm:"uniqueIndex:idx_client_inbound_traffic,priority:2;not null"`
	Up         int64  `json:"up" gorm:"default:0"`
	Down       int64  `json:"down" gorm:"default:0"`
	BilledUp   int64  `json:"billedUp" gorm:"default:0"`
	BilledDown int64  `json:"billedDown" gorm:"default:0"`
}

func (ClientInboundTraffic) TableName() string { return "client_inbound_traffics" }
