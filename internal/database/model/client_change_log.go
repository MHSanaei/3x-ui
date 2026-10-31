package model

// ClientChangeLog records one metadata change to a client: one row per
// changed field, newest read first in the client's usage history.
type ClientChangeLog struct {
	Id       int64  `json:"id" gorm:"primaryKey;autoIncrement" example:"14825"`
	Email    string `json:"email" gorm:"index:idx_client_change_log_email,composite:1;not null" example:"user1@example.com"`
	At       int64  `json:"at" gorm:"index:idx_client_change_log_email,composite:2" example:"1735689600000"`
	Field    string `json:"field" gorm:"size:32" example:"totalGB"`
	OldValue string `json:"oldValue" gorm:"type:text" example:"10"`
	NewValue string `json:"newValue" gorm:"type:text" example:"20"`
	Source   string `json:"source" gorm:"size:32" example:"panel"` // panel | tgbot | api | auto-renew
}
