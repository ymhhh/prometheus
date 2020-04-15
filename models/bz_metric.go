package models

import (
	"time"
)

type BzMetric struct {
	Id        string    `xorm:"not null pk VARCHAR(50)"`
	Name      string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
	UpdatedAt time.Time `xorm:"DATETIME updated"`
	Deleted   int32     `xorm:"INT(11)"`
	Type      int32     `xorm:"INT(11)"`
	RefId     string    `xorm:"VARCHAR(50)"`
	SystemId  string    `xorm:"VARCHAR(50)"`
	ProcessId string    `xorm:"VARCHAR(50)"`
}
