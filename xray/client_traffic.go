package xray

// ClientTraffic represents traffic statistics and limits for a specific client.
// It tracks upload/download usage, expiry times, and online status for inbound clients.
type ClientTraffic struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId"`
	Enable     bool   `json:"enable" form:"enable"`
	Email      string `json:"email" form:"email" gorm:"unique"`
	UUID       string `json:"uuid" form:"uuid" gorm:"-"`
	SubId      string `json:"subId" form:"subId" gorm:"-"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	AllTime    int64  `json:"allTime" form:"allTime"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	Total      int64  `json:"total" form:"total"`
	Reset      int    `json:"reset" form:"reset" gorm:"default:0"`
	LastOnline int64  `json:"lastOnline" form:"lastOnline" gorm:"default:0"`
	// DailyTrafficModelExtension: Adds persistent fields for 24h cycle accounting (DailyUp/Down)
	// and a timezone-aware reset checkpoint. Enables "lazy reset" logic during traffic updates,
	// eliminating the need for background cron jobs or scheduled tasks.
	DailyUp        int64 `json:"dailyUp" form:"dailyUp"`
	DailyDown      int64 `json:"dailyDown" form:"dailyDown"`
	LastDailyReset int64 `json:"lastDailyReset" form:"lastDailyReset" gorm:"default:0"`
}
