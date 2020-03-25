package models

import (
	"time"
)

type BzMonitorRel struct {
	Id        int32     `xorm:"not null pk INT(11)"`
	Type      int32     `xorm:"INT(11)"`
	RefId     string    `xorm:"VARCHAR(50)"`
	MonitorId string    `xorm:"not null default '' index VARCHAR(50)"`
	Name      string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"DATETIME"`
	UpdatedAt time.Time `xorm:"DATETIME"`
	Deleted   int32     `xorm:"INT(11)"`
	SystemId  int32     `xorm:"INT(11)"`
	ProcessId int32     `xorm:"INT(11)"`
}
