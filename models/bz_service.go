package models

import (
	"time"
)

type BzService struct {
	Id           string    `xorm:"not null pk VARCHAR(50)"`
	Name         string    `xorm:"not null default '' VARCHAR(50)"`
	NameCn       string    `xorm:"VARCHAR(50)"`
	Owner        string    `xorm:"TEXT"`
	Members      string    `xorm:"TEXT"`
	CreatorErp   string    `xorm:"VARCHAR(45)"`
	UpdaterErp   string    `xorm:"VARCHAR(45)"`
	CreatedAt    time.Time `xorm:"DATETIME created"`
	UpdatedAt    time.Time `xorm:"DATETIME updated"`
	Deleted      int32     `xorm:"INT(11)"`
	DeptmentCode string    `xorm:"VARCHAR(45)"`
	DeptmentName string    `xorm:"VARCHAR(45)"`
	Type         int32     `xorm:"INT(11)"`
	SystemId     string    `xorm:"VARCHAR(50)"`
	ServiceId    string    `xorm:"VARCHAR(50)"`
	FullPath     string    `xorm:"VARCHAR(100)"`
	Version      int       `xorm:"version"`
}
