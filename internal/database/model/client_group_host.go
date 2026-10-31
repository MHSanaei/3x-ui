package model

// ClientGroupHost assigns stable logical HostGroup IDs to a client group.
// An empty relation set preserves the default behavior: all eligible hosts.
type ClientGroupHost struct {
	GroupName   string `json:"groupName" gorm:"primaryKey;column:group_name"`
	HostGroupId string `json:"hostGroupId" gorm:"primaryKey;column:host_group_id"`
}

func (ClientGroupHost) TableName() string { return "client_group_hosts" }
