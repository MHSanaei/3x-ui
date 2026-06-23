package model

type NodeClientTraffic struct {
	Id     int    `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId int    `json:"nodeId" gorm:"uniqueIndex:idx_node_email,priority:1;not null"`
	Email  string `json:"email" gorm:"uniqueIndex:idx_node_email,priority:2;not null"`
	Up     int64  `json:"up"`
	Down   int64  `json:"down"`
	// BilledUp/BilledDown are the node-reported Billed baseline (raw, as the node
	// has already applied its inbounds' multipliers). Like Up/Down they are the
	// delta baseline — never multiplied again on the master.
	BilledUp   int64 `json:"billedUp" gorm:"default:0"`
	BilledDown int64 `json:"billedDown" gorm:"default:0"`
}
