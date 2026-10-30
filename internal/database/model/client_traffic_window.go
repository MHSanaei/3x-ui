package model

// ClientTrafficWindow is one closed traffic window of a client: usage zeroed
// at a reset, auto-renewal, or deletion; lifetime = SUM(rows) + live counters.
type ClientTrafficWindow struct {
	Id     int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Email  string `json:"email" gorm:"index:idx_client_traffic_window_email,composite:1;not null"`
	At     int64  `json:"at" gorm:"index:idx_client_traffic_window_email,composite:2"`
	Up     int64  `json:"up"`
	Down   int64  `json:"down"`
	Reason string `json:"reason" gorm:"size:16"` // reset | renew | deleted
}
