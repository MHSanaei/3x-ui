package model

// XrayTrafficReceipt records the latest committed sample of the single Xray poller.
type XrayTrafficReceipt struct {
	ID      int    `gorm:"primaryKey;autoIncrement:false"`
	BatchID string `gorm:"not null"`
}
