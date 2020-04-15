package models

import (
	"time"
)

type BzProcessHostRel struct {
	Id        string    `xorm:"not null pk VARCHAR(50)"`
	HostsId   string    `xorm:"index VARCHAR(50)"`
	ProcessId string    `xorm:"index VARCHAR(50)"`
	ProgramId string    `xorm:"VARCHAR(50)"`
	Deleted   int32     `xorm:"INT(11)"`
	CreatedAt time.Time `xorm:"DATETIME created"`
	UpdatedAt time.Time `xorm:"DATETIME updated"`
}
