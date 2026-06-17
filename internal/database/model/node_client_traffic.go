package model

type NodeClientTraffic struct {
	Id     int `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId int `json:"nodeId" gorm:"uniqueIndex:idx_node_email,priority:1;not null"`
	// The composite unique index leads with node_id, so the email-keyed deletes
	// (setRemoteTraffic / client removal — "email = ?" and "email IN (...)")
	// can't use it; this single-column index covers them.
	Email string `json:"email" gorm:"uniqueIndex:idx_node_email,priority:2;index:idx_node_client_traffics_email;not null"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
}
