package xray

// ClientTraffic represents traffic statistics and limits for a specific client.
// It tracks upload/download usage, expiry times, and online status for inbound clients.
type ClientTraffic struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement" example:"14825"`
	InboundId  int    `json:"inboundId" form:"inboundId" gorm:"index:idx_client_traffics_inbound" example:"1"`
	Enable     bool   `json:"enable" form:"enable" example:"true"`
	Email      string `json:"email" form:"email" gorm:"unique" example:"user1"`
	UUID       string `json:"uuid" form:"uuid" gorm:"-" example:"e18c9a96-71bf-48d4-933f-8b9a46d4290c"`
	SubId      string `json:"subId" form:"subId" gorm:"-" example:"i7tvdpeffi0hvvf1"`
	Up         int64  `json:"up" form:"up" example:"1048576"`
	Down       int64  `json:"down" form:"down" example:"2097152"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime" example:"1735689600000"`
	Total      int64  `json:"total" form:"total" example:"10737418240"`
	Reset      int    `json:"reset" form:"reset" gorm:"default:0" example:"0"`
	LastOnline int64  `json:"lastOnline" form:"lastOnline" gorm:"default:0" example:"1735680000000"`
}
