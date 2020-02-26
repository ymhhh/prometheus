package models

import (
	"time"
)

type BzMonitorRel struct {
	Id        int       `xorm:"not null pk INT(11)"`
	Type      int       `xorm:"INT(11)"`
	RefId     string    `xorm:"VARCHAR(50)"`
	MonitorId string    `xorm:"not null default '' index VARCHAR(50)"`
	Name      string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"DATETIME"`
	UpdatedAt time.Time `xorm:"DATETIME"`
	Deleted   int       `xorm:"INT(11)"`
	SystemId  int       `xorm:"INT(11)"`
	ProcessId int       `xorm:"INT(11)"`
}
