package models

import (
	"time"
)

type BzMonitorRel struct {
	Id        int64     `xorm:"pk autoincr BIGINT(20)"`
	Type      int32     `xorm:"INT(11)"`
	RefId     string    `xorm:"VARCHAR(50)"`
	MonitorId string    `xorm:"not null default '' index VARCHAR(50)"`
	Name      string    `xorm:"not null default '' VARCHAR(50)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
	UpdatedAt time.Time `xorm:"DATETIME updated"`
	Deleted   int32     `xorm:"INT(11)"`
	SystemId  int32     `xorm:"INT(11)"`
	ProcessId int32     `xorm:"INT(11)"`
}
