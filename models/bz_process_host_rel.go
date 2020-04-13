package models

import (
	"time"
)

type BzProcessHostRel struct {
	Id        string    `xorm:"not null pk VARCHAR(50)"`
	HostsId   string    `xorm:"index VARCHAR(50)"`
	ProcessId string    `xorm:"index VARCHAR(50)"`
	ProgramId string    `xorm:"VARCHAR(50)"`
	Deleted   int       `xorm:"INT(11)"`
	CreatedAt time.Time `xorm:"DATETIME"`
	UpdatedAt time.Time `xorm:"DATETIME"`
}
