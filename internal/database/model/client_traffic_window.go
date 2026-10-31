package model

// ClientTrafficWindow is one closed traffic window of a client: usage zeroed
// at a reset, auto-renewal, or deletion; lifetime = SUM(rows) + live counters.
type ClientTrafficWindow struct {
	Id     int64  `json:"id" gorm:"primaryKey;autoIncrement" example:"14825"`
	Email  string `json:"email" gorm:"index:idx_client_traffic_window_email,composite:1;not null" example:"user1@example.com"`
	At     int64  `json:"at" gorm:"index:idx_client_traffic_window_email,composite:2" example:"1735689600000"`
	Up     int64  `json:"up" example:"1048576"`
	Down   int64  `json:"down" example:"2097152"`
	Reason string `json:"reason" gorm:"size:16" example:"reset"`
}
