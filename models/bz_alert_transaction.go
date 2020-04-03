package models

import (
	"time"
)

type BzAlertTransaction struct {
	Id        string    `xorm:"not null pk default '' VARCHAR(50)"`
	AlertId   string    `xorm:"not null default '' index index(created_at_idx) VARCHAR(50)"`
	Status    string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
}
