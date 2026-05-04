package xray

// ClientTraffic represents traffic statistics and limits for a specific client.
// It tracks upload/download usage, expiry times, and online status for inbound clients.
type ClientTraffic struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId"`
	Enable     bool   `json:"enable" form:"enable"`
	Email      string `json:"email" form:"email" gorm:"unique"`
	UUID       string `json:"uuid" form:"uuid" gorm:"-"`
	SubId      string `json:"subId" form:"subId" gorm:"index"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	AllTime    int64  `json:"allTime" form:"allTime"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	Total      int64  `json:"total" form:"total"`
	SubTotal   int64  `json:"subTotal" form:"subTotal"`
	Reset      int    `json:"reset" form:"reset" gorm:"default:0"`
	LastOnline int64  `json:"lastOnline" form:"lastOnline" gorm:"default:0"`
}
