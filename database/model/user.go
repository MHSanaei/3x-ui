package model

// ВАЖНО: имена полей ДОЛЖНЫ остаться такими,
// потому что их использует остальной код: Id, Username, PasswordHash, Role.
type User struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username     string `json:"username" gorm:"uniqueIndex;not null"`
	Password     string `json:"-"` // может использоваться для приема сырого пароля (не храним)
	PasswordHash string `json:"-" gorm:"column:password_hash"`
	Role         string `json:"role" gorm:"not null"` // admin | moder | reader
}
