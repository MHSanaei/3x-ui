package model

// TuicTrafficReceipt marks a TUIC traffic journal batch as committed, so a
// batch replayed after an ambiguous commit is not counted twice.
type TuicTrafficReceipt struct {
	ID string `gorm:"primaryKey"`
}
