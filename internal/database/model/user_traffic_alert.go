package model

// UserTrafficAlertSample contains a trusted local-Xray user increment. Only
// non-zero intervals are stored; zero intervals still feed sustained-rate state.
type UserTrafficAlertSample struct {
	ID      uint64 `gorm:"primaryKey"`
	Email   string `gorm:"index:idx_user_alert_window,priority:1;not null"`
	StartMS int64  `gorm:"not null"`
	EndMS   int64  `gorm:"index:idx_user_alert_window,priority:2;index:idx_user_alert_expiry;not null"`
	Bytes   int64  `gorm:"not null"`
}

type UserTrafficAlertState struct {
	Email     string `gorm:"primaryKey"`
	Payload   string `gorm:"type:text;not null"`
	UpdatedMS int64  `gorm:"index"`
}
