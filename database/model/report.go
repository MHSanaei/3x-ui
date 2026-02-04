package model

type ConnectionReport struct {
	Id                   int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientIP             string `json:"client_ip"`
	Protocol             string `json:"protocol"`
	Remarks              string `json:"remarks"`
	Latency              int    `json:"latency"`
	Success              bool   `json:"success"`
	InterfaceName        string `json:"interface_name"`
	InterfaceDescription string `json:"interface_description"`
	InterfaceType        string `json:"interface_type"`
	Message              string `json:"message"`
	CreatedAt            int64  `json:"created_at" gorm:"autoCreateTime"`
}
