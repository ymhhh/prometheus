package models

import (
	"time"
)

type BzAppProcess struct {
	Id           string    `xorm:"not null pk VARCHAR(50)"`
	AppId        string    `xorm:"VARCHAR(50)"`
	Name         string    `xorm:"not null default '' VARCHAR(50)"`
	NameCn       string    `xorm:"VARCHAR(50)"`
	Owner        string    `xorm:"VARCHAR(50)"`
	Members      string    `xorm:"VARCHAR(150)"`
	CreatorErp   string    `xorm:"VARCHAR(45)"`
	UpdaterErp   string    `xorm:"VARCHAR(45)"`
	CreatedAt    time.Time `xorm:"DATETIME created"`
	UpdatedAt    time.Time `xorm:"DATETIME updated"`
	Deleted      int32     `xorm:"INT(11)"`
	DeptmentCode string    `xorm:"VARCHAR(45)"`
	DeptmentName string    `xorm:"VARCHAR(45)"`
	Type         int32     `xorm:"INT(11)"`
	LastVersion  string    `xorm:"VARCHAR(45)"`
	VersionId    string    `xorm:"VARCHAR(45)"`
}