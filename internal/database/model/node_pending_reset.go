package model

// NodePendingReset is a client traffic reset a hosting node has not confirmed;
// until it lands the node still counts pre-reset usage, so every sync replays it.
type NodePendingReset struct {
	Id     int    `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeId int    `json:"nodeId" gorm:"uniqueIndex:idx_node_pending_reset,priority:1;not null"`
	Email  string `json:"email" gorm:"uniqueIndex:idx_node_pending_reset,priority:2;not null"`
	// QueuedAt (ns) tells a delivery apart from a reset re-queued while it ran.
	QueuedAt int64 `json:"queuedAt"`
}
