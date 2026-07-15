package model

import "time"

type NaiveOutbound struct {
	ID                  uint      `gorm:"primarykey"`
	Tag                 string    `gorm:"uniqueIndex;not null"`
	ProxyURL            string    `gorm:"not null"`
	LocalPort           int       `gorm:"not null"`
	Enabled             bool      `gorm:"default:true"`
	InsecureConcurrency int
	TunnelTimeout       int
	IdleTimeout         int
	ExtraHeaders        string
	HostResolverRules   string
	ResolverRange       string
	NoPostQuantum       bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (NaiveOutbound) TableName() string { return "naive_outbounds" }