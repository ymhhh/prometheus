package models

import (
	"time"
)

type BzSystem struct {
	Id           string    `xorm:"not null pk VARCHAR(50)"`
	Name         string    `xorm:"not null default '' unique VARCHAR(50)"`
	NameCn       string    `xorm:"VARCHAR(50)"`
	Owner        string    `xorm:"TEXT"`
	Members      string    `xorm:"TEXT"`
	CreatorErp   string    `xorm:"VARCHAR(50)"`
	UpdaterErp   string    `xorm:"VARCHAR(50)"`
	CreatedAt    time.Time `xorm:"DATETIME created"`
	UpdatedAt    time.Time `xorm:"DATETIME updated"`
	Deleted      int32     `xorm:"INT(11)"`
	DeptmentCode string    `xorm:"VARCHAR(50)"`
	DeptmentName string    `xorm:"VARCHAR(50)"`
	Type         int32     `xorm:"INT(11)"`
	SystemId     string    `xorm:"VARCHAR(50)"`
}
