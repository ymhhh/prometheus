package models

import (
	"time"
)

type BzCodePackage struct {
	Id         string    `xorm:"not null pk VARCHAR(50)"`
	Branch     string    `xorm:"not null default '' VARCHAR(50)"`
	Tag        string    `xorm:"VARCHAR(45)"`
	CreatorErp string    `xorm:"VARCHAR(45)"`
	UpdaterErp string    `xorm:"VARCHAR(45)"`
	CreatedAt  time.Time `xorm:"DATETIME"`
	UpdatedAt  time.Time `xorm:"DATETIME"`
	Deleted    int       `xorm:"INT(11)"`
	Type       int       `xorm:"INT(11)"`
	Version    string    `xorm:"VARCHAR(45)"`
	Buildtime  time.Time `xorm:"DATETIME"`
	OnlineTime time.Time `xorm:"DATETIME"`
	Desc       string    `xorm:"VARCHAR(150)"`
	ServiceId  string    `xorm:"VARCHAR(50)"`
	CodeId     string    `xorm:"VARCHAR(45)"`
	CommitId   string    `xorm:"VARCHAR(45)"`
}
