package model

// ClientChangeLog records one metadata change to a client (renewal, limit or
// name change, switch, deletion); one row per changed field, newest read
// first in the client's usage history.
type ClientChangeLog struct {
	Id       int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Email    string `json:"email" gorm:"index:idx_client_change_log_email,composite:1;not null"`
	At       int64  `json:"at" gorm:"index:idx_client_change_log_email,composite:2"`
	Field    string `json:"field" gorm:"size:32"`
	OldValue string `json:"oldValue" gorm:"type:text"`
	NewValue string `json:"newValue" gorm:"type:text"`
	Source   string `json:"source" gorm:"size:32"` // panel | tgbot | api | auto-renew
}
