package model

import (
	"time"
)

// InboundClientIPs stores IP information for clients with IP-based access control
type InboundClientIPs struct {
	Id          int       `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientEmail string    `json:"clientEmail" form:"clientEmail" gorm:"unique"`
	IPs         string    `json:"ips" form:"ips" gorm:"type:text"` // Comma-separated IP list
	CreatedAt   int64     `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt   int64     `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

// TableName specifies the table name for InboundClientIPs
func (InboundClientIPs) TableName() string {
	return "inbound_client_ips"
}
