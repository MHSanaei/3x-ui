package model

// ClientGlobalTraffic separates each master's aggregate from local client_traffics for delta accounting; readers fold with MAX.
// Changed totals (including resets) overwrite on the next push; UpdatedAt tracks changes or hourly unchanged heartbeats.
type ClientGlobalTraffic struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	MasterGuid string `json:"masterGuid" gorm:"uniqueIndex:idx_master_email,priority:1;not null"`
	Email      string `json:"email" gorm:"uniqueIndex:idx_master_email,priority:2;index:idx_client_global_email;not null"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	UpdatedAt  int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}
