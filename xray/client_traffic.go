package xray

type ClientTraffic struct {
	Id  int `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId int `json:"inboundId" form:"inboundId"`
	Enable     bool   `json:"enable" form:"enable"`
	Email       string `json:"email" form:"email" gorm:"unique"`
	Up        int64 `json:"up" form:"up"`
	Down      int64 `json:"down" form:"down"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	Total      int64  `json:"total" form:"total"`
}
