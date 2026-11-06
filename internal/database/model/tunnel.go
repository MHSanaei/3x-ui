package model

// Tombstones survive node outages so a reconnect cannot resurrect a deleted tunnel.
type ManagedTunnel struct {
	ID        int    `gorm:"primaryKey" json:"-"`
	GUID      string `gorm:"uniqueIndex;size:36;not null" json:"-"`
	Owner     string `gorm:"size:128;not null" json:"-"`
	AuthOwner string `gorm:"size:128" json:"-"`
	NodeID    *int   `gorm:"index" json:"-"`
	Name      string `gorm:"size:128;not null" json:"-"`
	Revision  int64  `gorm:"not null" json:"-"`
	Restart   int64  `gorm:"not null" json:"-"`
	Enabled   bool   `gorm:"not null" json:"-"`
	Deleted   bool   `gorm:"not null;index" json:"-"`
	Config    string `gorm:"type:text;not null" json:"-"`
	Secret    string `gorm:"type:text;not null" json:"-"`
}
