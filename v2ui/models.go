package v2ui

import "x-ui/database/model"

type V2Inbound struct {
	Id             int `gorm:"primaryKey;autoIncrement"`
	Port           int `gorm:"unique"`
	Listen         string
	Protocol       string
	Settings       string
	StreamSettings string
	Tag            string `gorm:"unique"`
	Sniffing       string
	Remark         string
	Up             int64
	Down           int64
	Enable         bool
}

func (i *V2Inbound) TableName() string {
	return "inbound"
}

func (i *V2Inbound) ToInbound(userId int) *model.Inbound {
	return &model.Inbound{
		UserId:         userId,
		Up:             i.Up,
		Down:           i.Down,
		Total:          0,
		Remark:         i.Remark,
		Enable:         i.Enable,
		ExpiryTime:     0,
		Listen:         i.Listen,
		Port:           i.Port,
		Protocol:       model.Protocol(i.Protocol),
		Settings:       i.Settings,
		StreamSettings: i.StreamSettings,
		Tag:            i.Tag,
		Sniffing:       i.Sniffing,
	}
}
