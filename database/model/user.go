package model

import (
	"time"
)

// User represents a panel user with RBAC role.
type User struct {
	ID        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string    `json:"email" gorm:"unique;not null"`
	Password  string    `json:"-" gorm:"not null"`
	Role      string    `json:"role" gorm:"default:'reader'"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
