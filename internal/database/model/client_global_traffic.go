package model

// ClientGlobalTraffic mirrors a master panel's aggregated (global) usage for a
// client hosted on this panel. Masters push one row per (master, email) so the
// node can display the client's true cross-panel total and enforce its quota
// locally. The values never feed back into client_traffics — that table keeps
// this panel's local-only counters, which is what keeps every master's
// delta-baseline accounting over our snapshot correct.
//
// Rows are overwritten in place on every push (not max-merged), so a traffic
// reset on the master propagates here within one push cycle. Readers that need
// a single number fold the per-master rows with MAX.
type ClientGlobalTraffic struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	MasterGuid string `json:"masterGuid" gorm:"uniqueIndex:idx_master_email,priority:1;not null"`
	Email      string `json:"email" gorm:"uniqueIndex:idx_master_email,priority:2;index:idx_client_global_email;not null"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	UpdatedAt  int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}
